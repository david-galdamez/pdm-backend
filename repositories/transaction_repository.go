package repositories

import (
	"pdm-backend/models"
	"time"

	"gorm.io/gorm"
)

type TransactionRepository struct {
	DB *gorm.DB
}

func NewTransactionRepository(db *gorm.DB) *TransactionRepository {
	return &TransactionRepository{DB: db}
}

type TransactionListItem struct {
	TransactionID uint    `json:"transaction_id"`
	CategoryName  string  `json:"category_name"`
	Amount        float64 `json:"amount"`
	EntryTypeID   uint    `json:"entry_type_id"`
	EntryTypeName string  `json:"entry_type_name"`
	OccurredAt    string  `json:"occurred_at"`
	UserName      string  `json:"user_name"`
}

func (r *TransactionRepository) GetTransactions(monthStart, monthEnd time.Time, financeId uint) ([]TransactionListItem, error) {

	transactions := []TransactionListItem{}

	err := r.DB.Model(models.Transaction{}).
		Where("transactions.finance_id = ? AND transactions.occurred_at >= ? AND transactions.occurred_at < ?", financeId, monthStart, monthEnd).
		Select(`
		transactions.id AS transaction_id,
		CASE
			WHEN transactions.entry_type_id = ? THEN income_sources.name
			WHEN transactions.entry_type_id = ? THEN expense_categories.name
			ELSE ''
		END AS category_name,
		transactions.amount AS amount,
		transactions.entry_type_id AS entry_type_id,
		entry_types.name AS entry_type_name,
		transactions.occurred_at AS occurred_at,
		users.name AS user_name`, models.EntryTypeIncome, models.EntryTypeExpense).
		Joins("LEFT JOIN expense_categories ON expense_categories.id = transactions.expense_category_id AND expense_categories.deleted_at IS NULL").
		Joins("LEFT JOIN income_sources ON income_sources.id = transactions.income_source_id AND income_sources.deleted_at IS NULL").
		Joins("LEFT JOIN entry_types ON entry_types.id = transactions.entry_type_id AND entry_types.deleted_at IS NULL").
		Joins("LEFT JOIN users ON users.id = transactions.user_id AND users.deleted_at IS NULL").
		Order("transactions.occurred_at DESC").
		Scan(&transactions).Error

	if err != nil {
		return nil, err
	}

	return transactions, nil
}

type TransactionOptions struct {
	EntryTypeID   uint   `json:"entry_type_id"`
	EntryTypeName string `json:"entry_type_name"`
	Options       []any  `json:"options"`
}

func (r *TransactionRepository) GetOptions(financeId uint) ([]TransactionOptions, error) {

	transactionOptions := []TransactionOptions{}

	err := r.DB.Model(models.EntryType{}).
		Select("entry_types.id AS entry_type_id, entry_types.name AS entry_type_name").
		Scan(&transactionOptions).Error
	if err != nil {
		return nil, err
	}

	subcategoryOptions, err := NewSubcategoryRepository(r.DB).GetSubcategories(financeId)
	if err != nil {
		return nil, err
	}

	incomeOptions, err := NewIncomeSourceRepository(r.DB).GetIncomeSources(financeId)
	if err != nil {
		return nil, err
	}

	for i := range transactionOptions {
		transactionOptions[i].Options = make([]any, 0)

		switch transactionOptions[i].EntryTypeID {
		case models.EntryTypeExpense:
			for _, option := range subcategoryOptions {
				transactionOptions[i].Options = append(transactionOptions[i].Options, option)
			}
		case models.EntryTypeIncome:
			for _, option := range incomeOptions {
				transactionOptions[i].Options = append(transactionOptions[i].Options, option)
			}
		}
	}

	return transactionOptions, nil
}

type TransactionDetails struct {
	EntryTypeID   uint    `json:"entry_type_id" gorm:"column:entry_type_id"`
	EntryTypeName string  `json:"entry_type_name" gorm:"column:entry_type_name"`
	Movement      string  `json:"movement" gorm:"column:movement"`
	Category      string  `json:"category" gorm:"column:category"`
	BudgetType    string  `json:"budget_type" gorm:"column:budget_type"`
	Budget        float64 `json:"budget" gorm:"column:budget"`
	Amount        float64 `json:"amount" gorm:"column:amount"`
	Description   string  `json:"description" gorm:"column:description"`
	UserName      string  `json:"user_name" gorm:"column:user_name"`
}

func (r *TransactionRepository) GetTransactionById(transactionId *uint, financeId uint) (*TransactionDetails, error) {

	var transaction TransactionDetails

	income := models.EntryTypeIncome
	expense := models.EntryTypeExpense

	tx := r.DB.Model(models.Transaction{}).Where("transactions.id = ? AND transactions.finance_id = ?", transactionId, financeId).
		Select(`
		transactions.entry_type_id AS entry_type_id,
		entry_types.name AS entry_type_name,
		CASE
			WHEN transactions.entry_type_id = ? THEN income_sources.name
			WHEN transactions.entry_type_id = ? THEN expense_subcategories.name
			ELSE ''
		END AS movement,
		CASE
			WHEN transactions.entry_type_id = ? THEN expense_categories.name
			ELSE ''
		END AS category,
		CASE
			WHEN transactions.entry_type_id = ? THEN budget_types.name
			ELSE ''
		END AS budget_type,
		CASE
			WHEN expense_subcategories.name = ? THEN monthly_goals.target_amount
			WHEN transactions.entry_type_id = ? THEN income_sources.amount
			WHEN transactions.entry_type_id = ? THEN expense_subcategories.monthly_budget
			ELSE 0
		END AS budget,
		transactions.amount AS amount,
		transactions.description AS description,
		users.name AS user_name
	`, income, expense, expense, expense, models.SavingsCategoryName, income, expense).
		Joins("LEFT JOIN entry_types ON entry_types.id = transactions.entry_type_id AND entry_types.deleted_at IS NULL").
		Joins("LEFT JOIN income_sources ON income_sources.id = transactions.income_source_id AND income_sources.deleted_at IS NULL").
		Joins("LEFT JOIN expense_subcategories ON expense_subcategories.id = transactions.expense_subcategory_id AND expense_subcategories.deleted_at IS NULL").
		Joins("LEFT JOIN expense_categories ON expense_categories.id = transactions.expense_category_id AND expense_categories.deleted_at IS NULL").
		Joins("LEFT JOIN budget_types ON budget_types.id = transactions.budget_type_id AND budget_types.deleted_at IS NULL").
		Joins("LEFT JOIN monthly_goals ON monthly_goals.finance_id = transactions.finance_id AND monthly_goals.month = EXTRACT(MONTH FROM transactions.occurred_at) AND monthly_goals.year = EXTRACT(YEAR FROM transactions.occurred_at) AND monthly_goals.deleted_at IS NULL").
		Joins("LEFT JOIN users ON users.id = transactions.user_id AND users.deleted_at IS NULL").
		Scan(&transaction)

	// Error before RowsAffected: a failed query also reports zero rows, and
	// reporting that as "not found" turns a 500 into a silent 404.
	if err := tx.Error; err != nil {
		return nil, err
	}

	if tx.RowsAffected == 0 {
		return nil, gorm.ErrRecordNotFound
	}

	return &transaction, nil
}

type SubcategoryIdentifiers struct {
	CategoryID   uint
	BudgetTypeID uint
}

// GetIds resolves the category and budget type a subcategory belongs to. It is
// scoped to the finance so a caller cannot file an expense against a
// subcategory belonging to somebody else's finance, and returns
// gorm.ErrRecordNotFound rather than zero identifiers when there is no match.
func (r *TransactionRepository) GetIds(subcategoryId, financeId uint) (*SubcategoryIdentifiers, error) {

	var identifiers SubcategoryIdentifiers

	tx := r.DB.Model(models.ExpenseSubcategory{}).
		Where("expense_subcategories.id = ? AND expense_subcategories.finance_id = ?", subcategoryId, financeId).
		Select("expense_subcategories.expense_category_id AS category_id, expense_subcategories.budget_type_id AS budget_type_id").
		Scan(&identifiers)

	if err := tx.Error; err != nil {
		return nil, err
	}

	if tx.RowsAffected == 0 {
		return nil, gorm.ErrRecordNotFound
	}

	return &identifiers, nil
}

// IncomeSourceBelongsToFinance is the income-side counterpart of GetIds: the
// movement id on the request is client-supplied, so it has to be confirmed
// against the finance before it is written onto a transaction.
func (r *TransactionRepository) IncomeSourceBelongsToFinance(incomeSourceId, financeId uint) (bool, error) {

	var count int64

	err := r.DB.Model(models.IncomeSource{}).
		Where("id = ? AND finance_id = ?", incomeSourceId, financeId).
		Count(&count).Error
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

func (r *TransactionRepository) CreateTransaction(transaction *models.Transaction) error {
	return r.DB.Create(&transaction).Error
}

// CreateTransactionWithSaving writes the transaction and, when it is filed
// against the savings subcategory, rolls it into the month's MonthlySaving —
// both in one database transaction, so a failure partway cannot leave the
// ledger and the savings total permanently disagreeing.
func (r *TransactionRepository) CreateTransactionWithSaving(transaction *models.Transaction, rollUpIntoSavings bool) error {

	return r.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(transaction).Error; err != nil {
			return err
		}

		if !rollUpIntoSavings {
			return nil
		}

		return addToMonthlySaving(tx, transaction.FinanceID, transaction.Amount, transaction.OccurredAt)
	})
}

func (r *TransactionRepository) CreateOrUpdateSaving(financeId uint, amount float64, date time.Time) error {
	return addToMonthlySaving(r.DB, financeId, amount, date)
}

// addToMonthlySaving accumulates amount into the finance's saving row for the
// month. The increment happens in the UPDATE itself: reading the row into Go,
// adding, and writing it back loses one of two concurrent contributions, which
// on a shared finance is an ordinary Tuesday rather than a rare race.
func addToMonthlySaving(db *gorm.DB, financeId uint, amount float64, date time.Time) error {

	year := date.Year()
	month := int(date.Month())

	increment := func() *gorm.DB {
		return db.Model(&models.MonthlySaving{}).
			Where("finance_id = ? AND year = ? AND month = ?", financeId, year, month).
			Update("amount", gorm.Expr("monthly_savings.amount + ?", amount))
	}

	result := increment()
	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected > 0 {
		return nil
	}

	newSaving := models.MonthlySaving{
		FinanceID: financeId,
		Amount:    amount,
		Month:     month,
		Year:      year,
	}

	err := db.Create(&newSaving).Error
	if err == nil {
		return nil
	}

	// Another request inserted the month's row between the update and the
	// insert; the unique index caught it, so fold this amount into that row.
	if !IsUniqueViolation(err) {
		return err
	}

	return increment().Error
}

// GetSavingSubcategory returns the finance's reserved savings subcategory.
// Every finance gets one at creation, so a miss means the data is broken and
// is reported rather than silently returning id 0.
func (r *TransactionRepository) GetSavingSubcategory(financeId uint) (uint, error) {
	var subcategoryId uint

	tx := r.DB.Model(models.ExpenseSubcategory{}).
		Where("finance_id = ? AND name = ?", financeId, models.SavingsCategoryName).
		Select("id").Scan(&subcategoryId)

	if err := tx.Error; err != nil {
		return 0, err
	}

	if tx.RowsAffected == 0 {
		return 0, gorm.ErrRecordNotFound
	}

	return subcategoryId, nil
}

type PayloadEvent struct {
	Event string `json:"event"`
}

type BroadCastMessage struct {
	FinanceID uint           `json:"finance_id"`
	EventInfo []PayloadEvent `json:"event_info"`
}

func (r *TransactionRepository) BuildWebSocketEvent(financeId uint, transactionSubcategoryId *uint, savingSubcategoryId uint) *BroadCastMessage {
	eventInfo := []PayloadEvent{
		{Event: "finance_summary"},
		{Event: "finance_data"},
		{Event: "transaction_list"},
	}

	if transactionSubcategoryId != nil && *transactionSubcategoryId == savingSubcategoryId {
		eventInfo = append(eventInfo, PayloadEvent{Event: "finance_savings"})
	}

	return &BroadCastMessage{
		FinanceID: financeId,
		EventInfo: eventInfo,
	}
}
