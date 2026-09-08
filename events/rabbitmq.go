package events

import "context"

type RabbitPublisher struct {
}

func NewRabbitPublisher(rabbitUrl string) (*RabbitPublisher, error) {
	return &RabbitPublisher{}, nil
}

func (r *RabbitPublisher) Close() {
}

func (r *RabbitPublisher) PublishTransactionEmail(ctx context.Context, event TransactionEmailEvent) error {
	return nil
}
