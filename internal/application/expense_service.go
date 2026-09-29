package application

import (
	"context"
	"math"

	"github.com/AlexeyDimitriev/AccountingForExpenses/internal/domain"
)

type ExpenseService struct {
	expenses   ExpenseRepository
	categories CategoryRepository
}

type ExpenseList struct {
	Expenses     []domain.Expense
	TotalKopecks int64
}

// NewExpenseService создаёт сервис с репозиториями расходов и категорий.
func NewExpenseService(expenses ExpenseRepository, categories CategoryRepository) *ExpenseService {
	return &ExpenseService{expenses: expenses, categories: categories}
}

// Create проверяет расход и категорию, затем сохраняет запись с новым идентификатором.
func (service *ExpenseService) Create(ctx context.Context, expense domain.Expense) (domain.Expense, error) {
	expense.ID = 0
	expense.Date = calendarDate(expense.Date)
	if err := expense.Validate(); err != nil {
		return domain.Expense{}, err
	}
	if _, err := service.categories.Get(ctx, expense.CategoryID); err != nil {
		return domain.Expense{}, err
	}
	return service.expenses.Create(ctx, expense)
}

// Get возвращает расход по положительному идентификатору.
func (service *ExpenseService) Get(ctx context.Context, id int64) (domain.Expense, error) {
	if id <= 0 {
		return domain.Expense{}, ErrInvalidID
	}
	return service.expenses.Get(ctx, id)
}

// List возвращает отфильтрованные расходы и их сумму с проверкой переполнения.
func (service *ExpenseService) List(ctx context.Context, filter ExpenseFilter) (ExpenseList, error) {
	if err := filter.Validate(); err != nil {
		return ExpenseList{}, err
	}
	filter.DateFrom = calendarDate(filter.DateFrom)
	filter.DateTo = calendarDate(filter.DateTo)
	expenses, err := service.expenses.List(ctx, filter)
	if err != nil {
		return ExpenseList{}, err
	}
	result := ExpenseList{Expenses: expenses}
	if result.Expenses == nil {
		result.Expenses = []domain.Expense{}
	}
	for _, expense := range expenses {
		if expense.AmountKopecks > math.MaxInt64-result.TotalKopecks {
			return ExpenseList{}, ErrTotalOverflow
		}
		result.TotalKopecks += expense.AmountKopecks
	}
	return result, nil
}

// Update проверяет расход и категорию, затем изменяет существующую запись.
func (service *ExpenseService) Update(ctx context.Context, expense domain.Expense) (domain.Expense, error) {
	if expense.ID <= 0 {
		return domain.Expense{}, ErrInvalidID
	}
	expense.Date = calendarDate(expense.Date)
	if err := expense.Validate(); err != nil {
		return domain.Expense{}, err
	}
	if _, err := service.expenses.Get(ctx, expense.ID); err != nil {
		return domain.Expense{}, err
	}
	if _, err := service.categories.Get(ctx, expense.CategoryID); err != nil {
		return domain.Expense{}, err
	}
	return service.expenses.Update(ctx, expense)
}

// Delete удаляет расход по положительному идентификатору.
func (service *ExpenseService) Delete(ctx context.Context, id int64) error {
	if id <= 0 {
		return ErrInvalidID
	}
	return service.expenses.Delete(ctx, id)
}
