package events

import (
	"context"
	"time"

	"github.com/google/uuid"
)

var (
	EventsExchange               = "pdm.events"
	DeadLetterExchange           = "pdm.dlx"
	EmailTransactionQueue        = "email.transaction"
	EmailTransactionDeadQueue    = "email.transaction.dead"
	RoutingKeyTransactionCreated = "transaction.created"
	EmailEventVersion            = 1
)

type EmailPublisher interface {
	PublishTransactionEmail(ctx context.Context, event TransactionEmailEvent) error
}

type TransactionEmailEvent struct {
	ID          string    `json:"id"`
	Type        string    `json:"type"`
	Version     int       `json:"version"`
	OccurredAt  time.Time `json:"occurred_at"`
	FinanceID   uint      `json:"finance_id"`
	ActorUserID uint      `json:"actor_user_id"`
	EntryTypeID uint      `json:"entry_type_id"`
	Amount      float64   `json:"amount"`
	Description string    `json:"description"`
}

func BuildTransactionEmailEvent(financeId, actorUserId, entryTypeId uint, amount float64, description string) TransactionEmailEvent {
	return TransactionEmailEvent{
		ID:          uuid.NewString(),
		Type:        RoutingKeyTransactionCreated,
		Version:     EmailEventVersion,
		OccurredAt:  time.Now(),
		FinanceID:   financeId,
		ActorUserID: actorUserId,
		EntryTypeID: entryTypeId,
		Amount:      amount,
		Description: description,
	}
}

func TryPublishTransactionEmail(ctx context.Context, publisher EmailPublisher, emailEvent TransactionEmailEvent) {

}

type NoopEmailPublisher struct {
}

func (n NoopEmailPublisher) PublishTransactionEmail(ctx context.Context, event TransactionEmailEvent) error {
	return nil
}
