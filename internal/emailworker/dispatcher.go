package emailworker

import (
	"context"
	"encoding/json"
	"log"
	"pdm-backend/email"
	"pdm-backend/events"
	"pdm-backend/models"
	"pdm-backend/repositories"
)

type HandlerResponse int

const (
	ResultAck HandlerResponse = iota
	ResultDead
	ResultRetry
)

type Target interface {
	GetTransactionEmailTargets(financeId, userId uint) (*repositories.TransactionEmailTargets, error)
}

type Sender interface {
	Send(ctx context.Context, message email.Message) error
}

type Dispatcher struct {
	target Target
	sender Sender
}

type EmailTarget struct {
}

func (et EmailTarget) GetTransactionEmailTargets(financeId, userId uint) (*repositories.TransactionEmailTargets, error) {

	sharedRepo := repositories.NewSharedFinanceRepository(repositories.GetDB())

	return sharedRepo.GetTransactionEmailTargets(financeId, userId)
}

func (d *Dispatcher) Handle(ctx context.Context, body []byte) HandlerResponse {

	var emailEvent events.TransactionEmailEvent

	err := json.Unmarshal(body, &emailEvent)
	if err != nil {
		return ResultDead
	}

	targets, err := d.target.GetTransactionEmailTargets(emailEvent.FinanceID, emailEvent.ActorUserID)
	if err != nil {
		return ResultRetry
	}
	if targets == nil || len(targets.Recipients) == 0 {
		return ResultAck
	}

	var failed int
	for _, r := range targets.Recipients {
		msg, err := email.RenderTransactionCreated(email.TransactionEmailData{
			To:            r.Email,
			RecipientName: r.Name,
			ActorName:     targets.ActorName,
			FinanceName:   targets.FinanceName,
			Description:   emailEvent.Description,
			Amount:        emailEvent.Amount,
			IsIncome:      emailEvent.EntryTypeID == models.EntryTypeIncome,
			OccurredAt:    emailEvent.OccurredAt,
		})
		if err != nil {
			log.Printf("render for %s failed: %v", r.Email, err)
			continue
		}
		if err := d.sender.Send(ctx, msg); err != nil {
			log.Printf("send to %s failed: %v", r.Email, err)
			failed++
		}
	}

	return ResultAck
}

func NewDispatcher(target Target, sender Sender) Dispatcher {
	return Dispatcher{
		target: target,
		sender: sender,
	}
}
