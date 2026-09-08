package events

import (
	"context"
	"testing"
	"time"
)

// subscribe runs mb.Subscribe in the background and returns a channel carrying
// every message it dispatches. The subscriber is stopped and its exit verified
// during test cleanup.
func subscribe(t *testing.T, mb *MemoryBroker) <-chan BroadCastMessage {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	got := make(chan BroadCastMessage, 128)
	done := make(chan struct{})

	go func() {
		mb.Subscribe(ctx, func(msg BroadCastMessage) { got <- msg })
		close(done)
	}()

	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Error("Subscribe did not return after context cancel")
		}
	})

	return got
}

// recv reads one dispatched message or fails the test if none arrives.
func recv(t *testing.T, ch <-chan BroadCastMessage) BroadCastMessage {
	t.Helper()

	select {
	case msg := <-ch:
		return msg
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for a broadcast message")
		return BroadCastMessage{}
	}
}

func eventSet(msg BroadCastMessage) map[string]bool {
	set := make(map[string]bool, len(msg.EventInfo))
	for _, e := range msg.EventInfo {
		set[e.Event] = true
	}
	return set
}

func TestMemoryBrokerPublishDelivers(t *testing.T) {
	mb := NewMemoryBroker()
	got := subscribe(t, mb)

	if err := mb.Publish(42, false); err != nil {
		t.Fatalf("Publish returned error: %v", err)
	}

	msg := recv(t, got)
	if msg.FinanceID != 42 {
		t.Errorf("FinanceID = %d, want 42", msg.FinanceID)
	}

	events := eventSet(msg)
	for _, want := range []string{"finance_summary", "finance_data", "transaction_list"} {
		if !events[want] {
			t.Errorf("missing event %q in %v", want, events)
		}
	}
	if events["finance_savings"] {
		t.Error("non-saving publish should not include finance_savings")
	}
}

func TestMemoryBrokerPublishSavingAddsSavingsEvent(t *testing.T) {
	mb := NewMemoryBroker()
	got := subscribe(t, mb)

	if err := mb.Publish(7, true); err != nil {
		t.Fatalf("Publish returned error: %v", err)
	}

	if !eventSet(recv(t, got))["finance_savings"] {
		t.Error("saving publish should include finance_savings")
	}
}

func TestMemoryBrokerPreservesOrder(t *testing.T) {
	mb := NewMemoryBroker()
	got := subscribe(t, mb)

	for i := uint(1); i <= 20; i++ {
		if err := mb.Publish(i, false); err != nil {
			t.Fatalf("Publish(%d) returned error: %v", i, err)
		}
	}

	for i := uint(1); i <= 20; i++ {
		if msg := recv(t, got); msg.FinanceID != i {
			t.Fatalf("message %d out of order: got FinanceID %d", i, msg.FinanceID)
		}
	}
}

func TestMemoryBrokerSubscribeStopsOnContextCancel(t *testing.T) {
	mb := NewMemoryBroker()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	go func() {
		mb.Subscribe(ctx, func(BroadCastMessage) {})
		close(done)
	}()

	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Subscribe did not return after context cancel")
	}
}

func TestMemoryBrokerPublishDoesNotBlockWhenBufferFull(t *testing.T) {
	mb := NewMemoryBroker()

	// No subscriber drains the channel, so after the first 100 sends it stays
	// full. Every Publish must still return promptly, dropping the overflow
	// rather than blocking the HTTP request that called it.
	done := make(chan struct{})
	go func() {
		for i := 0; i < 300; i++ {
			if err := mb.Publish(uint(i), false); err != nil {
				t.Errorf("Publish returned error: %v", err)
				break
			}
		}
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Publish blocked when the buffer was full")
	}
}
