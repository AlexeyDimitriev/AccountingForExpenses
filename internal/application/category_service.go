package application

import (
	"context"
	"strings"

	"AccountingForExpenses/internal/domain"
)

type CategoryService struct {
	categories CategoryRepository
	expenses   ExpenseRepository
}

// NewCategoryService создаёт сервис с репозиториями категорий и расходов.
func NewCategoryService(categories CategoryRepository, expenses ExpenseRepository) *CategoryService {
	return &CategoryService{categories: categories, expenses: expenses}
}

// Create проверяет название и сохраняет новую категорию.
func (service *CategoryService) Create(ctx context.Context, name string) (domain.Category, error) {
	category := domain.Category{Name: strings.TrimSpace(name)}
	if err := category.Validate(); err != nil {
		return domain.Category{}, err
	}
	return service.categories.Create(ctx, category)
}

// Get возвращает категорию по положительному идентификатору.
func (service *CategoryService) Get(ctx context.Context, id int64) (domain.Category, error) {
	if id <= 0 {
		return domain.Category{}, ErrInvalidID
	}
	return service.categories.Get(ctx, id)
}

// List возвращает все категории.
func (service *CategoryService) List(ctx context.Context) ([]domain.Category, error) {
	return service.categories.List(ctx)
}

// Update проверяет данные и изменяет существующую категорию.
func (service *CategoryService) Update(ctx context.Context, category domain.Category) (domain.Category, error) {
	if category.ID <= 0 {
		return domain.Category{}, ErrInvalidID
	}
	category.Name = strings.TrimSpace(category.Name)
	if err := category.Validate(); err != nil {
		return domain.Category{}, err
	}
	return service.categories.Update(ctx, category)
}

// Delete удаляет категорию, если она существует и не содержит расходов.
func (service *CategoryService) Delete(ctx context.Context, id int64) error {
	if id <= 0 {
		return ErrInvalidID
	}
	if _, err := service.categories.Get(ctx, id); err != nil {
		return err
	}
	used, err := service.expenses.HasByCategory(ctx, id)
	if err != nil {
		return err
	}
	if used {
		return ErrCategoryInUse
	}
	return service.categories.Delete(ctx, id)
}
