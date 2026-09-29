package domain

import (
	"errors"
	"testing"
)

// TestCategoryValidate проверяет допустимые названия и отклонение пустых категорий.
func TestCategoryValidate(t *testing.T) {
	tests := []struct {
		name     string
		category Category
		wantErr  error
	}{
		{name: "new category", category: Category{Name: "Продукты"}},
		{name: "saved category", category: Category{ID: 1, Name: "Транспорт"}},
		{name: "surrounding spaces", category: Category{Name: "  Дом  "}},
		{name: "empty name", category: Category{}, wantErr: ErrEmptyCategoryName},
		{name: "whitespace", category: Category{Name: " \t\n\r "}, wantErr: ErrEmptyCategoryName},
		{name: "unicode whitespace", category: Category{Name: "\u00a0\u2003"}, wantErr: ErrEmptyCategoryName},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.category.Validate(); !errors.Is(err, test.wantErr) {
				t.Errorf("Validate() = %v, want %v", err, test.wantErr)
			}
		})
	}
}
