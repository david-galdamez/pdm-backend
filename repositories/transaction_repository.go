package repositories

import (
	"errors"
	"pdm-backend/models"
	"sync"
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
			WHEN transactions.entry_type_id = 1 THEN income_sources.name
			WHEN transactions.entry_type_id = 2 THEN expense_categories.name
			ELSE ''
		END AS category_name,
		transactions.amount AS amount,
		transactions.entry_type_id AS entry_type_id,
		entry_types.name AS entry_type_name,
		transactions.occurred_at AS occurred_at,
		users.name AS user_name`).
		Joins("LEFT JOIN expense_categories ON expense_categories.id = transactions.expense_category_id").
		Joins("LEFT JOIN income_sources ON income_sources.id = transactions.income_source_id").
		Joins("LEFT JOIN entry_types ON entry_types.id = transactions.entry_type_id").
		Joins("LEFT JOIN users ON users.id = transactions.user_id").
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
	errCh := make(chan error, 2)

	incomeSourceRepo := NewIncomeSourceRepository(r.DB)
	subcategoryRepo := NewSubcategoryRepository(r.DB)
	var wg sync.WaitGroup

	err := r.DB.Model(models.EntryType{}).
		Select("entry_types.id AS entry_type_id, entry_types.name AS entry_type_name").
		Scan(&transactionOptions).Error
	if err != nil {
		return nil, err
	}

	for i := range transactionOptions {
		transactionOptions[i].Options = make([]any, 0)
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		subcategoryOptions, err := subcategoryRepo.GetSubcategories(financeId)
		if err != nil {
			errCh <- err
			return
		}
		for i := range transactionOptions {
			if transactionOptions[i].EntryTypeID == models.EntryTypeExpense {
				for _, option := range subcategoryOptions {
					transactionOptions[i].Options = append(transactionOptions[i].Options, option)
				}
				break
			}
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		incomeOptions, err := incomeSourceRepo.GetIncomeSources(financeId)
		if err != nil {
			errCh <- err
			return
		}
		for i := range transactionOptions {
			if transactionOptions[i].EntryTypeID == models.EntryTypeIncome {
				for _, option := range incomeOptions {
					transactionOptions[i].Options = append(transactionOptions[i].Options, option)
				}
				break
			}
		}
	}()

	go func() {
		wg.Wait()
		close(errCh)
	}()

	for err := range errCh {
		if err != nil {
			return nil, err
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

	tx := r.DB.Model(models.Transaction{}).Where("transactions.id = ? AND transactions.finance_id = ?", transactionId, financeId).
		Select(`
		transactions.entry_type_id AS entry_type_id,
		entry_types.name AS entry_type_name,
		CASE
			WHEN transactions.entry_type_id = 1 THEN income_sources.name
			WHEN transactions.entry_type_id = 2 THEN expense_subcategories.name
			ELSE ''
		END AS movement,
		CASE
			WHEN transactions.entry_type_id = 2 THEN expense_categories.name
			ELSE ''
		END AS category,
		CASE
			WHEN transactions.entry_type_id = 2 THEN budget_types.name
			ELSE ''
		END AS budget_type,
		CASE
			WHEN expense_subcategories.name = ? THEN monthly_goals.target_amount
			WHEN transactions.entry_type_id = 1 THEN income_sources.amount
			WHEN transactions.entry_type_id = 2 THEN expense_subcategories.monthly_budget
			ELSE 0
		END AS budget,
		transactions.amount AS amount,
		transactions.description AS description,
		users.name AS user_name
	`, models.SavingsCategoryName).
		Joins("LEFT JOIN entry_types ON entry_types.id = transactions.entry_type_id").
		Joins("LEFT JOIN income_sources ON income_sources.id = transactions.income_source_id").
		Joins("LEFT JOIN expense_subcategories ON expense_subcategories.id = transactions.expense_subcategory_id").
		Joins("LEFT JOIN expense_categories ON expense_categories.id = transactions.expense_category_id").
		Joins("LEFT JOIN budget_types ON budget_types.id = transactions.budget_type_id").
		Joins("LEFT JOIN monthly_goals ON monthly_goals.finance_id = transactions.finance_id AND monthly_goals.month = EXTRACT(MONTH FROM transactions.occurred_at) AND monthly_goals.year = EXTRACT(YEAR FROM transactions.occurred_at)").
		Joins("LEFT JOIN users ON users.id = transactions.user_id").
		Scan(&transaction)

	if tx.RowsAffected == 0 {
		return nil, gorm.ErrRecordNotFound
	}

	if err := tx.Error; err != nil {
		return nil, err
	}

	return &transaction, nil
}

type SubcategoryIdentifiers struct {
	CategoryID   uint
	BudgetTypeID uint
}

func (r *TransactionRepository) GetIds(subcategoryId uint) (*SubcategoryIdentifiers, error) {

	var identifiers SubcategoryIdentifiers

	err := r.DB.Model(models.ExpenseSubcategory{}).Where("expense_subcategories.id = ?", subcategoryId).
		Select("expense_subcategories.expense_category_id AS category_id, expense_subcategories.budget_type_id AS budget_type_id").
		Scan(&identifiers).Error
	if err != nil {
		return nil, err
	}

	return &identifiers, nil
}

func (r *TransactionRepository) CreateTransaction(transaction *models.Transaction) error {
	return r.DB.Create(&transaction).Error
}

func (r *TransactionRepository) CreateOrUpdateSaving(financeId uint, amount float64, date time.Time) error {

	year := date.Year()
	month := int(date.Month())

	var saving models.MonthlySaving
	err := r.DB.Model(models.MonthlySaving{}).
		Where("finance_id = ? AND year = ? AND month = ?", financeId, year, month).
		First(&saving).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			newSaving := models.MonthlySaving{
				FinanceID: financeId,
				Amount:    amount,
				Month:     month,
				Year:      year,
			}
			return r.DB.Create(&newSaving).Error
		}
		return err
	}

	saving.Amount += amount

	return r.DB.Save(&saving).Error
}

func (r *TransactionRepository) GetSavingSubcategory(financeId uint) (uint, error) {
	var subcategoryId uint

	err := r.DB.Model(models.ExpenseSubcategory{}).
		Where("finance_id = ? AND name = ?", financeId, models.SavingsCategoryName).
		Select("id").Scan(&subcategoryId).Error
	if err != nil {
		return 0, err
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
