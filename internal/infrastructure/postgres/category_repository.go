package postgres

import (
	"context"

	"AccountingForExpenses/internal/application"
	"AccountingForExpenses/internal/domain"

	"github.com/jackc/pgx/v5/pgxpool"
)

type CategoryRepository struct{ pool *pgxpool.Pool }

var _ application.CategoryRepository = (*CategoryRepository)(nil)

// NewCategoryRepository создаёт репозиторий категорий с общим пулом соединений.
func NewCategoryRepository(pool *pgxpool.Pool) *CategoryRepository {
	return &CategoryRepository{pool: pool}
}

// Create сохраняет категорию и возвращает назначенный базой идентификатор.
func (repo *CategoryRepository) Create(ctx context.Context, category domain.Category) (domain.Category, error) {
	err := repo.pool.QueryRow(ctx, `INSERT INTO categories (name) VALUES ($1) RETURNING id, name`, category.Name).Scan(&category.ID, &category.Name)
	if err != nil {
		return domain.Category{}, translateError(err, application.ErrCategoryNotFound, application.ErrCategoryInUse)
	}
	return category, nil
}

// Get возвращает категорию по идентификатору или ошибку отсутствующей записи.
func (repo *CategoryRepository) Get(ctx context.Context, id int64) (domain.Category, error) {
	var category domain.Category
	err := repo.pool.QueryRow(ctx, `SELECT id, name FROM categories WHERE id = $1`, id).Scan(&category.ID, &category.Name)
	if err != nil {
		return domain.Category{}, translateError(err, application.ErrCategoryNotFound, application.ErrCategoryInUse)
	}
	return category, nil
}

// List возвращает категории по возрастанию идентификатора.
func (repo *CategoryRepository) List(ctx context.Context) ([]domain.Category, error) {
	rows, err := repo.pool.Query(ctx, `SELECT id, name FROM categories ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	categories := []domain.Category{}
	for rows.Next() {
		var category domain.Category
		if err := rows.Scan(&category.ID, &category.Name); err != nil {
			return nil, err
		}
		categories = append(categories, category)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return categories, nil
}

// Update меняет название существующей категории.
func (repo *CategoryRepository) Update(ctx context.Context, category domain.Category) (domain.Category, error) {
	err := repo.pool.QueryRow(ctx, `UPDATE categories SET name = $2 WHERE id = $1 RETURNING id, name`, category.ID, category.Name).Scan(&category.ID, &category.Name)
	if err != nil {
		return domain.Category{}, translateError(err, application.ErrCategoryNotFound, application.ErrCategoryInUse)
	}
	return category, nil
}

// Delete удаляет категорию; внешний ключ запрещает удаление категории с расходами.
func (repo *CategoryRepository) Delete(ctx context.Context, id int64) error {
	result, err := repo.pool.Exec(ctx, `DELETE FROM categories WHERE id = $1`, id)
	if err != nil {
		return translateError(err, application.ErrCategoryNotFound, application.ErrCategoryInUse)
	}
	if result.RowsAffected() == 0 {
		return application.ErrCategoryNotFound
	}
	return nil
}
