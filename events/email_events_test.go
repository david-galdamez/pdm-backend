package events

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

// The broker topology is named in one place so the publisher and the worker
// declare the same objects; a rename on one side only would silently create a
// second, unconsumed queue.
func TestTopologyNames(t *testing.T) {
	cases := []struct {
		name string
		got  string
		want string
	}{
		{"exchange", EventsExchange, "pdm.events"},
		{"dead letter exchange", DeadLetterExchange, "pdm.dlx"},
		{"queue", EmailTransactionQueue, "email.transaction"},
		{"dead letter queue", EmailTransactionDeadQueue, "email.transaction.dead"},
		{"routing key", RoutingKeyTransactionCreated, "transaction.created"},
	}

	for _, tc := range cases {
		if tc.got != tc.want {
			t.Errorf("%s = %q, want %q", tc.name, tc.got, tc.want)
		}
	}
}

func TestBuildTransactionEmailEventStampsEnvelope(t *testing.T) {
	before := time.Now()

	event := BuildTransactionEmailEvent(7, 42, 2, 150.25, "Groceries")

	if event.Type != RoutingKeyTransactionCreated {
		t.Errorf("Type = %q, want %q", event.Type, RoutingKeyTransactionCreated)
	}

	// Rolling deploys mean an old worker will see messages produced by a new
	// instance, so every message has to say which shape it is.
	if event.Version != EmailEventVersion {
		t.Errorf("Version = %d, want %d", event.Version, EmailEventVersion)
	}

	if event.OccurredAt.Before(before) {
		t.Errorf("OccurredAt = %v, want at or after %v", event.OccurredAt, before)
	}

	if event.FinanceID != 7 || event.ActorUserID != 42 || event.EntryTypeID != 2 {
		t.Errorf("ids = (%d, %d, %d), want (7, 42, 2)", event.FinanceID, event.ActorUserID, event.EntryTypeID)
	}

	if event.Amount != 150.25 || event.Description != "Groceries" {
		t.Errorf("amount/description = (%v, %q), want (150.25, \"Groceries\")", event.Amount, event.Description)
	}
}

// Delivery is at-least-once, so a consumer that wants to dedupe needs a stable
// per-event identifier that is never reused.
func TestBuildTransactionEmailEventIDIsUnique(t *testing.T) {
	seen := make(map[string]bool)

	for i := 0; i < 100; i++ {
		id := BuildTransactionEmailEvent(1, 1, 2, 10, "").ID

		if strings.TrimSpace(id) == "" {
			t.Fatal("ID is empty")
		}

		if seen[id] {
			t.Fatalf("ID %q was produced twice", id)
		}

		seen[id] = true
	}
}

// Once it crosses a process the event is bytes, so the JSON names are the
// contract and cannot drift with a field rename.
func TestTransactionEmailEventJSONContract(t *testing.T) {
	event := TransactionEmailEvent{
		ID:          "1f3a",
		Type:        RoutingKeyTransactionCreated,
		Version:     EmailEventVersion,
		OccurredAt:  time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC),
		FinanceID:   7,
		ActorUserID: 42,
		EntryTypeID: 2,
		Amount:      150.25,
		Description: "Groceries",
	}

	encoded, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshalling event: %v", err)
	}

	var fields map[string]any
	if err := json.Unmarshal(encoded, &fields); err != nil {
		t.Fatalf("unmarshalling into a map: %v", err)
	}

	for _, key := range []string{
		"id", "type", "version", "occurred_at", "finance_id",
		"actor_user_id", "entry_type_id", "amount", "description",
	} {
		if _, ok := fields[key]; !ok {
			t.Errorf("missing JSON field %q in %s", key, encoded)
		}
	}

	// Recipient addresses are resolved by the worker from the database. They
	// must never ride along in the payload: this crosses the network and lands
	// in broker logs and the management UI.
	for key := range fields {
		if strings.Contains(key, "email") || strings.Contains(key, "recipient") {
			t.Errorf("event carries recipient data in field %q", key)
		}
	}

	var decoded TransactionEmailEvent
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("round-tripping event: %v", err)
	}

	if !decoded.OccurredAt.Equal(event.OccurredAt) {
		t.Errorf("OccurredAt round-tripped to %v, want %v", decoded.OccurredAt, event.OccurredAt)
	}

	decoded.OccurredAt = event.OccurredAt
	if decoded != event {
		t.Errorf("round-trip = %+v, want %+v", decoded, event)
	}
}

func TestNoopEmailPublisherSucceeds(t *testing.T) {
	var publisher EmailPublisher = NoopEmailPublisher{}

	if err := publisher.PublishTransactionEmail(context.Background(), BuildTransactionEmailEvent(1, 1, 2, 10, "")); err != nil {
		t.Errorf("NoopEmailPublisher returned %v, want nil", err)
	}
}

type stubEmailPublisher struct {
	err      error
	received []TransactionEmailEvent
}

func (s *stubEmailPublisher) PublishTransactionEmail(_ context.Context, event TransactionEmailEvent) error {
	s.received = append(s.received, event)
	return s.err
}

// The transaction is already committed by the time the event is published, so
// a broker that is down must never turn a successful write into a 500. The
// helper is what the controller calls so the failure cannot be forgotten.
func TestTryPublishTransactionEmailSwallowsFailures(t *testing.T) {
	publisher := &stubEmailPublisher{err: errors.New("broker unreachable")}

	TryPublishTransactionEmail(context.Background(), publisher, BuildTransactionEmailEvent(7, 42, 2, 10, ""))

	if len(publisher.received) != 1 {
		t.Fatalf("publisher saw %d events, want 1", len(publisher.received))
	}
}

// main.go wires NoopEmailPublisher when RABBITMQ_URL is unset, but a nil
// publisher reaching a handler must degrade rather than panic mid-request.
func TestTryPublishTransactionEmailToleratesNilPublisher(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("panicked on a nil publisher: %v", r)
		}
	}()

	TryPublishTransactionEmail(context.Background(), nil, BuildTransactionEmailEvent(1, 1, 2, 10, ""))
}

// A cancelled request context must not be the thing that decides whether the
// event is published: the write it describes already happened.
func TestTryPublishTransactionEmailPublishesUnderCancelledContext(t *testing.T) {
	publisher := &stubEmailPublisher{}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	TryPublishTransactionEmail(ctx, publisher, BuildTransactionEmailEvent(7, 42, 2, 10, ""))

	if len(publisher.received) != 1 {
		t.Fatalf("publisher saw %d events, want 1", len(publisher.received))
	}
}
