package controllers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"pdm-backend/events"
	"pdm-backend/repositories"
	"pdm-backend/services"

	"github.com/gin-gonic/gin"
)

// recordingEmailPublisher stands in for the broker so a case can assert on
// whether the handler published, without any infrastructure.
type recordingEmailPublisher struct {
	err      error
	received []events.TransactionEmailEvent
}

func (r *recordingEmailPublisher) PublishTransactionEmail(_ context.Context, event events.TransactionEmailEvent) error {
	r.received = append(r.received, event)
	return r.err
}

// recordingBroker is the existing websocket publisher, stubbed the same way.
type recordingBroker struct {
	calls int
}

func (r *recordingBroker) Publish(uint, bool) error {
	r.calls++
	return nil
}

// postTransaction runs CreateTransaction against a dry-run database and reports
// the status alongside the stubs, so the cases can see what was published.
func postTransaction(t *testing.T, body string) (int, *recordingBroker, *recordingEmailPublisher) {
	t.Helper()

	gin.SetMode(gin.TestMode)

	broker := &recordingBroker{}
	publisher := &recordingEmailPublisher{}

	handler := NewTransactionHandler(repositories.NewTransactionRepository(failingDB(t)), broker, publisher)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/transactions", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	// AuthMiddleware and FinanceAccess would have put these here. The finance
	// differs from the token's own so the handler treats it as shared.
	c.Set("claims", &services.JWTClaims{UserID: 42, UserName: "David", FinanceID: 1, SavingsID: 9})
	c.Set(services.FinanceIdKey, uint(7))

	handler.CreateTransaction(c)

	return recorder.Code, broker, publisher
}

func validTransactionBody() string {
	occurred := time.Now().Format(time.RFC3339)

	return `{"entry_type_id":2,"movement_id":3,"amount":150.25,"description":"Groceries","occurred_at":"` + occurred + `"}`
}

// The publisher is a constructor argument like the repositories, so the
// transport can be swapped in main.go instead of at the call site.
func TestNewTransactionHandlerTakesAnEmailPublisher(t *testing.T) {
	handler := NewTransactionHandler(
		repositories.NewTransactionRepository(failingDB(t)),
		events.NewMemoryBroker(),
		events.NoopEmailPublisher{},
	)

	if handler == nil {
		t.Fatal("NewTransactionHandler returned nil")
	}
}

// Nothing is emailed about a transaction that was never written.
func TestCreateTransactionDoesNotPublishOnARejectedRequest(t *testing.T) {
	status, broker, publisher := postTransaction(t, `{"amount":-5}`)

	if status != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", status, http.StatusBadRequest)
	}

	if len(publisher.received) != 0 {
		t.Errorf("published %d email events for a rejected request", len(publisher.received))
	}

	if broker.calls != 0 {
		t.Errorf("published %d websocket events for a rejected request", broker.calls)
	}
}

// The write is what the email describes: a failed insert must not produce one.
func TestCreateTransactionDoesNotPublishWhenTheWriteFails(t *testing.T) {
	status, _, publisher := postTransaction(t, validTransactionBody())

	if status == http.StatusCreated {
		t.Fatalf("status = %d, want a failure: the database is unusable, so nothing can have been written", status)
	}

	if len(publisher.received) != 0 {
		t.Errorf("published %d email events although the write failed", len(publisher.received))
	}
}
