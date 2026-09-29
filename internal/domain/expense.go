package domain

import "time"

type Expense struct {
	ID            int64
	CategoryID    int64
	AmountKopecks int64
	Date          time.Time
	Comment       string
}

// Validate проверяет категорию, сумму в копейках и наличие даты расхода.
func (expense Expense) Validate() error {
	if expense.CategoryID <= 0 {
		return ErrInvalidCategoryID
	}

	if expense.AmountKopecks <= 0 {
		return ErrInvalidAmount
	}

	if expense.Date.IsZero() {
		return ErrInvalidDate
	}

	return nil
}
