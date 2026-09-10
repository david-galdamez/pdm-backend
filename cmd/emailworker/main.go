package main

import (
	"log"
	"pdm-backend/events"
	"pdm-backend/internal/config"
)

func main() {
	cfg := config.Get()

	rabbitPublisher, err := events.NewRabbitPublisher(cfg.RABBIT_URL)
	if err != nil {
		log.Fatalf("Failed to create RabbitMQ publisher: %v", err)
	}
	defer rabbitPublisher.Close()

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
		log.Printf(" [%s] %s", d.MessageId, d.Body)
		d.Ack(false)
	}

	log.Println("delivery channel closed, worker exiting")
}
