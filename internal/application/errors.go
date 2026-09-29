package application

import "errors"

var (
	ErrInvalidID        = errors.New("идентификатор должен быть положительным")
	ErrCategoryNotFound = errors.New("категория не найдена")
	ErrExpenseNotFound  = errors.New("расход не найден")
	ErrCategoryInUse    = errors.New("категория содержит расходы")
	ErrInvalidPeriod    = errors.New("начало периода не может быть позже окончания")
	ErrTotalOverflow    = errors.New("итоговая сумма превышает допустимое значение")
)
