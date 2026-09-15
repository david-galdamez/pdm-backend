package events

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

var (
	ErrUnroutable    = errors.New("event was not routed to any queue")
	ErrPublishNacked = errors.New("event was nacked by the broker")
)

type RabbitPublisher struct {
	Conn    *amqp.Connection
	Channel *amqp.Channel
	returns chan amqp.Return
	mu      sync.Mutex
}

func NewRabbitPublisher(rabbitUrl string) (*RabbitPublisher, error) {
	conn, err := amqp.Dial(rabbitUrl)
	if err != nil {
		return nil, err
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, err
	}

	if err := ch.Confirm(false); err != nil {
		ch.Close()
		conn.Close()
		return nil, err
	}

	err = DeclareTopology(ch)
	if err != nil {
		ch.Close()
		conn.Close()
		return nil, err
	}

	// Nothing drains this in the background on purpose: PublishTransactionEmail
	// reads it itself, right after the confirm, so an unroutable event becomes
	// the error of the call that published it instead of a log line nobody
	// correlates. A goroutine ranging over it here would consume the return
	// first and leave that check permanently empty.
	returns := ch.NotifyReturn(make(chan amqp.Return, 16))

	return &RabbitPublisher{
		Conn:    conn,
		Channel: ch,
		returns: returns,
	}, nil
}

func (r *RabbitPublisher) Close() {
	r.mu.Lock()
	r.Channel.Close()
	r.Conn.Close()
	r.mu.Unlock()
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

	confirmation, err := r.Channel.PublishWithDeferredConfirmWithContext(
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
