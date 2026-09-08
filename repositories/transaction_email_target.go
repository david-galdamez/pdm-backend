package repositories

type Recipient struct {
	UserID uint
	Name   string
	Email  string
}

type TransactionEmailTargets struct {
	FinanceName string
	ActorName   string
	Recipients  []Recipient
}
