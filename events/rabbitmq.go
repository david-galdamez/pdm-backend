package events

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

var (
	ErrUnroutable    = errors.New("event was not routed to any queue")
	ErrPublishNacked = errors.New("event was nacked by the broker")
)

func dial(rabbitUrl string) (*amqp.Connection, *amqp.Channel, error) {
	conn, err := amqp.Dial(rabbitUrl)
	if err != nil {
		return nil, nil, err
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, nil, err
	}

	err = DeclareTopology(ch)
	if err != nil {
		ch.Close()
		conn.Close()
		return nil, nil, err
	}

	return conn, ch, nil
}

type RabbitPublisher struct {
	url string

	// mu guards every field below it, which is what lets reconnect swap the
	// connection out from under an idle publisher: a publish holds mu for its
	// whole round-trip, so it can never be handed a channel mid-swap.
	mu      sync.Mutex
	conn    *amqp.Connection
	channel *amqp.Channel
	returns chan amqp.Return
	closed  bool
}

func NewRabbitPublisher(rabbitUrl string) (*RabbitPublisher, error) {
	r := &RabbitPublisher{url: rabbitUrl}

	if err := r.connect(); err != nil {
		return nil, err
	}

	return r, nil
}

// connect dials, installs the new connection, and starts watching it for loss.
// It must be called without mu held.
func (r *RabbitPublisher) connect() error {
	conn, ch, err := dial(r.url)
	if err != nil {
		return err
	}

	if err := ch.Confirm(false); err != nil {
		ch.Close()
		conn.Close()
		return err
	}

	// Nothing drains this in the background on purpose: PublishTransactionEmail
	// reads it itself, right after the confirm, so an unroutable event becomes
	// the error of the call that published it instead of a log line nobody
	// correlates. A goroutine ranging over it here would consume the return
	// first and leave that check permanently empty.
	returns := ch.NotifyReturn(make(chan amqp.Return, 16))
	lost := conn.NotifyClose(make(chan *amqp.Error, 1))

	r.mu.Lock()
	r.conn = conn
	r.channel = ch
	r.returns = returns
	r.mu.Unlock()

	go r.watch(lost)

	return nil
}

// watch reconnects after the broker goes away. Without it the publisher is
// dead for the life of the process: every later publish fails on a channel
// that will never reopen, and the API silently stops producing email events
// after any broker restart or network blip.
func (r *RabbitPublisher) watch(lost chan *amqp.Error) {
	amqpErr := <-lost

	// Close sets the flag before it closes the connection, so the flag — not
	// whether an error arrived with the close — is what separates our own
	// shutdown from the broker going away. A connection closed gracefully from
	// the other end (an operator killing it in the management UI, a broker
	// draining for restart) reports no error at all, and must still reconnect.
	if r.isClosed() {
		return
	}

	log.Printf("broker connection lost (%v); reconnecting", amqpErr)

	r.reconnect()
}

func (r *RabbitPublisher) reconnect() {
	backoff := time.Second
	const maxBackoff = 30 * time.Second

	for !r.isClosed() {
		time.Sleep(backoff)

		if r.isClosed() {
			return
		}

		if err := r.connect(); err == nil {
			log.Println("broker connection restored")
			return
		} else {
			log.Printf("reconnecting to the broker: %v", err)
		}

		if backoff < maxBackoff {
			backoff *= 2
		}
	}
}

func (r *RabbitPublisher) isClosed() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.closed
}

// Close stops the publisher for good: the reconnect loop checks the same flag,
// so a shutdown never races a redial.
func (r *RabbitPublisher) Close() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.closed = true

	if r.channel != nil {
		r.channel.Close()
	}

	if r.conn != nil {
		r.conn.Close()
	}
}

type RabbitReceiver struct {
	Conn    *amqp.Connection
	Channel *amqp.Channel
	lost    chan *amqp.Error
}

func NewRabbitReceiver(rabbitUrl string) (*RabbitReceiver, error) {

	conn, ch, err := dial(rabbitUrl)
	if err != nil {
		return nil, err
	}

	return &RabbitReceiver{
		Conn:    conn,
		Channel: ch,
		lost:    conn.NotifyClose(make(chan *amqp.Error, 1)),
	}, nil
}

// Lost fires when the broker connection drops. The consumer does not reconnect
// in-process the way the publisher does: the delivery channel closes anyway, so
// there is nothing left to receive on, and exiting non-zero lets the container's
// restart policy redial with a clean channel and a fresh prefetch. What Lost
// adds is the reason, which a bare closed delivery channel does not carry.
func (r *RabbitReceiver) Lost() <-chan *amqp.Error {
	return r.lost
}

func (r *RabbitReceiver) Close() {
	r.Channel.Close()
	r.Conn.Close()
}

func (r *RabbitPublisher) PublishTransactionEmail(ctx context.Context, event TransactionEmailEvent) error {

	pubCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	eventBytes, err := event.ToJSON()
	if err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	// A return left over from an earlier publish (or from before a reconnect)
	// would otherwise be blamed on this one.
	select {
	case <-r.returns:
	default:
	}

	confirmation, err := r.channel.PublishWithDeferredConfirmWithContext(
		pubCtx,
		EventsExchange,
		event.Type,
		true,  // mandatory
		false, // immediate
		amqp.Publishing{
			ContentType:  "application/json",
			Body:         eventBytes,
			DeliveryMode: amqp.Persistent,
			MessageId:    event.ID,
		},
	)
	if err != nil {
		return err
	}

	acked, err := confirmation.WaitContext(pubCtx)
	if err != nil {
		return err
	}

	if !acked {
		return fmt.Errorf("%w: %s", ErrPublishNacked, event.ID)
	}

	// The broker sends basic.return before the ack, so any return for this
	// message is already on the channel by now.
	select {
	case returned := <-r.returns:
		return fmt.Errorf("%w: key=%s reply=%d %s",
			ErrUnroutable, returned.RoutingKey, returned.ReplyCode, returned.ReplyText)
	default:
	}

	return nil
}
