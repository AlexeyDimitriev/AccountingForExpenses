package domain

import (
	"errors"
	"math"
	"testing"
	"time"
)

// TestExpenseValidate проверяет обязательные поля и границы допустимой суммы расхода.
func TestExpenseValidate(t *testing.T) {
	date := time.Date(2026, time.September, 29, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name    string
		expense Expense
		wantErr error
	}{
		{
			name:    "new expense with optional comment omitted",
			expense: Expense{CategoryID: 1, AmountKopecks: 12345, Date: date},
		},
		{
			name:    "saved expense with comment",
			expense: Expense{ID: 1, CategoryID: 2, AmountKopecks: 5000, Date: date, Comment: "Обед"},
		},
		{
			name:    "one kopeck",
			expense: Expense{CategoryID: 1, AmountKopecks: 1, Date: date},
		},
		{
			name:    "maximum integer amount",
			expense: Expense{CategoryID: 1, AmountKopecks: math.MaxInt64, Date: date},
		},
		{
			name:    "missing category",
			expense: Expense{AmountKopecks: 100, Date: date},
			wantErr: ErrInvalidCategoryID,
		},
		{
			name:    "negative category",
			expense: Expense{CategoryID: -1, AmountKopecks: 100, Date: date},
			wantErr: ErrInvalidCategoryID,
		},
		{
			name:    "zero amount",
			expense: Expense{CategoryID: 1, Date: date},
			wantErr: ErrInvalidAmount,
		},
		{
			name:    "negative amount",
			expense: Expense{CategoryID: 1, AmountKopecks: -1, Date: date},
			wantErr: ErrInvalidAmount,
		},
		{
			name:    "minimum integer amount",
			expense: Expense{CategoryID: 1, AmountKopecks: math.MinInt64, Date: date},
			wantErr: ErrInvalidAmount,
		},
		{
			name:    "missing date",
			expense: Expense{CategoryID: 1, AmountKopecks: 100},
			wantErr: ErrInvalidDate,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.expense.Validate(); !errors.Is(err, test.wantErr) {
				t.Errorf("Validate() = %v, want %v", err, test.wantErr)
			}
		})
	}
}
