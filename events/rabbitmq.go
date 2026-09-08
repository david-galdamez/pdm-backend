package events

import (
	"context"
	"sync"

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

	channel, err := conn.Channel()
	if err != nil {
		return nil, err
	}

	return &RabbitPublisher{
		Conn:    conn,
		Channel: channel,
	}, nil
}

func (r *RabbitPublisher) Close() {
	r.mu.Lock()
	r.Channel.Close()
	r.Conn.Close()
	r.mu.Unlock()
}

func (r *RabbitPublisher) PublishTransactionEmail(ctx context.Context, event TransactionEmailEvent) error {
	return nil
}
