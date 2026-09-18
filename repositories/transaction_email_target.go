package repositories

type Recipient struct {
	UserID uint   `json:"user_id"`
	Name   string `json:"name"`
	Email  string `json:"email"`
}

type TransactionEmailTargets struct {
	FinanceName string      `json:"finance_name"`
	ActorName   string      `json:"actor_name"`
	Recipients  []Recipient `json:"recipients"`
}
