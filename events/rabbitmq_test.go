// These cases talk to a real broker and need the amqp091 dependency, so they
// are behind a build tag: `go test ./...` stays green on a machine (and in CI)
// with neither. Run them with:
//
//	RABBITMQ_TEST_URL=amqp://guest:guest@localhost:5672/ go test -race -tags rabbitmq ./events/...

package events

import (
	"context"
	"encoding/json"
	"os"
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
