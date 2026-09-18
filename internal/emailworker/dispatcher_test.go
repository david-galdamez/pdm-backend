package emailworker

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"

	"pdm-backend/email"
	"pdm-backend/events"
	"pdm-backend/repositories"
)

type stubTargets struct {
	targets *repositories.TransactionEmailTargets
	err     error

	mu    sync.Mutex
	calls [][2]uint
}

func (s *stubTargets) GetTransactionEmailTargets(financeId, actorUserId uint) (*repositories.TransactionEmailTargets, error) {
	s.mu.Lock()
	s.calls = append(s.calls, [2]uint{financeId, actorUserId})
	s.mu.Unlock()

	return s.targets, s.err
}

type stubSender struct {
	err error

	mu   sync.Mutex
	sent []email.Message
}

func (s *stubSender) Send(_ context.Context, message email.Message) error {
	s.mu.Lock()
	s.sent = append(s.sent, message)
	s.mu.Unlock()

	return s.err
}

func (s *stubSender) messages() []email.Message {
	s.mu.Lock()
	defer s.mu.Unlock()

	return append([]email.Message(nil), s.sent...)
}

func twoMembers() *repositories.TransactionEmailTargets {
	return &repositories.TransactionEmailTargets{
		FinanceName: "Household",
		ActorName:   "David",
		Recipients: []repositories.Recipient{
			{UserID: 2, Name: "Ana", Email: "ana@example.test"},
			{UserID: 3, Name: "Luis", Email: "luis@example.test"},
		},
	}
}

func encode(t *testing.T, event events.TransactionEmailEvent) []byte {
	t.Helper()

	body, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshalling the event: %v", err)
	}

	return body
}

func TestHandleSendsOneMessagePerRecipient(t *testing.T) {
	targets := &stubTargets{targets: twoMembers()}
	sender := &stubSender{}

	dispatcher := NewDispatcher(targets, sender)

	event := events.BuildTransactionEmailEvent(7, 42, 2, 150.25, "Groceries")

	if result := dispatcher.Handle(context.Background(), encode(t, event)); result != ResultAck {
		t.Fatalf("Handle = %v, want ResultAck", result)
	}

	messages := sender.messages()

	// One message each: a single message addressed to everyone would show the
	// whole membership list to every member.
	if len(messages) != 2 {
		t.Fatalf("sender saw %d messages, want 2", len(messages))
	}

	for _, message := range messages {
		if len(message.To) != 1 {
			t.Errorf("message addressed to %v, want a single recipient", message.To)
		}
	}

	// The actor is excluded at the query, so the worker asks for it by id
	// rather than filtering afterwards.
	if len(targets.calls) != 1 || targets.calls[0] != [2]uint{7, 42} {
		t.Errorf("lookup calls = %v, want [[7 42]]", targets.calls)
	}
}

// Ack means "safe to delete". Acking before the send turns a crash mid-send
// into a lost email.
func TestHandleDeadLettersAFailedSend(t *testing.T) {
	dispatcher := NewDispatcher(&stubTargets{targets: twoMembers()}, &stubSender{err: errors.New("smtp is down")})

	event := events.BuildTransactionEmailEvent(7, 42, 2, 10, "")

	if result := dispatcher.Handle(context.Background(), encode(t, event)); result != ResultDead {
		t.Fatalf("Handle = %v, want ResultDead after a send failure", result)
	}
}

// The database being briefly unavailable is transient, but it is still not an
// ack: the message goes to the dead-letter queue where it can be inspected.
func TestHandleDeadLettersALookupFailure(t *testing.T) {
	sender := &stubSender{}

	dispatcher := NewDispatcher(&stubTargets{err: errors.New("connection refused")}, sender)

	event := events.BuildTransactionEmailEvent(7, 42, 2, 10, "")

	if result := dispatcher.Handle(context.Background(), encode(t, event)); result != ResultDead {
		t.Fatalf("Handle = %v, want ResultDead after a lookup failure", result)
	}

	if len(sender.messages()) != 0 {
		t.Error("the worker sent mail without resolving the recipients")
	}
}

// Malformed bytes will never parse, however many times they are redelivered.
// Requeueing them is an infinite loop; the ack drops them after the log line.
func TestHandleAcksAMalformedBody(t *testing.T) {
	sender := &stubSender{}

	dispatcher := NewDispatcher(&stubTargets{targets: twoMembers()}, sender)

	if result := dispatcher.Handle(context.Background(), []byte("{not json")); result != ResultAck {
		t.Fatalf("Handle = %v, want ResultAck for a body that will never parse", result)
	}

	if len(sender.messages()) != 0 {
		t.Error("the worker sent mail for an unparseable body")
	}
}

// Rolling deploys mean an old worker receives a new instance's messages. An
// unknown type is ignored, not fatal, and not retried.
func TestHandleAcksAnUnknownEventType(t *testing.T) {
	sender := &stubSender{}

	dispatcher := NewDispatcher(&stubTargets{targets: twoMembers()}, sender)

	event := events.BuildTransactionEmailEvent(7, 42, 2, 10, "")
	event.Type = "transaction.reversed"

	if result := dispatcher.Handle(context.Background(), encode(t, event)); result != ResultAck {
		t.Fatalf("Handle = %v, want ResultAck for an unknown type", result)
	}

	if len(sender.messages()) != 0 {
		t.Error("the worker sent mail for an event type it does not understand")
	}
}

func TestHandleAcksAnUnknownEventVersion(t *testing.T) {
	sender := &stubSender{}

	dispatcher := NewDispatcher(&stubTargets{targets: twoMembers()}, sender)

	event := events.BuildTransactionEmailEvent(7, 42, 2, 10, "")
	event.Version = events.EmailEventVersion + 1

	if result := dispatcher.Handle(context.Background(), encode(t, event)); result != ResultAck {
		t.Fatalf("Handle = %v, want ResultAck for a future version", result)
	}

	if len(sender.messages()) != 0 {
		t.Error("the worker sent mail for a payload shape it cannot read")
	}
}

// The last other member may have left between the publish and the consume.
// Nothing to send is a success, not a dead letter.
func TestHandleAcksWhenThereAreNoRecipients(t *testing.T) {
	sender := &stubSender{}

	targets := &stubTargets{targets: &repositories.TransactionEmailTargets{FinanceName: "Household", ActorName: "David"}}

	dispatcher := NewDispatcher(targets, sender)

	event := events.BuildTransactionEmailEvent(7, 42, 2, 10, "")

	if result := dispatcher.Handle(context.Background(), encode(t, event)); result != ResultAck {
		t.Fatalf("Handle = %v, want ResultAck when the finance has no other members", result)
	}

	if len(sender.messages()) != 0 {
		t.Errorf("sender saw %d messages, want 0", len(sender.messages()))
	}
}

// A finance that was deleted between publish and consume has no targets at
// all. Retrying will never find them.
func TestHandleAcksAMissingFinance(t *testing.T) {
	sender := &stubSender{}

	dispatcher := NewDispatcher(&stubTargets{targets: nil}, sender)

	event := events.BuildTransactionEmailEvent(7, 42, 2, 10, "")

	if result := dispatcher.Handle(context.Background(), encode(t, event)); result != ResultAck {
		t.Fatalf("Handle = %v, want ResultAck for a finance that no longer exists", result)
	}

	if len(sender.messages()) != 0 {
		t.Error("the worker sent mail for a finance it could not resolve")
	}
}

// One bad address must not cost the other members their email.
func TestHandlePartialSendFailureStillDeadLetters(t *testing.T) {
	sender := &failOnceSender{failFor: "ana@example.test"}

	dispatcher := NewDispatcher(&stubTargets{targets: twoMembers()}, sender)

	event := events.BuildTransactionEmailEvent(7, 42, 2, 10, "")

	result := dispatcher.Handle(context.Background(), encode(t, event))

	if len(sender.attempted) != 2 {
		t.Errorf("worker attempted %d sends, want 2: it stopped at the first failure", len(sender.attempted))
	}

	if result != ResultDead {
		t.Errorf("Handle = %v, want ResultDead when a recipient could not be reached", result)
	}
}

type failOnceSender struct {
	failFor   string
	attempted []string
}

func (s *failOnceSender) Send(_ context.Context, message email.Message) error {
	address := strings.Join(message.To, ",")
	s.attempted = append(s.attempted, address)

	if address == s.failFor {
		return errors.New("mailbox unavailable")
	}

	return nil
}

// The rendered mail has to name the finance and the person who recorded the
// transaction; without them the reader cannot tell which of their shared
// finances moved.
func TestHandleRendersTheFinanceAndActor(t *testing.T) {
	sender := &stubSender{}

	dispatcher := NewDispatcher(&stubTargets{targets: twoMembers()}, sender)

	event := events.BuildTransactionEmailEvent(7, 42, 2, 150.25, "Groceries")

	if result := dispatcher.Handle(context.Background(), encode(t, event)); result != ResultAck {
		t.Fatalf("Handle = %v, want ResultAck", result)
	}

	messages := sender.messages()
	if len(messages) == 0 {
		t.Fatal("no messages were sent")
	}

	for _, want := range []string{"Household", "David", "Groceries", "150.25"} {
		if !strings.Contains(messages[0].HTMLBody, want) {
			t.Errorf("body is missing %q:\n%s", want, messages[0].HTMLBody)
		}
	}

	// Each member is greeted by their own name, which is the reason the worker
	// renders per recipient rather than once.
	if len(messages) > 1 && messages[0].HTMLBody == messages[1].HTMLBody {
		t.Error("both members received an identical body; the template is not personalised")
	}
}
