package application

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/AlexeyDimitriev/AccountingForExpenses/internal/domain"
)

type repositoryFailure struct {
	operation string
	err       error
	ctx       context.Context
	t         *testing.T
}

// check проверяет передачу контекста и имитирует сбой выбранной операции.
func (failure *repositoryFailure) check(ctx context.Context, operation string) error {
	if ctx != failure.ctx {
		failure.t.Fatal("repository received another context")
	}
	if operation == failure.operation {
		return failure.err
	}
	return nil
}

type failingCategories struct {
	CategoryRepository
	failure *repositoryFailure
}

type failingExpenses struct {
	ExpenseRepository
	failure *repositoryFailure
}

// Create имитирует сбой репозитория или передаёт вызов тестовому хранилищу.
func (repo *failingCategories) Create(ctx context.Context, value domain.Category) (domain.Category, error) {
	if err := repo.failure.check(ctx, "category.Create"); err != nil {
		return domain.Category{}, err
	}
	return repo.CategoryRepository.Create(ctx, value)
}

// Get имитирует сбой репозитория или передаёт вызов тестовому хранилищу.
func (repo *failingCategories) Get(ctx context.Context, id int64) (domain.Category, error) {
	if err := repo.failure.check(ctx, "category.Get"); err != nil {
		return domain.Category{}, err
	}
	return repo.CategoryRepository.Get(ctx, id)
}

// List имитирует сбой репозитория или передаёт вызов тестовому хранилищу.
func (repo *failingCategories) List(ctx context.Context) ([]domain.Category, error) {
	if err := repo.failure.check(ctx, "category.List"); err != nil {
		return nil, err
	}
	return repo.CategoryRepository.List(ctx)
}

// Update имитирует сбой репозитория или передаёт вызов тестовому хранилищу.
func (repo *failingCategories) Update(ctx context.Context, value domain.Category) (domain.Category, error) {
	if err := repo.failure.check(ctx, "category.Update"); err != nil {
		return domain.Category{}, err
	}
	return repo.CategoryRepository.Update(ctx, value)
}

// Delete имитирует сбой репозитория или передаёт вызов тестовому хранилищу.
func (repo *failingCategories) Delete(ctx context.Context, id int64) error {
	if err := repo.failure.check(ctx, "category.Delete"); err != nil {
		return err
	}
	return repo.CategoryRepository.Delete(ctx, id)
}

// Create имитирует сбой репозитория или передаёт вызов тестовому хранилищу.
func (repo *failingExpenses) Create(ctx context.Context, value domain.Expense) (domain.Expense, error) {
	if err := repo.failure.check(ctx, "expense.Create"); err != nil {
		return domain.Expense{}, err
	}
	return repo.ExpenseRepository.Create(ctx, value)
}

// Get имитирует сбой репозитория или передаёт вызов тестовому хранилищу.
func (repo *failingExpenses) Get(ctx context.Context, id int64) (domain.Expense, error) {
	if err := repo.failure.check(ctx, "expense.Get"); err != nil {
		return domain.Expense{}, err
	}
	return repo.ExpenseRepository.Get(ctx, id)
}

// List имитирует сбой репозитория или передаёт вызов тестовому хранилищу.
func (repo *failingExpenses) List(ctx context.Context, filter ExpenseFilter) ([]domain.Expense, error) {
	if err := repo.failure.check(ctx, "expense.List"); err != nil {
		return nil, err
	}
	return repo.ExpenseRepository.List(ctx, filter)
}

// Update имитирует сбой репозитория или передаёт вызов тестовому хранилищу.
func (repo *failingExpenses) Update(ctx context.Context, value domain.Expense) (domain.Expense, error) {
	if err := repo.failure.check(ctx, "expense.Update"); err != nil {
		return domain.Expense{}, err
	}
	return repo.ExpenseRepository.Update(ctx, value)
}

// Delete имитирует сбой репозитория или передаёт вызов тестовому хранилищу.
func (repo *failingExpenses) Delete(ctx context.Context, id int64) error {
	if err := repo.failure.check(ctx, "expense.Delete"); err != nil {
		return err
	}
	return repo.ExpenseRepository.Delete(ctx, id)
}

// HasByCategory имитирует сбой репозитория или передаёт вызов тестовому хранилищу.
func (repo *failingExpenses) HasByCategory(ctx context.Context, id int64) (bool, error) {
	if err := repo.failure.check(ctx, "expense.HasByCategory"); err != nil {
		return false, err
	}
	return repo.ExpenseRepository.HasByCategory(ctx, id)
}

// TestRepositoryFailures проверяет передачу контекста и сохранение ошибок на каждом шаге сценариев.
func TestRepositoryFailures(t *testing.T) {
	tests := []struct {
		name, operation string
		run             func(context.Context, *CategoryService, *ExpenseService, domain.Expense) error
	}{
		{"category create", "category.Create", func(ctx context.Context, categories *CategoryService, expenses *ExpenseService, expense domain.Expense) error {
			_, err := categories.Create(ctx, "Дом")
			return err
		}},
		{"category get", "category.Get", func(ctx context.Context, categories *CategoryService, expenses *ExpenseService, expense domain.Expense) error {
			_, err := categories.Get(ctx, 1)
			return err
		}},
		{"category list", "category.List", func(ctx context.Context, categories *CategoryService, expenses *ExpenseService, expense domain.Expense) error {
			_, err := categories.List(ctx)
			return err
		}},
		{"category update", "category.Update", func(ctx context.Context, categories *CategoryService, expenses *ExpenseService, expense domain.Expense) error {
			_, err := categories.Update(ctx, domain.Category{ID: 1, Name: "Дом"})
			return err
		}},
		{"category delete category.Get", "category.Get", func(ctx context.Context, categories *CategoryService, expenses *ExpenseService, expense domain.Expense) error {
			return categories.Delete(ctx, 1)
		}},
		{"category delete expense.HasByCategory", "expense.HasByCategory", func(ctx context.Context, categories *CategoryService, expenses *ExpenseService, expense domain.Expense) error {
			return categories.Delete(ctx, 1)
		}},
		{"category delete category.Delete", "category.Delete", func(ctx context.Context, categories *CategoryService, expenses *ExpenseService, expense domain.Expense) error {
			return categories.Delete(ctx, 1)
		}},
		{"expense create category.Get", "category.Get", func(ctx context.Context, categories *CategoryService, expenses *ExpenseService, expense domain.Expense) error {
			_, err := expenses.Create(ctx, expense)
			return err
		}},
		{"expense create expense.Create", "expense.Create", func(ctx context.Context, categories *CategoryService, expenses *ExpenseService, expense domain.Expense) error {
			_, err := expenses.Create(ctx, expense)
			return err
		}},
		{"expense update expense.Get", "expense.Get", func(ctx context.Context, categories *CategoryService, expenses *ExpenseService, expense domain.Expense) error {
			_, err := expenses.Update(ctx, expense)
			return err
		}},
		{"expense update category.Get", "category.Get", func(ctx context.Context, categories *CategoryService, expenses *ExpenseService, expense domain.Expense) error {
			_, err := expenses.Update(ctx, expense)
			return err
		}},
		{"expense update expense.Update", "expense.Update", func(ctx context.Context, categories *CategoryService, expenses *ExpenseService, expense domain.Expense) error {
			_, err := expenses.Update(ctx, expense)
			return err
		}},
		{"expense get", "expense.Get", func(ctx context.Context, categories *CategoryService, expenses *ExpenseService, expense domain.Expense) error {
			_, err := expenses.Get(ctx, 1)
			return err
		}},
		{"expense list", "expense.List", func(ctx context.Context, categories *CategoryService, expenses *ExpenseService, expense domain.Expense) error {
			_, err := expenses.List(ctx, ExpenseFilter{})
			return err
		}},
		{"expense delete", "expense.Delete", func(ctx context.Context, categories *CategoryService, expenses *ExpenseService, expense domain.Expense) error {
			return expenses.Delete(ctx, 1)
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			expected := errors.New("storage unavailable")
			failure := &repositoryFailure{operation: test.operation, err: fmt.Errorf("repository: %w", expected), ctx: ctx, t: t}
			expense := domain.Expense{ID: 1, CategoryID: 1, AmountKopecks: 100, Date: time.Date(2026, 9, 29, 0, 0, 0, 0, time.UTC)}
			categories := &failingCategories{CategoryRepository: &categoryMemory{items: map[int64]domain.Category{1: {ID: 1, Name: "Дом"}}, nextID: 1}, failure: failure}
			expenses := &failingExpenses{ExpenseRepository: &expenseMemory{items: map[int64]domain.Expense{1: expense}, nextID: 1}, failure: failure}
			if test.operation == "category.Delete" {
				expenses.ExpenseRepository = &expenseMemory{items: make(map[int64]domain.Expense)}
			}
			err := test.run(ctx, NewCategoryService(categories, expenses), NewExpenseService(expenses, categories), expense)
			requireError(t, err, expected)
		})
	}
}
