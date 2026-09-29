package httpapi

import (
	"context"
	"sort"

	"AccountingForExpenses/internal/application"
	"AccountingForExpenses/internal/domain"
)

type categoryMemory struct {
	items  map[int64]domain.Category
	nextID int64
}

// Create сохраняет тестовую категорию с новым идентификатором.
func (repo *categoryMemory) Create(_ context.Context, category domain.Category) (domain.Category, error) {
	repo.nextID++
	category.ID = repo.nextID
	repo.items[category.ID] = category
	return category, nil
}

// Get находит тестовую категорию по идентификатору.
func (repo *categoryMemory) Get(_ context.Context, id int64) (domain.Category, error) {
	category, ok := repo.items[id]
	if !ok {
		return domain.Category{}, application.ErrCategoryNotFound
	}
	return category, nil
}

// List возвращает тестовые категории в порядке идентификаторов.
func (repo *categoryMemory) List(_ context.Context) ([]domain.Category, error) {
	result := []domain.Category{}
	for _, category := range repo.items {
		result = append(result, category)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result, nil
}

// Update заменяет существующую тестовую категорию.
func (repo *categoryMemory) Update(ctx context.Context, category domain.Category) (domain.Category, error) {
	if _, err := repo.Get(ctx, category.ID); err != nil {
		return domain.Category{}, err
	}
	repo.items[category.ID] = category
	return category, nil
}

// Delete удаляет тестовую категорию после проверки существования.
func (repo *categoryMemory) Delete(ctx context.Context, id int64) error {
	if _, err := repo.Get(ctx, id); err != nil {
		return err
	}
	delete(repo.items, id)
	return nil
}

type expenseMemory struct {
	items  map[int64]domain.Expense
	nextID int64
}

// Create сохраняет тестовый расход с новым идентификатором.
func (repo *expenseMemory) Create(_ context.Context, expense domain.Expense) (domain.Expense, error) {
	repo.nextID++
	expense.ID = repo.nextID
	repo.items[expense.ID] = expense
	return expense, nil
}

// Get находит тестовый расход по идентификатору.
func (repo *expenseMemory) Get(_ context.Context, id int64) (domain.Expense, error) {
	expense, ok := repo.items[id]
	if !ok {
		return domain.Expense{}, application.ErrExpenseNotFound
	}
	return expense, nil
}

// List отбирает тестовые расходы по категории и включительным границам дат.
func (repo *expenseMemory) List(_ context.Context, filter application.ExpenseFilter) ([]domain.Expense, error) {
	result := []domain.Expense{}
	for _, expense := range repo.items {
		if filter.CategoryID != 0 && expense.CategoryID != filter.CategoryID {
			continue
		}
		if !filter.DateFrom.IsZero() && expense.Date.Before(filter.DateFrom) {
			continue
		}
		if !filter.DateTo.IsZero() && expense.Date.After(filter.DateTo) {
			continue
		}
		result = append(result, expense)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Date.Equal(result[j].Date) {
			return result[i].ID > result[j].ID
		}
		return result[i].Date.After(result[j].Date)
	})
	return result, nil
}

// Update заменяет существующий тестовый расход.
func (repo *expenseMemory) Update(ctx context.Context, expense domain.Expense) (domain.Expense, error) {
	if _, err := repo.Get(ctx, expense.ID); err != nil {
		return domain.Expense{}, err
	}
	repo.items[expense.ID] = expense
	return expense, nil
}

// Delete удаляет существующий тестовый расход.
func (repo *expenseMemory) Delete(ctx context.Context, id int64) error {
	if _, err := repo.Get(ctx, id); err != nil {
		return err
	}
	delete(repo.items, id)
	return nil
}

// HasByCategory проверяет связь тестовых расходов с категорией.
func (repo *expenseMemory) HasByCategory(_ context.Context, id int64) (bool, error) {
	for _, expense := range repo.items {
		if expense.CategoryID == id {
			return true, nil
		}
	}
	return false, nil
}
