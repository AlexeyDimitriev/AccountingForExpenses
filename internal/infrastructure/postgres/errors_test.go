package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"AccountingForExpenses/internal/application"
	"AccountingForExpenses/internal/domain"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// TestTranslateError проверяет преобразование ограничений БД и сохранение неизвестных ошибок.
func TestTranslateError(t *testing.T) {
	unknown := &pgconn.PgError{Code: "08006", Message: "connection failure"}
	unknownConstraint := &pgconn.PgError{Code: "23514", ConstraintName: "another_constraint"}
	tests := []struct {
		name       string
		err        error
		notFound   error
		foreignKey error
		want       error
	}{
		{name: "success"},
		{name: "missing category", err: pgx.ErrNoRows, notFound: application.ErrCategoryNotFound, want: application.ErrCategoryNotFound},
		{name: "missing expense", err: fmt.Errorf("query: %w", pgx.ErrNoRows), notFound: application.ErrExpenseNotFound, want: application.ErrExpenseNotFound},
		{name: "category in use", err: &pgconn.PgError{Code: "23503", ConstraintName: "expenses_category_fk"}, foreignKey: application.ErrCategoryInUse, want: application.ErrCategoryInUse},
		{name: "unknown category", err: &pgconn.PgError{Code: "23503", ConstraintName: "expenses_category_fk"}, foreignKey: application.ErrCategoryNotFound, want: application.ErrCategoryNotFound},
		{name: "empty name", err: &pgconn.PgError{Code: "23514", ConstraintName: "categories_name_not_empty"}, want: domain.ErrEmptyCategoryName},
		{name: "invalid amount", err: &pgconn.PgError{Code: "23514", ConstraintName: "expenses_amount_positive"}, want: domain.ErrInvalidAmount},
		{name: "invalid date", err: &pgconn.PgError{Code: "23514", ConstraintName: "expenses_date_finite"}, want: domain.ErrInvalidDate},
		{name: "null name", err: &pgconn.PgError{Code: "23502", ColumnName: "name"}, want: domain.ErrEmptyCategoryName},
		{name: "null category", err: &pgconn.PgError{Code: "23502", ColumnName: "category_id"}, want: application.ErrCategoryNotFound},
		{name: "null amount", err: &pgconn.PgError{Code: "23502", ColumnName: "amount_kopecks"}, want: domain.ErrInvalidAmount},
		{name: "null date", err: &pgconn.PgError{Code: "23502", ColumnName: "expense_date"}, want: domain.ErrInvalidDate},
		{name: "canceled", err: fmt.Errorf("query: %w", context.Canceled), want: context.Canceled},
		{name: "unknown PostgreSQL error", err: unknown, want: unknown},
		{name: "unknown constraint", err: unknownConstraint, want: unknownConstraint},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := translateError(test.err, test.notFound, test.foreignKey)
			if !errors.Is(err, test.want) {
				t.Fatalf("error = %v, want %v", err, test.want)
			}
		})
	}
}

// TestOpenInvalidConfiguration проверяет обязательность адреса БД и отсутствие пароля в ошибке разбора.
func TestOpenInvalidConfiguration(t *testing.T) {
	for _, databaseURL := range []string{"", " \n", "postgres://user:secret-password@%zz/database"} {
		pool, err := Open(t.Context(), databaseURL)
		if pool != nil {
			pool.Close()
			t.Fatal("expected no pool for invalid configuration")
		}
		if err == nil {
			t.Fatal("expected configuration error")
		}
		if strings.Contains(err.Error(), "secret-password") {
			t.Fatal("configuration error exposes password")
		}
	}
}
