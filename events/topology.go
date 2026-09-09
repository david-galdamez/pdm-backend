package events

import amqp "github.com/rabbitmq/amqp091-go"

/*
	DeclareTopology declares the exchanges, queues, and bindings for the RabbitMQ topology.

It ensures that the necessary exchanges and queues are created and bound correctly.
This function should be called before publishing or consuming messages to ensure the topology is set up properly.
*/
func DeclareTopology(ch *amqp.Channel) error {

	err := ch.ExchangeDeclare(
		EventsExchange,
		"topic",
		true,  // durable
		false, // auto-deleted
		false, // internal
		false, // no-wait
		nil,   // arguments
	)
	if err != nil {
		return err
	}

	err = ch.ExchangeDeclare(
		DeadLetterExchange,
		"topic",
		true,  // durable
		false, // auto-deleted
		false, // internal
		false, // no-wait
		nil,   // arguments
	)
	if err != nil {
		return err
	}

	_, err = ch.QueueDeclare(
		EmailTransactionQueue,
		true,  // durable
		false, // auto-deleted
		false, // exclusive
		false, // no-wait
		amqp.Table{
			"x-dead-letter-exchange": DeadLetterExchange,
		},
	)
	if err != nil {
		return err
	}

	err = ch.QueueBind(
		EmailTransactionQueue,
		RoutingKeyTransactionCreated,
		EventsExchange,
		false, // no-wait
		nil,   // arguments
	)
	if err != nil {
		return err
	}

	_, err = ch.QueueDeclare(
		EmailTransactionDeadQueue,
		true,  // durable
		false, // auto-deleted
		false, // exclusive
		false, // no-wait
		nil,   // arguments
	)
	if err != nil {
		return err
	}

	err = ch.QueueBind(
		EmailTransactionDeadQueue,
		RoutingKeyTransactionCreated,
		DeadLetterExchange,
		false, // no-wait
		nil,   // arguments
	)
	if err != nil {
		return err
	}

	return nil
}
