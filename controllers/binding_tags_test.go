package controllers

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// requestStructs is every struct in this package that gets bound from a
// request body. Anything added here is covered by the tag hygiene check below.
func requestStructs() []any {
	return []any{
		RegisterRequest{},
		LoginRequest{},
		UpdateProfileRequest{},
		UpdatePasswordRequest{},
		TransactionRequest{},
		IncomeSourceRequest{},
		SavingGoalRequest{},
		CategoryRequest{},
		SubcategoryRequest{},
		CreateSharedFinanceRequest{},
		JoinRequest{},
	}
}

// TestBindingTagsHaveNoSpaces is the regression test for `binding:"required,
// gt=0"`. go-playground/validator splits the tag on commas without trimming,
// so a space makes it look up a validator named " gt", which does not exist —
// and an unregistered validator is a panic at request time, not a validation
// error. gofmt and go vet both consider the tag fine.
func TestBindingTagsHaveNoSpaces(t *testing.T) {
	for _, request := range requestStructs() {
		structType := reflect.TypeOf(request)

		for i := range structType.NumField() {
			field := structType.Field(i)

			tag, ok := field.Tag.Lookup("binding")
			if !ok {
				continue
			}

			for _, rule := range strings.Split(tag, ",") {
				if rule != strings.TrimSpace(rule) {
					t.Errorf("%s.%s has binding:%q: the rule %q is padded with whitespace, which panics the request",
						structType.Name(), field.Name, tag, rule)
				}
			}
		}
	}
}

// TestRequestStructsBindWithoutPanicking is the behavioural half: every
// request struct is bound once against a body that satisfies nothing, which is
// enough to make the validator resolve each rule it carries.
func TestRequestStructsBindWithoutPanicking(t *testing.T) {
	gin.SetMode(gin.TestMode)

	for _, request := range requestStructs() {
		structType := reflect.TypeOf(request)

		t.Run(structType.Name(), func(t *testing.T) {
			defer func() {
				if recovered := recover(); recovered != nil {
					t.Fatalf("binding %s panicked: %v", structType.Name(), recovered)
				}
			}()

			target := reflect.New(structType).Interface()

			context, _ := gin.CreateTestContext(httptest.NewRecorder())
			context.Request = httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{}`))
			context.Request.Header.Set("Content-Type", "application/json")

			// The error is expected; the point is that resolving the rules did
			// not blow up on the way to producing it.
			_ = context.ShouldBindJSON(target)
		})
	}
}

// TestIncomeSourceAmountRejectsNonPositiveAmounts is the rule the spaced tag
// was meant to add, verified end to end through the binder.
func TestIncomeSourceAmountRejectsNonPositiveAmounts(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cases := []struct {
		name    string
		body    string
		wantErr bool
	}{
		{"positive amount", `{"name":"Salary","description":"monthly","amount":1500.50}`, false},
		{"zero amount", `{"name":"Salary","description":"monthly","amount":0}`, true},
		{"negative amount", `{"name":"Salary","description":"monthly","amount":-10}`, true},
		{"missing name", `{"description":"monthly","amount":10}`, true},
		{"missing description", `{"name":"Salary","amount":10}`, true},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			var request IncomeSourceRequest

			context, _ := gin.CreateTestContext(httptest.NewRecorder())
			context.Request = httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(testCase.body))
			context.Request.Header.Set("Content-Type", "application/json")

			err := context.ShouldBindJSON(&request)

			if testCase.wantErr && err == nil {
				t.Errorf("binding %s was accepted; it should have been rejected", testCase.body)
			}

			if !testCase.wantErr && err != nil {
				t.Errorf("binding %s was rejected: %v", testCase.body, err)
			}
		})
	}
}

// TestSubcategoryBudgetAcceptsZero pins the other half of that pairing:
// "required" on a float rejects the zero value, so required + gte=0 made a
// zero budget impossible to express.
func TestSubcategoryBudgetAcceptsZero(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cases := []struct {
		name    string
		body    string
		wantErr bool
	}{
		{"zero budget", `{"category_id":1,"name":"Cinema","budget_type_id":1,"budget":0}`, false},
		{"positive budget", `{"category_id":1,"name":"Cinema","budget_type_id":1,"budget":75}`, false},
		{"negative budget", `{"category_id":1,"name":"Cinema","budget_type_id":1,"budget":-1}`, true},
		{"missing category", `{"name":"Cinema","budget_type_id":1,"budget":10}`, true},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			var request SubcategoryRequest

			context, _ := gin.CreateTestContext(httptest.NewRecorder())
			context.Request = httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(testCase.body))
			context.Request.Header.Set("Content-Type", "application/json")

			err := context.ShouldBindJSON(&request)

			if testCase.wantErr && err == nil {
				t.Errorf("binding %s was accepted; it should have been rejected", testCase.body)
			}

			if !testCase.wantErr && err != nil {
				t.Errorf("binding %s was rejected: %v", testCase.body, err)
			}
		})
	}
}
