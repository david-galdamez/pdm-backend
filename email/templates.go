package email

import (
	"bytes"
	_ "embed"
	"fmt"
	"html/template"
	"regexp"
	"strconv"
	"strings"
	"time"
)

//go:embed templates/transaction_created.html
var transactionCreatedHTML string

// Parsed once at startup so a missing or broken template panics the binary
// immediately instead of on the first email the worker tries to send.
var transactionCreatedTmpl = template.Must(
	template.New("transaction_created").Parse(transactionCreatedHTML),
)

type TransactionEmailData struct {
	To            string `json:"to"`
	RecipientName string `json:"recipient_name"`
	ActorName     string `json:"actor_name"`
	FinanceName   string `json:"finance_name"`
	Description   string `json:"description"`
	Amount        float64
	IsIncome      bool
	OccurredAt    time.Time
}

// transactionView is what the template actually sees: the amount is
// pre-formatted to two decimals and the date to a fixed layout so the template
// stays free of formatting logic.
type transactionView struct {
	RecipientName string
	ActorName     string
	FinanceName   string
	Description   string
	Amount        string
	OccurredAt    string
	IsIncome      bool
}

func RenderTransactionCreated(data TransactionEmailData) (Message, error) {
	// Names and descriptions are free text typed by users and must never carry
	// markup into a member's inbox. Strip tags here; html/template then escapes
	// whatever plain text is left (e.g. "&" -> "&amp;").
	view := transactionView{
		RecipientName: stripTags(data.RecipientName),
		ActorName:     stripTags(data.ActorName),
		FinanceName:   stripTags(data.FinanceName),
		Description:   stripTags(data.Description),
		Amount:        strconv.FormatFloat(data.Amount, 'f', 2, 64),
		OccurredAt:    data.OccurredAt.Format("2 Jan 2006"),
		IsIncome:      data.IsIncome,
	}

	var body bytes.Buffer
	if err := transactionCreatedTmpl.Execute(&body, view); err != nil {
		return Message{}, err
	}

	kind := "expense"
	if data.IsIncome {
		kind = "income"
	}

	// The subject carries user-typed text (finance name, description), so strip
	// CR/LF: a newline here would let that text inject extra mail headers.
	subject := sanitizeHeader(fmt.Sprintf("New %s in %s: %s", kind, data.FinanceName, data.Description))

	return Message{
		To:       []string{data.To},
		Subject:  subject,
		HTMLBody: body.String(),
	}, nil
}

func sanitizeHeader(s string) string {
	return strings.NewReplacer("\r", "", "\n", "").Replace(strings.TrimSpace(s))
}

var htmlTag = regexp.MustCompile(`<[^>]*>`)

func stripTags(s string) string {
	return strings.TrimSpace(htmlTag.ReplaceAllString(s, ""))
}
