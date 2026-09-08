package email

import (
	"strings"
	"testing"
	"time"
)

func sampleData() TransactionEmailData {
	return TransactionEmailData{
		To:            "member@example.test",
		RecipientName: "Ana",
		ActorName:     "David",
		FinanceName:   "Household",
		Description:   "Groceries",
		Amount:        150.25,
		IsIncome:      false,
		OccurredAt:    time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC),
	}
}

func TestRenderTransactionCreatedIncludesTheDetails(t *testing.T) {
	message, err := RenderTransactionCreated(sampleData())
	if err != nil {
		t.Fatalf("rendering: %v", err)
	}

	if len(message.To) != 1 || message.To[0] != "member@example.test" {
		t.Errorf("To = %v, want [member@example.test]", message.To)
	}

	if strings.TrimSpace(message.Subject) == "" {
		t.Error("Subject is empty")
	}

	// A subject with a newline in it lets a crafted description inject extra
	// headers into the message.
	if strings.ContainsAny(message.Subject, "\r\n") {
		t.Errorf("Subject contains a line break: %q", message.Subject)
	}

	for _, want := range []string{"Ana", "David", "Household", "Groceries", "150.25"} {
		if !strings.Contains(message.HTMLBody, want) {
			t.Errorf("body is missing %q:\n%s", want, message.HTMLBody)
		}
	}
}

// The description is free text typed by a user and lands in every member's
// inbox, so the template has to be html/template, not text/template.
func TestRenderTransactionCreatedEscapesUserText(t *testing.T) {
	data := sampleData()
	data.Description = `<script>alert(1)</script>`
	data.ActorName = `<img src=x onerror=alert(1)>`
	data.FinanceName = `Tom & Jerry's`

	message, err := RenderTransactionCreated(data)
	if err != nil {
		t.Fatalf("rendering: %v", err)
	}

	for _, unwanted := range []string{"<script>", "onerror=", "<img "} {
		if strings.Contains(message.HTMLBody, unwanted) {
			t.Errorf("body contains unescaped %q:\n%s", unwanted, message.HTMLBody)
		}
	}

	if !strings.Contains(message.HTMLBody, "&amp;") {
		t.Errorf("body did not escape the ampersand in the finance name:\n%s", message.HTMLBody)
	}
}

// The header injection is worth its own case: a name is enough to add a Bcc if
// the subject is built by concatenation.
func TestRenderTransactionCreatedRejectsHeaderInjection(t *testing.T) {
	data := sampleData()
	data.ActorName = "David\r\nBcc: attacker@example.test"

	message, err := RenderTransactionCreated(data)
	if err != nil {
		return
	}

	if strings.ContainsAny(message.Subject, "\r\n") {
		t.Errorf("Subject carried the injected break: %q", message.Subject)
	}
}

func TestRenderTransactionCreatedDistinguishesIncomeFromExpense(t *testing.T) {
	expense, err := RenderTransactionCreated(sampleData())
	if err != nil {
		t.Fatalf("rendering the expense: %v", err)
	}

	income := sampleData()
	income.IsIncome = true

	rendered, err := RenderTransactionCreated(income)
	if err != nil {
		t.Fatalf("rendering the income: %v", err)
	}

	if rendered.Subject == expense.Subject && rendered.HTMLBody == expense.HTMLBody {
		t.Error("income and expense render identically; the reader cannot tell them apart")
	}
}

// The template is embedded so the worker binary stays self-contained: the
// Dockerfile copies no template directory and there is no runtime path to get
// wrong. A missing file has to fail at parse time, not on the first email.
func TestTemplateIsEmbedded(t *testing.T) {
	if _, err := RenderTransactionCreated(sampleData()); err != nil {
		t.Fatalf("rendering from the embedded template: %v", err)
	}
}
