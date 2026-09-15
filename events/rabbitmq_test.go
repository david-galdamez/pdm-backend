// These cases talk to a real broker and need the amqp091 dependency, so they
// are behind a build tag: `go test ./...` stays green on a machine (and in CI)
// with neither. Run them with:
//
//	RABBITMQ_TEST_URL=amqp://guest:guest@localhost:5672/ go test -race -tags rabbitmq ./events/...

package events

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// rabbitURL is the broker the integration cases below talk to. They skip when
// it is unset so `go test ./...` still passes on a machine with no broker,
// matching how the repositories suite treats Postgres.
func rabbitURL(t *testing.T) string {
	t.Helper()

	url := os.Getenv("RABBITMQ_TEST_URL")
	if url == "" {
		t.Skip("no broker; set RABBITMQ_TEST_URL to run the rabbitmq integration cases")
	}

	return url
}

// A broker that is not answering must surface as an error the caller can log
// and fall back on, never a panic or a nil publisher that fails later.
func TestNewRabbitPublisherReportsAnUnreachableBroker(t *testing.T) {
	publisher, err := NewRabbitPublisher("amqp://guest:guest@127.0.0.1:1/")
	if err == nil {
		if publisher != nil {
			publisher.Close()
		}

		t.Fatal("NewRabbitPublisher returned no error for an unreachable broker")
	}

	if publisher != nil {
		t.Error("NewRabbitPublisher returned a publisher alongside an error")
	}
}

func TestNewRabbitPublisherRejectsAMalformedURL(t *testing.T) {
	if _, err := NewRabbitPublisher("not-a-url"); err == nil {
		t.Fatal("NewRabbitPublisher accepted a malformed URL")
	}
}

// Declaring the topology from the publisher is what lets either side start
// first; the worker declares the same objects with the same arguments.
func TestRabbitPublisherDeclaresTopology(t *testing.T) {
	url := rabbitURL(t)

	freshQueues(t, url)

	publisher, err := NewRabbitPublisher(url)
	if err != nil {
		t.Fatalf("connecting to the broker: %v", err)
	}
	defer publisher.Close()

	// Re-declaring with the same arguments has to succeed. A mismatch against
	// what is already on the broker fails the channel with PRECONDITION_FAILED,
	// which is exactly the drift this guards against.
	second, err := NewRabbitPublisher(url)
	if err != nil {
		t.Fatalf("re-declaring the topology: %v", err)
	}

	second.Close()
}

// The end-to-end path: what the controller publishes is what the worker reads.
func TestRabbitPublisherRoundTrip(t *testing.T) {
	url := rabbitURL(t)

	freshQueues(t, url)

	publisher, err := NewRabbitPublisher(url)
	if err != nil {
		t.Fatalf("connecting to the broker: %v", err)
	}
	defer publisher.Close()

	event := BuildTransactionEmailEvent(7, 42, 2, 150.25, "Groceries")

	if err := publisher.PublishTransactionEmail(context.Background(), event); err != nil {
		t.Fatalf("publishing: %v", err)
	}

	body, delivery := consumeOne(t, url, 5*time.Second)

	var received TransactionEmailEvent
	if err := json.Unmarshal(body, &received); err != nil {
		t.Fatalf("unmarshalling the delivered body: %v", err)
	}

	if received.ID != event.ID {
		t.Errorf("delivered ID = %q, want %q", received.ID, event.ID)
	}

	if received.Amount != event.Amount || received.Description != event.Description {
		t.Errorf("delivered payload = (%v, %q), want (%v, %q)",
			received.Amount, received.Description, event.Amount, event.Description)
	}

	if delivery.ContentType != "application/json" {
		t.Errorf("ContentType = %q, want application/json", delivery.ContentType)
	}

	// A broker restart between the 201 and the send must not lose the email.
	if delivery.DeliveryMode != 2 {
		t.Errorf("DeliveryMode = %d, want 2 (persistent)", delivery.DeliveryMode)
	}

	// MessageId carries the dedupe key without the consumer having to parse
	// the body first.
	if delivery.MessageId != event.ID {
		t.Errorf("MessageId = %q, want %q", delivery.MessageId, event.ID)
	}
}

// Gin serves requests concurrently, and an amqp channel is not safe for
// concurrent use: two goroutines interleaving frames corrupt the stream. Run
// this one with -race.
func TestRabbitPublisherIsSafeForConcurrentUse(t *testing.T) {
	url := rabbitURL(t)

	freshQueues(t, url)

	publisher, err := NewRabbitPublisher(url)
	if err != nil {
		t.Fatalf("connecting to the broker: %v", err)
	}
	defer publisher.Close()

	const publishers = 32

	var wg sync.WaitGroup
	errs := make(chan error, publishers)

	for i := 0; i < publishers; i++ {
		wg.Add(1)

		go func(i int) {
			defer wg.Done()

			if err := publisher.PublishTransactionEmail(
				context.Background(),
				BuildTransactionEmailEvent(uint(i+1), 1, 2, float64(i), "concurrent"),
			); err != nil {
				errs <- err
			}
		}(i)
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Errorf("concurrent publish failed: %v", err)
	}
}

// Publishing must not block a request forever when the broker has stopped
// reading, so the call honours a deadline.
func TestRabbitPublisherHonoursContextDeadline(t *testing.T) {
	url := rabbitURL(t)

	freshQueues(t, url)

	publisher, err := NewRabbitPublisher(url)
	if err != nil {
		t.Fatalf("connecting to the broker: %v", err)
	}
	defer publisher.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancel()

	time.Sleep(time.Millisecond)

	// Either it published before noticing, or it reported the deadline. What it
	// must not do is hang.
	done := make(chan error, 1)
	go func() {
		done <- publisher.PublishTransactionEmail(ctx, BuildTransactionEmailEvent(1, 1, 2, 1, ""))
	}()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("PublishTransactionEmail ignored the context deadline and hung")
	}
}

// consumeOne drains a single delivery from the email queue, so a case can
// assert on what the publisher actually put on the wire. It acks what it
// reads, leaving the queue empty for the next case.
func consumeOne(t *testing.T, url string, timeout time.Duration) ([]byte, amqp.Delivery) {
	t.Helper()

	conn, err := amqp.Dial(url)
	if err != nil {
		t.Fatalf("dialling the broker: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	ch, err := conn.Channel()
	if err != nil {
		t.Fatalf("opening a channel: %v", err)
	}
	t.Cleanup(func() { ch.Close() })

	deliveries, err := ch.Consume(EmailTransactionQueue, "", false, false, false, false, nil)
	if err != nil {
		t.Fatalf("consuming from %s: %v", EmailTransactionQueue, err)
	}

	select {
	case delivery := <-deliveries:
		if err := delivery.Ack(false); err != nil {
			t.Fatalf("acking the delivery: %v", err)
		}

		return delivery.Body, delivery

	case <-time.After(timeout):
		t.Fatalf("no delivery on %s within %v", EmailTransactionQueue, timeout)
		return nil, amqp.Delivery{}
	}
}

// An event whose Type is not bound to any queue matches nothing on the
// exchange. The broker hands it straight back, and that has to reach the caller
// as an error rather than a log line, because the publish "succeeded" as far as
// the socket is concerned.
func TestPublishReportsAnUnroutableEvent(t *testing.T) {
	url := rabbitURL(t)

	freshQueues(t, url)

	publisher, err := NewRabbitPublisher(url)
	if err != nil {
		t.Fatalf("connecting to the broker: %v", err)
	}
	defer publisher.Close()

	event := BuildTransactionEmailEvent(7, 42, 2, 10, "unroutable")
	event.Type = "transaction.nothing.is.bound.to.this"

	err = publisher.PublishTransactionEmail(context.Background(), event)
	if err == nil {
		t.Fatal("PublishTransactionEmail reported success for an event no queue is bound to")
	}

	if !errors.Is(err, ErrUnroutable) {
		t.Fatalf("err = %v, want it to wrap ErrUnroutable", err)
	}

	// The routing key is the whole diagnosis: it says which binding is missing.
	if !strings.Contains(err.Error(), event.Type) {
		t.Errorf("error %q does not name the routing key %q", err, event.Type)
	}
}

// Republishing a returned event would hit the same missing binding and be
// returned again, forever. The publisher must report and stop.
func TestPublishDoesNotRetryAnUnroutableEvent(t *testing.T) {
	url := rabbitURL(t)

	freshQueues(t, url)

	publisher, err := NewRabbitPublisher(url)
	if err != nil {
		t.Fatalf("connecting to the broker: %v", err)
	}
	defer publisher.Close()

	event := BuildTransactionEmailEvent(7, 42, 2, 10, "unroutable")
	event.Type = "transaction.nothing.is.bound.to.this"

	done := make(chan error, 1)
	go func() {
		done <- publisher.PublishTransactionEmail(context.Background(), event)
	}()

	select {
	case err := <-done:
		if !errors.Is(err, ErrUnroutable) {
			t.Fatalf("err = %v, want ErrUnroutable", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("PublishTransactionEmail never returned: it is retrying a message that can never be routed")
	}
}

// The return is asynchronous and arrives on a shared channel, so a stale one
// must not be charged to the next publish. This is the case the drain at the
// top of PublishTransactionEmail exists for.
func TestPublishDoesNotBlameAStaleReturnOnTheNextEvent(t *testing.T) {
	url := rabbitURL(t)

	freshQueues(t, url)

	publisher, err := NewRabbitPublisher(url)
	if err != nil {
		t.Fatalf("connecting to the broker: %v", err)
	}
	defer publisher.Close()

	unroutable := BuildTransactionEmailEvent(7, 42, 2, 10, "unroutable")
	unroutable.Type = "transaction.nothing.is.bound.to.this"

	if err := publisher.PublishTransactionEmail(context.Background(), unroutable); !errors.Is(err, ErrUnroutable) {
		t.Fatalf("setup publish err = %v, want ErrUnroutable", err)
	}

	// A well-formed event published straight afterwards routes fine and must be
	// reported as such.
	if err := publisher.PublishTransactionEmail(
		context.Background(),
		BuildTransactionEmailEvent(7, 42, 2, 10, "routable"),
	); err != nil {
		t.Fatalf("a routable event was reported as failed: %v", err)
	}

	consumeOne(t, url, 5*time.Second)
}

// A successful publish means the broker confirmed it, not that the frame was
// written. Without the confirm this passes even when the message never lands.
func TestPublishWaitsForTheBrokerConfirm(t *testing.T) {
	url := rabbitURL(t)

	freshQueues(t, url)

	publisher, err := NewRabbitPublisher(url)
	if err != nil {
		t.Fatalf("connecting to the broker: %v", err)
	}
	defer publisher.Close()

	event := BuildTransactionEmailEvent(7, 42, 2, 10, "confirmed")

	if err := publisher.PublishTransactionEmail(context.Background(), event); err != nil {
		t.Fatalf("publishing: %v", err)
	}

	// The confirm means the queue already holds it, so it is readable with no
	// grace period.
	body, _ := consumeOne(t, url, time.Second)

	var received TransactionEmailEvent
	if err := json.Unmarshal(body, &received); err != nil {
		t.Fatalf("unmarshalling the delivered body: %v", err)
	}

	if received.ID != event.ID {
		t.Errorf("delivered ID = %q, want %q", received.ID, event.ID)
	}
}

// Publishing on a closed channel must come back as an error, not a panic or a
// silent success: it is what a dropped connection looks like to a live handler.
func TestPublishReportsAClosedChannel(t *testing.T) {
	url := rabbitURL(t)

	freshQueues(t, url)

	publisher, err := NewRabbitPublisher(url)
	if err != nil {
		t.Fatalf("connecting to the broker: %v", err)
	}

	publisher.Close()

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("panicked publishing on a closed publisher: %v", r)
		}
	}()

	if err := publisher.PublishTransactionEmail(
		context.Background(),
		BuildTransactionEmailEvent(7, 42, 2, 10, ""),
	); err == nil {
		t.Fatal("PublishTransactionEmail reported success on a closed publisher")
	}
}

// The dead-letter binding is what makes a rejected email inspectable instead of
// silently discarded. The broker republishes to the DLX with
// x-dead-letter-routing-key, so the dead queue must be bound with that key and
// not the original one.
func TestDeadLetteredEventReachesTheDeadQueue(t *testing.T) {
	url := rabbitURL(t)

	freshQueues(t, url)

	publisher, err := NewRabbitPublisher(url)
	if err != nil {
		t.Fatalf("connecting to the broker: %v", err)
	}
	defer publisher.Close()

	event := BuildTransactionEmailEvent(7, 42, 2, 10, "dead lettered")

	if err := publisher.PublishTransactionEmail(context.Background(), event); err != nil {
		t.Fatalf("publishing: %v", err)
	}

	// Reject it the way the worker rejects a send it could not complete.
	nackOne(t, url, EmailTransactionQueue)

	body := readFrom(t, url, EmailTransactionDeadQueue, 5*time.Second)

	var dead TransactionEmailEvent
	if err := json.Unmarshal(body, &dead); err != nil {
		t.Fatalf("unmarshalling the dead-lettered body: %v", err)
	}

	if dead.ID != event.ID {
		t.Errorf("dead-lettered ID = %q, want %q", dead.ID, event.ID)
	}
}

// channelTo opens a throwaway channel on its own connection, so a case can act
// on the broker without disturbing the publisher under test.
func channelTo(t *testing.T, url string) *amqp.Channel {
	t.Helper()

	conn, err := amqp.Dial(url)
	if err != nil {
		t.Fatalf("dialling the broker: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	ch, err := conn.Channel()
	if err != nil {
		t.Fatalf("opening a channel: %v", err)
	}
	t.Cleanup(func() { ch.Close() })

	return ch
}

// drain empties a queue so a case starts from a known state; the queues are
// durable and survive both the process and the broker.
func drain(t *testing.T, url, queue string) {
	t.Helper()

	if _, err := channelTo(t, url).QueuePurge(queue, false); err != nil {
		t.Fatalf("purging %s: %v", queue, err)
	}
}

// freshQueues empties both queues before and after a case. Without it the cases
// are order-dependent: every message any of them publishes stays in the durable
// queue, so the next one to read asserts against a leftover instead of its own
// event.
func freshQueues(t *testing.T, url string) {
	t.Helper()

	purge := func() {
		drain(t, url, EmailTransactionQueue)
		drain(t, url, EmailTransactionDeadQueue)
	}

	purge()
	t.Cleanup(purge)
}

// nackOne rejects a single delivery without requeueing it, which is what sends
// it to the dead-letter exchange.
func nackOne(t *testing.T, url, queue string) {
	t.Helper()

	ch := channelTo(t, url)

	delivery, ok, err := ch.Get(queue, false)
	if err != nil {
		t.Fatalf("getting from %s: %v", queue, err)
	}

	if !ok {
		t.Fatalf("%s was empty; the publish did not land", queue)
	}

	if err := delivery.Nack(false, false); err != nil {
		t.Fatalf("nacking: %v", err)
	}
}

// readFrom waits for one message on a queue and acks it, leaving the queue
// empty for the next case.
func readFrom(t *testing.T, url, queue string, timeout time.Duration) []byte {
	t.Helper()

	ch := channelTo(t, url)

	deliveries, err := ch.Consume(queue, "", false, false, false, false, nil)
	if err != nil {
		t.Fatalf("consuming from %s: %v", queue, err)
	}

	select {
	case delivery := <-deliveries:
		if err := delivery.Ack(false); err != nil {
			t.Fatalf("acking: %v", err)
		}

		return delivery.Body

	case <-time.After(timeout):
		t.Fatalf("no delivery on %s within %v", queue, timeout)
		return nil
	}
}
