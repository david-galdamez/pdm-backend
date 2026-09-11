package main

import (
	"context"
	"log"
	"pdm-backend/email"
	"pdm-backend/events"
	"pdm-backend/internal/config"
	"pdm-backend/internal/emailworker"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

func main() {
	cfg := config.Get()

	rabbitPublisher, err := events.NewRabbitPublisher(cfg.RABBIT_URL)
	if err != nil {
		log.Fatalf("Failed to create RabbitMQ publisher: %v", err)
	}
	defer rabbitPublisher.Close()

	smtpSender := email.NewSMTPSender(cfg.SMTP_HOST, cfg.SMTP_PORT, cfg.SMTP_USERNAME, cfg.SMTP_PASSWORD, cfg.SMTP_FROM)

	target := emailworker.EmailTransactionTarget{}
	dispatcher := emailworker.NewDispatcher(target, smtpSender)

	ch := rabbitPublisher.Channel
	if err := ch.Qos(10, 0, false); err != nil {
		log.Fatalf("setting qos: %v", err)
	}

	msgs, err := ch.Consume(
		events.EmailTransactionQueue,
		"",
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		log.Fatalf("consuming from %s, %v", events.EmailTransactionQueue, err)
	}

	log.Printf("email worker listening on %s", events.EmailTransactionQueue)

	for d := range msgs {
		result := handle(d, dispatcher)

		switch result {
		case emailworker.ResultAck:
			if err := d.Ack(false); err != nil {
				log.Printf("failed ack: %v", err)
			}
		case emailworker.ResultDead:
			log.Printf("event %s dead-lettering", d.MessageId)
			if err := d.Nack(false, false); err != nil {
				log.Printf("failed nack: %v", err)
			}
		}
	}

	log.Println("delivery channel closed, worker exiting")
}

func handle(d amqp.Delivery, dispatcher emailworker.Dispatcher) emailworker.HandlerResponse {

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return dispatcher.Handle(ctx, d.Body)
}
