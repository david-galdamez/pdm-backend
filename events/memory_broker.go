package events

import (
	"context"
	"log"
)

type MemoryBroker struct {
	messages chan BroadCastMessage
}

func NewMemoryBroker() *MemoryBroker {
	return &MemoryBroker{messages: make(chan BroadCastMessage, 100)}
}

func (mb *MemoryBroker) Publish(financeId uint, isSaving bool) error {
	select {
	case mb.messages <- *BuildWebSocketEvent(financeId, isSaving):
	default:
		log.Printf("broadcast buffer full, dropping event for finance: %v \n", financeId)
	}
	return nil
}

func (mb *MemoryBroker) Subscribe(ctx context.Context, handler func(BroadCastMessage)) {
	for {
		select {
		case msg := <-mb.messages:
			handler(msg)
		case <-ctx.Done():
			return
		}
	}
}
