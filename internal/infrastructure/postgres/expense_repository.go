package postgres

import (
	"context"
	"fmt"
	"strings"

	"AccountingForExpenses/internal/application"
	"AccountingForExpenses/internal/domain"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ExpenseRepository struct{ pool *pgxpool.Pool }

var _ application.ExpenseRepository = (*ExpenseRepository)(nil)

// NewExpenseRepository создаёт репозиторий расходов с общим пулом соединений.
func NewExpenseRepository(pool *pgxpool.Pool) *ExpenseRepository {
	return &ExpenseRepository{pool: pool}
}

// scanExpense читает поля расхода и преобразует ошибки отдельной записи.
func scanExpense(row pgx.Row) (domain.Expense, error) {
	var expense domain.Expense
	err := row.Scan(&expense.ID, &expense.CategoryID, &expense.AmountKopecks, &expense.Date, &expense.Comment)
	if err != nil {
		return domain.Expense{}, translateError(err, application.ErrExpenseNotFound, application.ErrCategoryNotFound)
	}
	return expense, nil
}

// Create сохраняет расход, проверяя существование категории внешним ключом.
func (repo *ExpenseRepository) Create(ctx context.Context, expense domain.Expense) (domain.Expense, error) {
	return scanExpense(repo.pool.QueryRow(ctx, `
  INSERT INTO expenses (category_id, amount_kopecks, expense_date, comment)
  VALUES ($1, $2, $3, $4)
  RETURNING id, category_id, amount_kopecks, expense_date, comment`,
		expense.CategoryID, expense.AmountKopecks, expense.Date, expense.Comment))
}

// Get возвращает расход по идентификатору или ошибку отсутствующей записи.
func (repo *ExpenseRepository) Get(ctx context.Context, id int64) (domain.Expense, error) {
	return scanExpense(repo.pool.QueryRow(ctx, `
  SELECT id, category_id, amount_kopecks, expense_date, comment FROM expenses WHERE id = $1`, id))
}

// List применяет фильтры и возвращает расходы по убыванию даты и идентификатора.
func (repo *ExpenseRepository) List(ctx context.Context, filter application.ExpenseFilter) ([]domain.Expense, error) {
	query := `SELECT id, category_id, amount_kopecks, expense_date, comment FROM expenses`
	conditions := []string{}
	arguments := []any{}
	if filter.CategoryID != 0 {
		arguments = append(arguments, filter.CategoryID)
		conditions = append(conditions, fmt.Sprintf("category_id = $%d", len(arguments)))
	}
	if !filter.DateFrom.IsZero() {
		arguments = append(arguments, filter.DateFrom)
		conditions = append(conditions, fmt.Sprintf("expense_date >= $%d", len(arguments)))
	}
	if !filter.DateTo.IsZero() {
		arguments = append(arguments, filter.DateTo)
		conditions = append(conditions, fmt.Sprintf("expense_date <= $%d", len(arguments)))
	}
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY expense_date DESC, id DESC"
	rows, err := repo.pool.Query(ctx, query, arguments...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	expenses := []domain.Expense{}
	for rows.Next() {
		expense, err := scanExpense(rows)
		if err != nil {
			return nil, err
		}
		expenses = append(expenses, expense)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return expenses, nil
}

// Update заменяет поля существующего расхода, сохраняя связь с категорией.
func (repo *ExpenseRepository) Update(ctx context.Context, expense domain.Expense) (domain.Expense, error) {
	return scanExpense(repo.pool.QueryRow(ctx, `
  UPDATE expenses SET category_id = $2, amount_kopecks = $3, expense_date = $4, comment = $5
  WHERE id = $1 RETURNING id, category_id, amount_kopecks, expense_date, comment`,
		expense.ID, expense.CategoryID, expense.AmountKopecks, expense.Date, expense.Comment))
}

// Delete удаляет расход и сообщает, если записи не было.
func (repo *ExpenseRepository) Delete(ctx context.Context, id int64) error {
	result, err := repo.pool.Exec(ctx, `DELETE FROM expenses WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return application.ErrExpenseNotFound
	}
	return nil
}

// HasByCategory проверяет наличие хотя бы одного расхода у категории.
func (repo *ExpenseRepository) HasByCategory(ctx context.Context, id int64) (bool, error) {
	var exists bool
	err := repo.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM expenses WHERE category_id = $1)`, id).Scan(&exists)
	return exists, err
}
