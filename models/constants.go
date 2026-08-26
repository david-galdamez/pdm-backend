package models

// SavingsCategoryName is the reserved name given to the category and
// subcategory that every finance gets on creation to track savings.
// Transactions filed against that subcategory also roll up into
// MonthlySaving, and users are not allowed to create another one by
// the same name.
const SavingsCategoryName = "Savings"

// Seeded lookup identifiers. These match the rows inserted by
// cmd/migrations and are referenced directly when creating records.
const (
	FinanceTypePersonal uint = 1
	FinanceTypeShared   uint = 2

	RoleAdmin        uint = 1
	RoleCollaborator uint = 2

	BudgetTypeVariable    uint = 1
	BudgetTypeFixed       uint = 2
	BudgetTypeProvisional uint = 3

	EntryTypeIncome  uint = 1
	EntryTypeExpense uint = 2
)
