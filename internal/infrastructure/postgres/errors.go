package postgres

import (
	"errors"

	"AccountingForExpenses/internal/application"
	"AccountingForExpenses/internal/domain"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// translateError преобразует известные ошибки PostgreSQL, сохраняя остальные для диагностики.
func translateError(err, notFound, foreignKey error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return notFound
	}
	var databaseError *pgconn.PgError
	if !errors.As(err, &databaseError) {
		return err
	}
	switch databaseError.Code {
	case "23503":
		if databaseError.ConstraintName == "expenses_category_fk" {
			return foreignKey
		}
	case "23514":
		switch databaseError.ConstraintName {
		case "categories_name_not_empty":
			return domain.ErrEmptyCategoryName
		case "expenses_amount_positive":
			return domain.ErrInvalidAmount
		case "expenses_date_finite":
			return domain.ErrInvalidDate
		}
	case "23502":
		switch databaseError.ColumnName {
		case "name":
			return domain.ErrEmptyCategoryName
		case "category_id":
			return application.ErrCategoryNotFound
		case "amount_kopecks":
			return domain.ErrInvalidAmount
		case "expense_date":
			return domain.ErrInvalidDate
		}
	}
	return err
}
