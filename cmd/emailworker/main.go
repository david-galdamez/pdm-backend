package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"pdm-backend/email"
	"pdm-backend/events"
	"pdm-backend/internal/config"
	"pdm-backend/internal/emailworker"
	"syscall"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

func main() {
	cfg := config.Get()

	rabbitReceiver, err := events.NewRabbitReceiver(cfg.RABBIT_URL)
	if err != nil {
		log.Fatalf("Failed to create RabbitMQ receiver: %v", err)
	}

	defer rabbitReceiver.Close()

	smtpSender := email.NewSMTPSender(cfg.SMTP_HOST, cfg.SMTP_PORT, cfg.SMTP_USERNAME, cfg.SMTP_PASSWORD, cfg.SMTP_FROM)

	target := emailworker.EmailTransactionTarget{}
	dispatcher := emailworker.NewDispatcher(target, smtpSender)

	ch := rabbitReceiver.Channel
	if err := ch.Qos(10, 0, false); err != nil {
		log.Fatalf("setting qos: %v", err)
	}

	quit := make(chan os.Signal, 1)

	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	const consumerTag = "emailworker"

	msgs, err := ch.Consume(
		events.EmailTransactionQueue,
		consumerTag,
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

	done := make(chan struct{})
	go func() {
		defer close(done)

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
	}()

	select {
	case <-quit:
		log.Println("shutting down worker...")

		// Cancel stops new deliveries and closes msgs, which ends the loop above
		// once the deliveries already prefetched have been handled.
		if err := ch.Cancel(consumerTag, false); err != nil {
			log.Printf("cancelling consumer: %v", err)
		}

	case amqpErr := <-rabbitReceiver.Lost():
		// Nothing to close: the connection is already gone. Exiting non-zero
		// lets the restart policy redial.
		log.Printf("broker connection lost: %v", amqpErr)
		os.Exit(1)

	case <-done:
		// Reached when the channel closed without the connection dying, e.g.
		// the queue was deleted underneath the consumer.
		log.Println("delivery channel closed, worker exiting")
		os.Exit(1)
	}

	select {
	case <-done:
		log.Println("in-flight deliveries finished")
	case <-time.After(30 * time.Second):
		log.Println("shutdown timeout; some deliveries will be redelivered")
	}
}

func handle(d amqp.Delivery, dispatcher *emailworker.Dispatcher) emailworker.HandlerResponse {

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return dispatcher.Handle(ctx, d.Body)
}
