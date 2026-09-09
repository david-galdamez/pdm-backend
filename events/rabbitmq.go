package events

import (
	"context"
	"log"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

type RabbitPublisher struct {
	Conn    *amqp.Connection
	Channel *amqp.Channel
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

	ch.Confirm(false)

	err = DeclareTopology(ch)
	if err != nil {
		ch.Close()
		conn.Close()
		return nil, err
	}

	returns := ch.NotifyReturn(make(chan amqp.Return, 16))
	go func() {
		for r := range returns {
			log.Printf("unrouted event %s: exchange=%s key=%s reply=%d %s",
				r.MessageId, r.Exchange, r.RoutingKey, r.ReplyCode, r.ReplyText)
		}
	}()

	return &RabbitPublisher{
		Conn:    conn,
		Channel: ch,
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
	err = r.Channel.PublishWithContext(
		pubCtx,
		EventsExchange,
		event.Type,
		true,  // mandatory
		false, // immediate
		amqp.Publishing{
			ContentType: "application/json",
			Body:        eventBytes,

			DeliveryMode: 2,
			MessageId:    event.ID,
		},
	)
	if err != nil {
		return err
	}

	return nil
}
