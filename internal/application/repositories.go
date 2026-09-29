package application

import (
	"context"

	"github.com/AlexeyDimitriev/AccountingForExpenses/internal/domain"
)

// CategoryRepository хранит категории; отсутствующие записи возвращают ErrCategoryNotFound.
// Delete должен атомарно запрещать удаление используемой категории с ErrCategoryInUse.
type CategoryRepository interface {
	// Create сохраняет категорию и возвращает её с назначенным идентификатором.
	Create(context.Context, domain.Category) (domain.Category, error)
	// Get возвращает категорию по идентификатору.
	Get(context.Context, int64) (domain.Category, error)
	// List возвращает категории по возрастанию идентификатора.
	List(context.Context) ([]domain.Category, error)
	// Update заменяет данные существующей категории.
	Update(context.Context, domain.Category) (domain.Category, error)
	// Delete удаляет существующую категорию без расходов.
	Delete(context.Context, int64) error
}

// ExpenseRepository хранит расходы; отсутствующие записи возвращают ErrExpenseNotFound.
// Create и Update должны проверять связь с категорией и возвращать ErrCategoryNotFound,
// если категория удалена между проверкой сервиса и сохранением.
type ExpenseRepository interface {
	// Create сохраняет расход и возвращает его с назначенным идентификатором.
	Create(context.Context, domain.Expense) (domain.Expense, error)
	// Get возвращает расход по идентификатору.
	Get(context.Context, int64) (domain.Expense, error)
	// List применяет все фильтры и сортирует расходы по дате и идентификатору по убыванию.
	List(context.Context, ExpenseFilter) ([]domain.Expense, error)
	// Update заменяет данные существующего расхода.
	Update(context.Context, domain.Expense) (domain.Expense, error)
	// Delete удаляет существующий расход.
	Delete(context.Context, int64) error
	// HasByCategory сообщает, есть ли расходы у категории.
	HasByCategory(context.Context, int64) (bool, error)
}
