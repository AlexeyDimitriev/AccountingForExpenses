package domain

import "errors"

var (
	ErrEmptyCategoryName = errors.New("название категории не должно быть пустым")
	ErrInvalidCategoryID = errors.New("идентификатор категории должен быть положительным")
	ErrInvalidAmount     = errors.New("сумма расхода должна быть положительной")
	ErrInvalidDate       = errors.New("дата расхода должна быть задана")
)
