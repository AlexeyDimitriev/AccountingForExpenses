package application

import (
	"time"

	"github.com/AlexeyDimitriev/AccountingForExpenses/internal/domain"
)

// ExpenseFilter задаёт включительные границы календарных дат; нулевые поля отключают фильтр.
type ExpenseFilter struct {
	CategoryID int64
	DateFrom   time.Time
	DateTo     time.Time
}

// Validate проверяет категорию и порядок календарных границ периода.
func (filter ExpenseFilter) Validate() error {
	if filter.CategoryID < 0 {
		return domain.ErrInvalidCategoryID
	}
	if !filter.DateFrom.IsZero() && !filter.DateTo.IsZero() && calendarDate(filter.DateFrom).After(calendarDate(filter.DateTo)) {
		return ErrInvalidPeriod
	}
	return nil
}

// calendarDate сохраняет календарный день без времени суток и часового пояса.
func calendarDate(value time.Time) time.Time {
	if value.IsZero() {
		return time.Time{}
	}
	year, month, day := value.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}
