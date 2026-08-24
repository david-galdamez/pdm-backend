package routes

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"pdm-backend/models"
	"pdm-backend/repositories"
	"pdm-backend/services"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// TestMain points the whole package at a disposable local Postgres database,
// runs the same migrations cmd/migrations does, and seeds the lookup tables
// once. It never touches the app's real DATABASE_URL.
func TestMain(m *testing.M) {
	os.Setenv("ENV", "test")
	os.Setenv("JWT_SECRET", "test-only-secret-for-authz-suite-do-not-use")
	os.Setenv("DATABASE_URL", "postgres://postgres:analissa@localhost:5432/finance_app_test?sslmode=disable")

	db := repositories.GetDB()
	// Several of this suite's assertions are deliberately "not found" — quiet
	// gorm's per-query error logging so a passing run isn't full of red text.
	db.Logger = logger.Default.LogMode(logger.Silent)

	if err := db.AutoMigrate(
		&models.FinanceType{},
		&models.BudgetType{},
		&models.EntryType{},
		&models.IncomeSource{},
		&models.SharedFinanceRole{},
		&models.User{},
		&models.Finance{},
		&models.SharedFinance{},
		&models.ExpenseCategory{},
		&models.ExpenseSubcategory{},
		&models.Transaction{},
		&models.MonthlyGoal{},
		&models.MonthlySaving{},
		&models.Invitation{},
	); err != nil {
		panic(err)
	}

	seedLookups(db)
	truncateAppTables(db)

	os.Exit(m.Run())
}

// seedLookups mirrors cmd/migrations' SeedData: same rows, same order, so the
// generated ids line up with models/constants.go on a fresh database. It is
// idempotent, so re-running the suite against a database that already has
// these rows does not duplicate or reorder them.
func seedLookups(db *gorm.DB) {
	insertIfMissing := func(model any, name string) {
		if db.Where("name = ?", name).First(model).Error == gorm.ErrRecordNotFound {
			db.Create(model)
		}
	}

	insertIfMissing(&models.FinanceType{Name: "personal"}, "personal")
	insertIfMissing(&models.FinanceType{Name: "shared"}, "shared")
	insertIfMissing(&models.SharedFinanceRole{Name: "admin"}, "admin")
	insertIfMissing(&models.SharedFinanceRole{Name: "collaborator"}, "collaborator")
	insertIfMissing(&models.BudgetType{Name: "Variable expenses"}, "Variable expenses")
	insertIfMissing(&models.BudgetType{Name: "Fixed expenses"}, "Fixed expenses")
	insertIfMissing(&models.BudgetType{Name: "Provisional expenses"}, "Provisional expenses")
	insertIfMissing(&models.EntryType{Name: "Income"}, "Income")
	insertIfMissing(&models.EntryType{Name: "Expense"}, "Expense")
}

// truncateAppTables clears everything except the lookup tables seedLookups
// owns, so each suite run starts from a clean slate regardless of what a
// previous run left behind.
func truncateAppTables(db *gorm.DB) {
	tables := []string{
		"invitations", "monthly_savings", "monthly_goals", "transactions",
		"shared_finances", "expense_subcategories", "expense_categories",
		"income_sources", "finances", "users",
	}

	for _, table := range tables {
		db.Exec(fmt.Sprintf("TRUNCATE TABLE %s RESTART IDENTITY CASCADE", table))
	}
}

// newTestEngine mounts the same routers main.go does, minus CORS and
// websockets, which this suite does not exercise.
func newTestEngine() *gin.Engine {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	UserRouter(r)
	FinanceRouter(r)
	CategoryRouter(r)
	TransactionRouter(r)
	SubcategoryRouter(r)
	IncomeSourceRouter(r)
	SavingRouter(r)
	InvitationRouter(r)
	SharedFinanceRouter(r)

	return r
}

func doRequest(r *gin.Engine, method, path, token, body string) *httptest.ResponseRecorder {
	var reader *strings.Reader
	if body != "" {
		reader = strings.NewReader(body)
	} else {
		reader = strings.NewReader("")
	}

	req := httptest.NewRequest(method, path, reader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	return w
}

// testUser is a real, fully provisioned account: a row in users, a personal
// finance, and the seeded Savings category/subcategory that comes with it.
type testUser struct {
	id        uint
	financeID uint
	savingsID uint
	token     string
}

func createTestUser(t *testing.T, db *gorm.DB, userRepo *repositories.UserRepository, name string) testUser {
	t.Helper()

	user := models.User{Name: name, Email: name + "@example.com", PasswordHash: "irrelevant-for-this-suite"}

	if err := userRepo.CreateUserAndFinance(&user); err != nil {
		t.Fatalf("creating user %s: %v", name, err)
	}

	identifiers, err := userRepo.GetFinanceAndSavingSubcategoryByUserId(user.ID)
	if err != nil {
		t.Fatalf("resolving finance for %s: %v", name, err)
	}

	token, err := services.GenerateJWT(user.ID, user.Name, user.Email, identifiers.FinanceID, identifiers.SavingsID, 0)
	if err != nil {
		t.Fatalf("issuing token for %s: %v", name, err)
	}

	return testUser{id: user.ID, financeID: identifiers.FinanceID, savingsID: identifiers.SavingsID, token: token}
}

// fixture is one full tenant of realistic data plus a second, unrelated
// tenant and a bystander, wired the way a real deployment would be.
type fixture struct {
	engine *gin.Engine
	db     *gorm.DB

	alice, bob, charlie testUser

	// Resources that live in alice's personal finance.
	categoryID      uint
	subcategoryID   uint
	incomeSourceID  uint
	transactionID   uint
	sharedFinanceID uint
}

func setupFixture(t *testing.T) fixture {
	t.Helper()

	db := repositories.GetDB()
	truncateAppTables(db)

	userRepo := repositories.NewUserRepository(db)
	categoryRepo := repositories.NewCategoryRepository(db)
	subcategoryRepo := repositories.NewSubcategoryRepository(db)
	incomeSourceRepo := repositories.NewIncomeSourceRepository(db)
	transactionRepo := repositories.NewTransactionRepository(db)
	sharedFinanceRepo := repositories.NewSharedFinanceRepository(db)

	alice := createTestUser(t, db, userRepo, "alice")
	bob := createTestUser(t, db, userRepo, "bob")
	charlie := createTestUser(t, db, userRepo, "charlie")

	category := models.ExpenseCategory{FinanceID: alice.financeID, Name: "Groceries", UserID: alice.id}
	if err := categoryRepo.CreateCategory(&category); err != nil {
		t.Fatalf("seeding category: %v", err)
	}

	subcategory := models.ExpenseSubcategory{
		FinanceID: alice.financeID, Name: "Supermarket", ExpenseCategoryID: category.ID,
		BudgetTypeID: models.BudgetTypeVariable, MonthlyBudget: 200, UserID: alice.id,
	}
	if err := subcategoryRepo.CreateSubcategory(&subcategory); err != nil {
		t.Fatalf("seeding subcategory: %v", err)
	}

	incomeSource := models.IncomeSource{FinanceID: alice.financeID, Name: "Salary", Amount: 3000, Description: "job", UserID: alice.id}
	if err := incomeSourceRepo.CreateIncomeSource(&incomeSource); err != nil {
		t.Fatalf("seeding income source: %v", err)
	}

	transaction := models.Transaction{
		FinanceID: alice.financeID, UserID: alice.id, EntryTypeID: models.EntryTypeIncome,
		IncomeSourceID: &incomeSource.ID, OccurredAt: time.Now(), Amount: 100,
	}
	if err := transactionRepo.CreateTransaction(&transaction); err != nil {
		t.Fatalf("seeding transaction: %v", err)
	}

	if err := sharedFinanceRepo.CreateSharedFinance(alice.id, "Alice & co", "shared household"); err != nil {
		t.Fatalf("seeding shared finance: %v", err)
	}

	var sharedFinance models.Finance
	if err := db.Where("user_id = ? AND finance_type_id = ?", alice.id, models.FinanceTypeShared).
		Order("id DESC").First(&sharedFinance).Error; err != nil {
		t.Fatalf("looking up seeded shared finance: %v", err)
	}

	return fixture{
		engine:          newTestEngine(),
		db:              db,
		alice:           alice,
		bob:             bob,
		charlie:         charlie,
		categoryID:      category.ID,
		subcategoryID:   subcategory.ID,
		incomeSourceID:  incomeSource.ID,
		transactionID:   transaction.ID,
		sharedFinanceID: sharedFinance.ID,
	}
}

// TestForeignFinanceIdRejected proves that no group-level endpoint honors a
// finance_id the caller does not belong to, regardless of method or path.
// This is the endpoint-by-endpoint sweep for the ?finance_id= IDOR.
func TestForeignFinanceIdRejected(t *testing.T) {
	f := setupFixture(t)

	endpoints := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/finances/summary?month=1&year=2026"},
		{http.MethodGet, "/finances/breakdown?month=1&year=2026"},
		{http.MethodGet, "/categories"},
		{http.MethodGet, "/categories/options"},
		{http.MethodGet, fmt.Sprintf("/categories/%d/breakdown", f.categoryID)},
		{http.MethodPost, "/categories"},
		{http.MethodGet, "/subcategories"},
		{http.MethodGet, "/subcategories/budget-types"},
		{http.MethodGet, fmt.Sprintf("/subcategories/%d", f.subcategoryID)},
		{http.MethodPost, "/subcategories"},
		{http.MethodPut, fmt.Sprintf("/subcategories/%d", f.subcategoryID)},
		{http.MethodGet, "/transactions?month=1&year=2026"},
		{http.MethodGet, "/transactions/options"},
		{http.MethodGet, fmt.Sprintf("/transactions/%d", f.transactionID)},
		{http.MethodPost, "/transactions"},
		{http.MethodGet, "/income-sources"},
		{http.MethodGet, fmt.Sprintf("/income-sources/%d", f.incomeSourceID)},
		{http.MethodPost, "/income-sources"},
		{http.MethodPut, fmt.Sprintf("/income-sources/%d", f.incomeSourceID)},
		{http.MethodGet, "/savings?year=2026"},
		{http.MethodPost, "/savings/goals"},
	}

	sep := func(path string) string {
		if strings.Contains(path, "?") {
			return "&"
		}
		return "?"
	}

	for _, ep := range endpoints {
		t.Run(ep.method+" "+ep.path, func(t *testing.T) {
			path := fmt.Sprintf("%s%sfinance_id=%d", ep.path, sep(ep.path), f.alice.financeID)

			w := doRequest(f.engine, ep.method, path, f.bob.token, "")

			if w.Code != http.StatusForbidden {
				t.Errorf("bob against alice's finance_id: status = %d, want %d. body: %s", w.Code, http.StatusForbidden, w.Body.String())
			}
		})
	}
}

// TestPathParamIdorRejected proves that a caller cannot reach another
// finance's resource by guessing its primary key, even without passing a
// foreign finance_id — the id-scoped repository getters must reject it on
// their own.
func TestPathParamIdorRejected(t *testing.T) {
	f := setupFixture(t)

	// PUT/PATCH handlers bind the request body before doing the finance-scoped
	// lookup, so the body must satisfy each request struct's own validation —
	// otherwise a 400 would mask whether the IDOR check ran at all.
	validSubcategoryBody := fmt.Sprintf(`{"category_id":%d,"name":"whatever","budget_type_id":%d,"budget":10}`,
		f.categoryID, models.BudgetTypeVariable)
	validIncomeSourceBody := `{"name":"whatever","description":"whatever","amount":10}`
	validCategoryBody := `{"name":"whatever"}`

	endpoints := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodGet, fmt.Sprintf("/transactions/%d", f.transactionID), ""},
		{http.MethodGet, fmt.Sprintf("/subcategories/%d", f.subcategoryID), ""},
		{http.MethodPut, fmt.Sprintf("/subcategories/%d", f.subcategoryID), validSubcategoryBody},
		{http.MethodGet, fmt.Sprintf("/income-sources/%d", f.incomeSourceID), ""},
		{http.MethodPut, fmt.Sprintf("/income-sources/%d", f.incomeSourceID), validIncomeSourceBody},
		{http.MethodPatch, fmt.Sprintf("/categories/%d", f.categoryID), validCategoryBody},
	}

	for _, ep := range endpoints {
		t.Run(ep.method+" "+ep.path, func(t *testing.T) {
			// No finance_id: bob resolves to his own personal finance, and
			// tries to reach alice's resource by id alone.
			w := doRequest(f.engine, ep.method, ep.path, f.bob.token, ep.body)

			if w.Code != http.StatusNotFound {
				t.Errorf("bob against alice's resource by id: status = %d, want %d. body: %s", w.Code, http.StatusNotFound, w.Body.String())
			}
		})
	}
}

// TestSharedFinanceLifecycle walks the invite -> join -> admin-gated actions
// -> removal path end to end, proving both the finance-type restriction on
// invitations and the admin-only gate on removal actually hold.
func TestSharedFinanceLifecycle(t *testing.T) {
	f := setupFixture(t)

	// A stranger cannot even see the shared finance exists.
	w := doRequest(f.engine, http.MethodGet, fmt.Sprintf("/shared-finances/%d", f.sharedFinanceID), f.charlie.token, "")
	if w.Code != http.StatusForbidden {
		t.Fatalf("charlie viewing alice's shared finance: status = %d, want 403. body: %s", w.Code, w.Body.String())
	}

	// A stranger cannot create an invite for it either.
	w = doRequest(f.engine, http.MethodPost, fmt.Sprintf("/invitations/%d", f.sharedFinanceID), f.charlie.token, "")
	if w.Code != http.StatusForbidden {
		t.Fatalf("charlie inviting into alice's shared finance: status = %d, want 403. body: %s", w.Code, w.Body.String())
	}

	// Alice cannot invite into her own PERSONAL finance — this is the fix for
	// the personal-finance takeover path: an invitation must target a shared
	// finance, not just any finance the caller administers.
	w = doRequest(f.engine, http.MethodPost, fmt.Sprintf("/invitations/%d", f.alice.financeID), f.alice.token, "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("alice inviting into her own personal finance: status = %d, want 404. body: %s", w.Code, w.Body.String())
	}

	// Alice, the real admin, can invite into the shared finance.
	w = doRequest(f.engine, http.MethodPost, fmt.Sprintf("/invitations/%d", f.sharedFinanceID), f.alice.token, "")
	if w.Code != http.StatusOK {
		t.Fatalf("alice inviting into her shared finance: status = %d, want 200. body: %s", w.Code, w.Body.String())
	}

	var inviteResp struct {
		InvitationCode string `json:"invitation_code"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &inviteResp); err != nil {
		t.Fatalf("decoding invitation response: %v", err)
	}

	// Bob redeems it and becomes a collaborator.
	w = doRequest(f.engine, http.MethodPost, "/shared-finances/join", f.bob.token, fmt.Sprintf(`{"code":%q}`, inviteResp.InvitationCode))
	if w.Code != http.StatusOK {
		t.Fatalf("bob joining with a valid code: status = %d, want 200. body: %s", w.Code, w.Body.String())
	}

	// Now a member, bob can view the finance...
	w = doRequest(f.engine, http.MethodGet, fmt.Sprintf("/shared-finances/%d", f.sharedFinanceID), f.bob.token, "")
	if w.Code != http.StatusOK {
		t.Fatalf("bob viewing the shared finance after joining: status = %d, want 200. body: %s", w.Code, w.Body.String())
	}

	// ...but being a member is not being the admin: bob cannot mint invites...
	w = doRequest(f.engine, http.MethodPost, fmt.Sprintf("/invitations/%d", f.sharedFinanceID), f.bob.token, "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("bob (member, not admin) inviting: status = %d, want 404. body: %s", w.Code, w.Body.String())
	}

	// ...nor remove another member.
	w = doRequest(f.engine, http.MethodDelete,
		fmt.Sprintf("/shared-finances/members/%d?finance_id=%d", f.alice.id, f.sharedFinanceID), f.bob.token, "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("bob (member, not admin) removing alice: status = %d, want 404. body: %s", w.Code, w.Body.String())
	}

	// Alice, the admin, can remove bob.
	w = doRequest(f.engine, http.MethodDelete,
		fmt.Sprintf("/shared-finances/members/%d?finance_id=%d", f.bob.id, f.sharedFinanceID), f.alice.token, "")
	if w.Code != http.StatusOK {
		t.Fatalf("alice removing bob: status = %d, want 200. body: %s", w.Code, w.Body.String())
	}

	// Removed, bob loses access immediately.
	w = doRequest(f.engine, http.MethodGet, fmt.Sprintf("/shared-finances/%d", f.sharedFinanceID), f.bob.token, "")
	if w.Code != http.StatusForbidden {
		t.Fatalf("bob viewing the shared finance after removal: status = %d, want 403. body: %s", w.Code, w.Body.String())
	}
}

// TestJoinRejectsPersonalFinanceInvitation is defense in depth: even if an
// invitation somehow existed for a personal finance (the endpoint that issues
// them now refuses to create one), redeeming it must still fail.
func TestJoinRejectsPersonalFinanceInvitation(t *testing.T) {
	f := setupFixture(t)

	invitation := models.Invitation{
		FinanceID: f.alice.financeID, // alice's PERSONAL finance
		Code:      "PERSONALINVITE",
		ExpiresAt: time.Now().Add(15 * time.Minute),
	}
	if err := f.db.Create(&invitation).Error; err != nil {
		t.Fatalf("planting a personal-finance invitation directly: %v", err)
	}

	w := doRequest(f.engine, http.MethodPost, "/shared-finances/join", f.charlie.token, `{"code":"PERSONALINVITE"}`)
	if w.Code == http.StatusOK {
		t.Fatalf("charlie joined alice's personal finance via a planted invitation: status = %d, body: %s", w.Code, w.Body.String())
	}

	// And even if that somehow let a membership row through, access is still
	// gated on the finance being reachable — confirm charlie still can't touch
	// alice's personal finance.
	w = doRequest(f.engine, http.MethodGet,
		fmt.Sprintf("/finances/summary?month=1&year=2026&finance_id=%d", f.alice.financeID), f.charlie.token, "")
	if w.Code != http.StatusForbidden {
		t.Fatalf("charlie accessing alice's personal finance after the planted invitation attempt: status = %d, want 403. body: %s", w.Code, w.Body.String())
	}
}

// TestOwnerCanAccessOwnData is the sanity check the rest of this suite leans
// on: if every request started failing for unrelated reasons, the 403s and
// 404s above would be meaningless. This proves the happy path still works.
func TestOwnerCanAccessOwnData(t *testing.T) {
	f := setupFixture(t)

	w := doRequest(f.engine, http.MethodGet, "/categories", f.alice.token, "")
	if w.Code != http.StatusOK {
		t.Fatalf("alice listing her own categories: status = %d, want 200. body: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "Groceries") {
		t.Errorf("alice's category list is missing the seeded category: %s", w.Body.String())
	}

	w = doRequest(f.engine, http.MethodGet, fmt.Sprintf("/transactions/%d", f.transactionID), f.alice.token, "")
	if w.Code != http.StatusOK {
		t.Fatalf("alice fetching her own transaction: status = %d, want 200. body: %s", w.Code, w.Body.String())
	}
}
