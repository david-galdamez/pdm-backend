package events

import "context"

type Publisher interface {
	Publish(financeId uint, isSaving bool) error
}

type Subscriber interface {
	Subscribe(ctx context.Context, handler func(BroadCastMessage))
}

type PayloadEvent struct {
	Event string `json:"event"`
}

type BroadCastMessage struct {
	FinanceID uint           `json:"finance_id"`
	EventInfo []PayloadEvent `json:"event_info"`
}

func BuildWebSocketEvent(financeId uint, isSaving bool) *BroadCastMessage {
	eventInfo := []PayloadEvent{
		{Event: "finance_summary"},
		{Event: "finance_data"},
		{Event: "transaction_list"},
	}

	if isSaving {
		eventInfo = append(eventInfo, PayloadEvent{Event: "finance_savings"})
	}

	return &BroadCastMessage{
		FinanceID: financeId,
		EventInfo: eventInfo,
	}
}
