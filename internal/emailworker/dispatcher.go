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

type Dispatcher struct {
	target Target
	sender email.Sender
}

type EmailTransactionTarget struct {
}

func (et EmailTransactionTarget) GetTransactionEmailTargets(financeId, userId uint) (*repositories.TransactionEmailTargets, error) {

	sharedRepo := repositories.NewSharedFinanceRepository(repositories.GetDB())

	return sharedRepo.GetTransactionEmailTargets(financeId, userId)
}

func (d *Dispatcher) Handle(ctx context.Context, body []byte) HandlerResponse {

	var emailEvent events.TransactionEmailEvent

	err := json.Unmarshal(body, &emailEvent)
	if err != nil {
		return ResultAck
	}

	if emailEvent.Type != events.RoutingKeyTransactionCreated || emailEvent.Version != events.EmailEventVersion {
		log.Printf("ignoring event %s: type=%q version=%d", emailEvent.ID, emailEvent.Type, emailEvent.Version)
		return ResultAck
	}

	targets, err := d.target.GetTransactionEmailTargets(emailEvent.FinanceID, emailEvent.ActorUserID)
	if err != nil {
		return ResultDead
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

	if failed > 0 {
		return ResultDead
	}

	return ResultAck
}

func NewDispatcher(target Target, sender email.Sender) Dispatcher {
	return Dispatcher{
		target: target,
		sender: sender,
	}
}
