//go:build integration

package postgres

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math"
	"os"
	"reflect"
	"testing"
	"testing/fstest"
	"time"

	"AccountingForExpenses/internal/application"
	"AccountingForExpenses/internal/domain"
	"AccountingForExpenses/migrations"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// testDatabase создаёт отдельную схему и удаляет только её после теста.
func testDatabase(t *testing.T, migrate bool) (context.Context, *pgxpool.Pool) {
	t.Helper()
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Fatal("задайте TEST_DATABASE_URL для запуска интеграционных тестов")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	t.Cleanup(cancel)
	admin, err := Open(ctx, databaseURL)
	requireNoError(t, err)
	t.Cleanup(admin.Close)
	schema := "expenses_test_" + rand.Text()
	identifier := pgx.Identifier{schema}.Sanitize()
	_, err = admin.Exec(ctx, "CREATE SCHEMA "+identifier)
	requireNoError(t, err)
	t.Cleanup(func() {
		cleanupContext, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		if _, err := admin.Exec(cleanupContext, "DROP SCHEMA "+identifier+" CASCADE"); err != nil {
			t.Errorf("cleanup: %v", err)
		}
	})
	config, err := pgxpool.ParseConfig(databaseURL)
	requireNoError(t, err)
	config.ConnConfig.RuntimeParams["search_path"] = identifier
	pool, err := pgxpool.NewWithConfig(ctx, config)
	requireNoError(t, err)
	t.Cleanup(pool.Close)
	requireNoError(t, pool.Ping(ctx))
	if migrate {
		requireNoError(t, Migrate(ctx, pool))
	}
	return ctx, pool
}

// requireNoError останавливает тест при неожиданной ошибке.
func requireNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// requireError сравнивает ожидаемую ошибку с результатом репозитория.
func requireError(t *testing.T, got, want error) {
	t.Helper()
	if !errors.Is(got, want) {
		t.Fatalf("error = %v, want %v", got, want)
	}
}

// TestCategoryRepository проверяет CRUD, порядок записей, ограничения и безопасную передачу текста.
func TestCategoryRepository(t *testing.T) {
	ctx, pool := testDatabase(t, true)
	repo := NewCategoryRepository(pool)
	empty, err := repo.List(ctx)
	requireNoError(t, err)
	if empty == nil || len(empty) != 0 {
		t.Fatal(empty)
	}
	category, err := repo.Create(ctx, domain.Category{Name: "Еда'; DROP TABLE categories; --"})
	requireNoError(t, err)
	if category.ID <= 0 {
		t.Fatal(category)
	}
	got, err := repo.Get(ctx, category.ID)
	requireNoError(t, err)
	if got != category {
		t.Fatalf("got %+v, want %+v", got, category)
	}
	category.Name = "Дом"
	updated, err := repo.Update(ctx, category)
	requireNoError(t, err)
	if updated != category {
		t.Fatal(updated)
	}
	second, err := repo.Create(ctx, domain.Category{Name: "Транспорт"})
	requireNoError(t, err)
	list, err := repo.List(ctx)
	requireNoError(t, err)
	if !reflect.DeepEqual(list, []domain.Category{category, second}) {
		t.Fatal(list)
	}
	for _, name := range []string{"", " \t\n\r\f\v"} {
		_, err = repo.Create(ctx, domain.Category{Name: name})
		requireError(t, err, domain.ErrEmptyCategoryName)
		_, err = repo.Update(ctx, domain.Category{ID: category.ID, Name: name})
		requireError(t, err, domain.ErrEmptyCategoryName)
	}
	requireNoError(t, repo.Delete(ctx, category.ID))
	_, err = repo.Get(ctx, category.ID)
	requireError(t, err, application.ErrCategoryNotFound)
	_, err = repo.Update(ctx, category)
	requireError(t, err, application.ErrCategoryNotFound)
	requireError(t, repo.Delete(ctx, category.ID), application.ErrCategoryNotFound)
}

// TestExpenseRepository проверяет CRUD, точность денег, связи и сохранность записи после ошибочного изменения.
func TestExpenseRepository(t *testing.T) {
	ctx, pool := testDatabase(t, true)
	categories := NewCategoryRepository(pool)
	expenses := NewExpenseRepository(pool)
	category, err := categories.Create(ctx, domain.Category{Name: "Дом"})
	requireNoError(t, err)
	second, err := categories.Create(ctx, domain.Category{Name: "Еда"})
	requireNoError(t, err)
	used, err := expenses.HasByCategory(ctx, category.ID)
	requireNoError(t, err)
	if used {
		t.Fatal("empty category is in use")
	}
	date := time.Date(2026, 9, 29, 0, 0, 0, 0, time.UTC)
	expense, err := expenses.Create(ctx, domain.Expense{CategoryID: category.ID, AmountKopecks: math.MaxInt64, Date: date, Comment: "Обед'; DELETE FROM expenses; --"})
	requireNoError(t, err)
	if expense.ID <= 0 || expense.AmountKopecks != math.MaxInt64 || !expense.Date.Equal(date) {
		t.Fatal(expense)
	}
	got, err := expenses.Get(ctx, expense.ID)
	requireNoError(t, err)
	if !reflect.DeepEqual(got, expense) {
		t.Fatal(got)
	}
	used, err = expenses.HasByCategory(ctx, category.ID)
	requireNoError(t, err)
	if !used {
		t.Fatal("category must be in use")
	}
	requireError(t, categories.Delete(ctx, category.ID), application.ErrCategoryInUse)
	for _, amount := range []int64{0, -1} {
		invalid := expense
		invalid.AmountKopecks = amount
		_, err = expenses.Create(ctx, invalid)
		requireError(t, err, domain.ErrInvalidAmount)
		_, err = expenses.Update(ctx, invalid)
		requireError(t, err, domain.ErrInvalidAmount)
	}
	invalid := expense
	invalid.CategoryID = 99999
	_, err = expenses.Create(ctx, invalid)
	requireError(t, err, application.ErrCategoryNotFound)
	_, err = expenses.Update(ctx, invalid)
	requireError(t, err, application.ErrCategoryNotFound)
	got, err = expenses.Get(ctx, expense.ID)
	requireNoError(t, err)
	if !reflect.DeepEqual(got, expense) {
		t.Fatal("failed update changed expense")
	}
	expense.CategoryID = second.ID
	expense.AmountKopecks = 1
	expense.Date = date.AddDate(0, 0, 1)
	expense.Comment = "Исправлено"
	updated, err := expenses.Update(ctx, expense)
	requireNoError(t, err)
	if !reflect.DeepEqual(updated, expense) {
		t.Fatal(updated)
	}
	requireNoError(t, categories.Delete(ctx, category.ID))
	requireNoError(t, expenses.Delete(ctx, expense.ID))
	_, err = expenses.Get(ctx, expense.ID)
	requireError(t, err, application.ErrExpenseNotFound)
	_, err = expenses.Update(ctx, expense)
	requireError(t, err, application.ErrExpenseNotFound)
	requireError(t, expenses.Delete(ctx, expense.ID), application.ErrExpenseNotFound)
	requireNoError(t, categories.Delete(ctx, second.ID))
}

// TestFiltersAndTotals проверяет SQL-фильтры и сумму прикладного сервиса на реальной БД.
func TestFiltersAndTotals(t *testing.T) {
	ctx, pool := testDatabase(t, true)
	categories := NewCategoryRepository(pool)
	expenses := NewExpenseRepository(pool)
	service := application.NewExpenseService(expenses, categories)
	first, err := categories.Create(ctx, domain.Category{Name: "Дом"})
	requireNoError(t, err)
	second, err := categories.Create(ctx, domain.Category{Name: "Еда"})
	requireNoError(t, err)
	date := time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC)
	saved := []domain.Expense{}
	for _, expense := range []domain.Expense{
		{CategoryID: first.ID, AmountKopecks: 101, Date: date},
		{CategoryID: first.ID, AmountKopecks: 202, Date: date.AddDate(0, 0, 1)},
		{CategoryID: second.ID, AmountKopecks: 303, Date: date.AddDate(0, 0, 1)},
		{CategoryID: first.ID, AmountKopecks: 404, Date: date.AddDate(0, 0, 2)},
	} {
		result, err := expenses.Create(ctx, expense)
		requireNoError(t, err)
		saved = append(saved, result)
	}
	tests := []struct {
		name   string
		filter application.ExpenseFilter
		ids    []int64
		total  int64
	}{
		{"all", application.ExpenseFilter{}, []int64{saved[3].ID, saved[2].ID, saved[1].ID, saved[0].ID}, 1010},
		{"category", application.ExpenseFilter{CategoryID: first.ID}, []int64{saved[3].ID, saved[1].ID, saved[0].ID}, 707},
		{"from", application.ExpenseFilter{DateFrom: date.AddDate(0, 0, 1)}, []int64{saved[3].ID, saved[2].ID, saved[1].ID}, 909},
		{"to", application.ExpenseFilter{DateTo: date.AddDate(0, 0, 1)}, []int64{saved[2].ID, saved[1].ID, saved[0].ID}, 606},
		{"period", application.ExpenseFilter{DateFrom: date, DateTo: date.AddDate(0, 0, 1)}, []int64{saved[2].ID, saved[1].ID, saved[0].ID}, 606},
		{"category from", application.ExpenseFilter{CategoryID: first.ID, DateFrom: date.AddDate(0, 0, 1)}, []int64{saved[3].ID, saved[1].ID}, 606},
		{"category to", application.ExpenseFilter{CategoryID: first.ID, DateTo: date.AddDate(0, 0, 1)}, []int64{saved[1].ID, saved[0].ID}, 303},
		{"all filters", application.ExpenseFilter{CategoryID: first.ID, DateFrom: date, DateTo: date.AddDate(0, 0, 1)}, []int64{saved[1].ID, saved[0].ID}, 303},
		{"same day", application.ExpenseFilter{DateFrom: date.Add(23 * time.Hour), DateTo: date}, []int64{saved[0].ID}, 101},
		{"empty", application.ExpenseFilter{CategoryID: 99999}, []int64{}, 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := service.List(ctx, test.filter)
			requireNoError(t, err)
			ids := []int64{}
			for _, expense := range result.Expenses {
				ids = append(ids, expense.ID)
			}
			if !reflect.DeepEqual(ids, test.ids) || result.TotalKopecks != test.total {
				t.Fatalf("IDs %v, total %d", ids, result.TotalKopecks)
			}
		})
	}
}

// TestDatabaseConstraints проверяет ограничения без участия прикладной валидации.
func TestDatabaseConstraints(t *testing.T) {
	ctx, pool := testDatabase(t, true)
	category, err := NewCategoryRepository(pool).Create(ctx, domain.Category{Name: "Дом"})
	requireNoError(t, err)
	tests := []struct{ name, query, code string }{
		{"null name", `INSERT INTO categories (name) VALUES (NULL)`, "23502"},
		{"null date", fmt.Sprintf(`INSERT INTO expenses (category_id,amount_kopecks,expense_date) VALUES (%d,1,NULL)`, category.ID), "23502"},
		{"infinite date", fmt.Sprintf(`INSERT INTO expenses (category_id,amount_kopecks,expense_date) VALUES (%d,1,'infinity')`, category.ID), "23514"},
		{"null category", `INSERT INTO expenses (category_id,amount_kopecks,expense_date) VALUES (NULL,1,'2026-09-29')`, "23502"},
		{"null amount", fmt.Sprintf(`INSERT INTO expenses (category_id,amount_kopecks,expense_date) VALUES (%d,NULL,'2026-09-29')`, category.ID), "23502"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := pool.Exec(ctx, test.query)
			var pgError *pgconn.PgError
			if !errors.As(err, &pgError) || pgError.Code != test.code {
				t.Fatalf("error = %v, want SQLSTATE %s", err, test.code)
			}
		})
	}
}

// TestMigrations проверяет применение с нуля, повторный запуск и сохранение данных.
func TestMigrations(t *testing.T) {
	ctx, pool := testDatabase(t, false)
	requireNoError(t, Migrate(ctx, pool))
	category, err := NewCategoryRepository(pool).Create(ctx, domain.Category{Name: "Дом"})
	requireNoError(t, err)
	requireNoError(t, Migrate(ctx, pool))
	got, err := NewCategoryRepository(pool).Get(ctx, category.ID)
	requireNoError(t, err)
	if got != category {
		t.Fatal(got)
	}
	var count int
	requireNoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM schema_migrations`).Scan(&count))
	if count != 1 {
		t.Fatal(count)
	}
}

// TestConcurrentMigrations проверяет сериализацию двух одновременных запусков.
func TestConcurrentMigrations(t *testing.T) {
	ctx, pool := testDatabase(t, false)
	results := make(chan error, 2)
	for range 2 {
		go func() { results <- Migrate(ctx, pool) }()
	}
	for range 2 {
		requireNoError(t, <-results)
	}
	var count int
	requireNoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM schema_migrations`).Scan(&count))
	if count != 1 {
		t.Fatal(count)
	}
}

// TestMigrationRollback проверяет полный откат неудачной миграции и возможность повторного запуска.
func TestMigrationRollback(t *testing.T) {
	ctx, pool := testDatabase(t, false)
	files := fstest.MapFS{"001_failure.sql": {Data: []byte(`CREATE TABLE rollback_probe (id BIGINT); SELECT missing_column FROM rollback_probe;`)}}
	if err := applyMigrations(ctx, pool, files); err == nil {
		t.Fatal("expected migration failure")
	}
	var absent bool
	requireNoError(t, pool.QueryRow(ctx, `SELECT to_regclass('rollback_probe') IS NULL AND to_regclass('schema_migrations') IS NULL`).Scan(&absent))
	if !absent {
		t.Fatal("failed migration left tables")
	}
	requireNoError(t, Migrate(ctx, pool))
}

// TestMigrationChecksum запрещает незаметное изменение уже применённого SQL-файла.
func TestMigrationChecksum(t *testing.T) {
	ctx, pool := testDatabase(t, true)
	entries, err := migrations.Files.ReadDir(".")
	requireNoError(t, err)
	changed := fstest.MapFS{}
	for _, entry := range entries {
		content, err := migrations.Files.ReadFile(entry.Name())
		requireNoError(t, err)
		changed[entry.Name()] = &fstest.MapFile{Data: append(content, []byte("\n-- changed\n")...)}
	}
	if err := applyMigrations(ctx, pool, changed); err == nil {
		t.Fatal("expected checksum mismatch")
	}
	requireNoError(t, Migrate(ctx, pool))
}

// TestConcurrentCategoryDeletion проверяет внешний ключ при одновременном создании расхода и удалении категории.
func TestConcurrentCategoryDeletion(t *testing.T) {
	ctx, pool := testDatabase(t, true)
	categories := NewCategoryRepository(pool)
	expenses := NewExpenseRepository(pool)
	for range 10 {
		category, err := categories.Create(ctx, domain.Category{Name: "Дом"})
		requireNoError(t, err)
		start := make(chan struct{})
		created := make(chan error, 1)
		deleted := make(chan error, 1)
		go func() {
			<-start
			_, err := expenses.Create(ctx, domain.Expense{CategoryID: category.ID, AmountKopecks: 1, Date: time.Date(2026, 9, 29, 0, 0, 0, 0, time.UTC)})
			created <- err
		}()
		go func() { <-start; deleted <- categories.Delete(ctx, category.ID) }()
		close(start)
		createError, deleteError := <-created, <-deleted
		if createError == nil {
			requireError(t, deleteError, application.ErrCategoryInUse)
		} else {
			requireError(t, createError, application.ErrCategoryNotFound)
			requireNoError(t, deleteError)
		}
	}
	var orphans int
	requireNoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM expenses e LEFT JOIN categories c ON c.id=e.category_id WHERE c.id IS NULL`).Scan(&orphans))
	if orphans != 0 {
		t.Fatal(orphans)
	}
}

// TestCanceledContext проверяет, что репозиторий не скрывает отмену запроса.
func TestCanceledContext(t *testing.T) {
	ctx, pool := testDatabase(t, true)
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	_, err := NewCategoryRepository(pool).List(canceled)
	requireError(t, err, context.Canceled)
	_, err = NewExpenseRepository(pool).List(canceled, application.ExpenseFilter{})
	requireError(t, err, context.Canceled)
}
