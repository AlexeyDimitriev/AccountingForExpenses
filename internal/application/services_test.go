package application

import (
	"context"
	"errors"
	"math"
	"reflect"
	"sort"
	"testing"
	"time"

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
		return domain.Category{}, ErrCategoryNotFound
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
		return domain.Expense{}, ErrExpenseNotFound
	}
	return expense, nil
}

// List отбирает тестовые расходы по категории и включительным границам дат.
func (repo *expenseMemory) List(_ context.Context, filter ExpenseFilter) ([]domain.Expense, error) {
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

// newServices создаёт изолированные сервисы с хранилищами в памяти.
func newServices() (*CategoryService, *ExpenseService) {
	categories := &categoryMemory{items: make(map[int64]domain.Category)}
	expenses := &expenseMemory{items: make(map[int64]domain.Expense)}
	return NewCategoryService(categories, expenses), NewExpenseService(expenses, categories)
}

// requireError сравнивает ошибку сценария с ожидаемой, включая обёрнутые ошибки.
func requireError(t *testing.T, got, want error) {
	t.Helper()
	if !errors.Is(got, want) {
		t.Fatalf("error = %v, want %v", got, want)
	}
}

// TestServicesCRUD проверяет полный цикл записей и защиту используемой категории.
func TestServicesCRUD(t *testing.T) {
	ctx := context.Background()
	categories, expenses := newServices()
	category, err := categories.Create(ctx, "  Продукты  ")
	requireError(t, err, nil)
	if category.ID <= 0 || category.Name != "Продукты" {
		t.Fatalf("unexpected category: %+v", category)
	}
	category.Name = "  Покупки  "
	category, err = categories.Update(ctx, category)
	requireError(t, err, nil)
	if category.Name != "Покупки" {
		t.Fatal(category)
	}
	gotCategory, err := categories.Get(ctx, category.ID)
	requireError(t, err, nil)
	if gotCategory != category {
		t.Fatal(gotCategory)
	}
	allCategories, err := categories.List(ctx)
	requireError(t, err, nil)
	if !reflect.DeepEqual(allCategories, []domain.Category{category}) {
		t.Fatal(allCategories)
	}
	date := time.Date(2026, 9, 29, 23, 30, 0, 0, time.FixedZone("local", -3*3600))
	expense, err := expenses.Create(ctx, domain.Expense{ID: 999, CategoryID: category.ID, AmountKopecks: 12345, Date: date})
	requireError(t, err, nil)
	if expense.ID <= 0 || expense.ID == 999 || expense.Date != time.Date(2026, 9, 29, 0, 0, 0, 0, time.UTC) {
		t.Fatal(expense)
	}
	requireError(t, categories.Delete(ctx, category.ID), ErrCategoryInUse)
	expense.AmountKopecks = 1
	expense.Comment = "Исправлено"
	updated, err := expenses.Update(ctx, expense)
	requireError(t, err, nil)
	if updated != expense {
		t.Fatal(updated)
	}
	gotExpense, err := expenses.Get(ctx, expense.ID)
	requireError(t, err, nil)
	if gotExpense != expense {
		t.Fatal(gotExpense)
	}
	requireError(t, expenses.Delete(ctx, expense.ID), nil)
	_, err = expenses.Get(ctx, expense.ID)
	requireError(t, err, ErrExpenseNotFound)
	requireError(t, categories.Delete(ctx, category.ID), nil)
	_, err = categories.Get(ctx, category.ID)
	requireError(t, err, ErrCategoryNotFound)
	result, err := expenses.List(ctx, ExpenseFilter{})
	requireError(t, err, nil)
	if result.Expenses == nil || len(result.Expenses) != 0 || result.TotalKopecks != 0 {
		t.Fatal(result)
	}
}

// TestInvalidInput проверяет, что некорректные данные отклоняются до обращения к хранилищам.
func TestInvalidInput(t *testing.T) {
	ctx := context.Background()
	categories := NewCategoryService(nil, nil)
	expenses := NewExpenseService(nil, nil)
	date := time.Date(2026, 9, 29, 0, 0, 0, 0, time.UTC)
	_, err := categories.Create(ctx, " \t")
	requireError(t, err, domain.ErrEmptyCategoryName)
	_, err = categories.Update(ctx, domain.Category{ID: 1})
	requireError(t, err, domain.ErrEmptyCategoryName)
	for _, id := range []int64{0, -1} {
		_, err = categories.Get(ctx, id)
		requireError(t, err, ErrInvalidID)
		_, err = categories.Update(ctx, domain.Category{ID: id, Name: "Дом"})
		requireError(t, err, ErrInvalidID)
		requireError(t, categories.Delete(ctx, id), ErrInvalidID)
		_, err = expenses.Get(ctx, id)
		requireError(t, err, ErrInvalidID)
		_, err = expenses.Update(ctx, domain.Expense{ID: id})
		requireError(t, err, ErrInvalidID)
		requireError(t, expenses.Delete(ctx, id), ErrInvalidID)
	}
	tests := []struct {
		expense domain.Expense
		want    error
	}{
		{domain.Expense{ID: 1, AmountKopecks: 1, Date: date}, domain.ErrInvalidCategoryID},
		{domain.Expense{ID: 1, CategoryID: 1, Date: date}, domain.ErrInvalidAmount},
		{domain.Expense{ID: 1, CategoryID: 1, AmountKopecks: 1}, domain.ErrInvalidDate},
	}
	for _, test := range tests {
		_, err = expenses.Create(ctx, test.expense)
		requireError(t, err, test.want)
		_, err = expenses.Update(ctx, test.expense)
		requireError(t, err, test.want)
	}
	_, err = expenses.List(ctx, ExpenseFilter{CategoryID: -1})
	requireError(t, err, domain.ErrInvalidCategoryID)
	_, err = expenses.List(ctx, ExpenseFilter{DateFrom: date.AddDate(0, 0, 1), DateTo: date})
	requireError(t, err, ErrInvalidPeriod)
}

// TestMissingRecords проверяет ошибки отсутствующих записей и неизменность расхода при неверной категории.
func TestMissingRecords(t *testing.T) {
	ctx := context.Background()
	categories, expenses := newServices()
	requireError(t, categories.Delete(ctx, 99), ErrCategoryNotFound)
	_, err := categories.Update(ctx, domain.Category{ID: 99, Name: "Дом"})
	requireError(t, err, ErrCategoryNotFound)
	requireError(t, expenses.Delete(ctx, 99), ErrExpenseNotFound)
	expense := domain.Expense{CategoryID: 99, AmountKopecks: 100, Date: time.Date(2026, 9, 29, 0, 0, 0, 0, time.UTC)}
	_, err = expenses.Create(ctx, expense)
	requireError(t, err, ErrCategoryNotFound)
	expense.ID = 99
	_, err = expenses.Update(ctx, expense)
	requireError(t, err, ErrExpenseNotFound)
	category, err := categories.Create(ctx, "Дом")
	requireError(t, err, nil)
	expense.CategoryID = category.ID
	saved, err := expenses.Create(ctx, expense)
	requireError(t, err, nil)
	changed := saved
	changed.CategoryID = 99
	_, err = expenses.Update(ctx, changed)
	requireError(t, err, ErrCategoryNotFound)
	got, err := expenses.Get(ctx, saved.ID)
	requireError(t, err, nil)
	if got != saved {
		t.Fatal("failed update changed expense")
	}
}

// TestExpenseFilters проверяет сочетания фильтров, границы периода и итог в копейках.
func TestExpenseFilters(t *testing.T) {
	ctx := context.Background()
	categories, expenses := newServices()
	first, err := categories.Create(ctx, "Дом")
	requireError(t, err, nil)
	second, err := categories.Create(ctx, "Еда")
	requireError(t, err, nil)
	date := time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC)
	for _, expense := range []domain.Expense{
		{CategoryID: first.ID, AmountKopecks: 101, Date: date},
		{CategoryID: first.ID, AmountKopecks: 202, Date: date.AddDate(0, 0, 1)},
		{CategoryID: second.ID, AmountKopecks: 303, Date: date.AddDate(0, 0, 1)},
		{CategoryID: first.ID, AmountKopecks: 404, Date: date.AddDate(0, 0, 2)},
	} {
		_, err := expenses.Create(ctx, expense)
		requireError(t, err, nil)
	}
	tests := []struct {
		name   string
		filter ExpenseFilter
		ids    []int64
		total  int64
	}{
		{"all", ExpenseFilter{}, []int64{4, 3, 2, 1}, 1010},
		{"category", ExpenseFilter{CategoryID: first.ID}, []int64{4, 2, 1}, 707},
		{"from inclusive", ExpenseFilter{DateFrom: date.AddDate(0, 0, 1)}, []int64{4, 3, 2}, 909},
		{"to inclusive", ExpenseFilter{DateTo: date.AddDate(0, 0, 1)}, []int64{3, 2, 1}, 606},
		{"combined", ExpenseFilter{CategoryID: first.ID, DateFrom: date, DateTo: date.AddDate(0, 0, 1)}, []int64{2, 1}, 303},
		{"same calendar day", ExpenseFilter{DateFrom: date.Add(23 * time.Hour), DateTo: date}, []int64{1}, 101},
		{"unknown category", ExpenseFilter{CategoryID: 99}, []int64{}, 0},
		{"empty period", ExpenseFilter{DateFrom: date.AddDate(0, 0, 3)}, []int64{}, 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := expenses.List(ctx, test.filter)
			requireError(t, err, nil)
			ids := []int64{}
			for _, expense := range result.Expenses {
				ids = append(ids, expense.ID)
			}
			if !reflect.DeepEqual(ids, test.ids) || result.TotalKopecks != test.total {
				t.Fatalf("got IDs %v, total %d", ids, result.TotalKopecks)
			}
		})
	}
}

// TestTotalOverflow проверяет точное максимальное значение суммы и отказ при переполнении.
func TestTotalOverflow(t *testing.T) {
	ctx := context.Background()
	categories, expenses := newServices()
	category, err := categories.Create(ctx, "Дом")
	requireError(t, err, nil)
	expense := domain.Expense{CategoryID: category.ID, AmountKopecks: math.MaxInt64, Date: time.Date(2026, 9, 29, 0, 0, 0, 0, time.UTC)}
	_, err = expenses.Create(ctx, expense)
	requireError(t, err, nil)
	result, err := expenses.List(ctx, ExpenseFilter{})
	requireError(t, err, nil)
	if result.TotalKopecks != math.MaxInt64 {
		t.Fatal(result.TotalKopecks)
	}
	expense.AmountKopecks = 1
	_, err = expenses.Create(ctx, expense)
	requireError(t, err, nil)
	_, err = expenses.List(ctx, ExpenseFilter{})
	requireError(t, err, ErrTotalOverflow)
}
