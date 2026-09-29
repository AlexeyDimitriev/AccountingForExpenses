package domain

import "strings"

type Category struct {
	ID   int64
	Name string
}

// Validate проверяет, что название категории содержит не только пробельные символы.
func (category Category) Validate() error {
	if strings.TrimSpace(category.Name) == "" {
		return ErrEmptyCategoryName
	}

	return nil
}
