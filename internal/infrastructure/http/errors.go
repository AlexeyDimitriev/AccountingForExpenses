package httpapi

import (
	"errors"
	"net/http"

	"AccountingForExpenses/internal/application"
	"AccountingForExpenses/internal/domain"
)

type requestError struct {
	status  int
	code    string
	message string
}

type errorResponse struct {
	Error errorDetails `json:"error"`
}
type errorDetails struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Error возвращает понятное описание ошибки запроса.
func (err *requestError) Error() string { return err.message }

// badRequest создаёт ошибку некорректного клиентского запроса.
func badRequest(message string) error {
	return &requestError{http.StatusBadRequest, "invalid_request", message}
}

// writeError преобразует известные ошибки в HTTP-ответ, скрывая внутренние детали остальных ошибок.
func writeError(w http.ResponseWriter, err error) {
	var invalid *requestError
	if errors.As(err, &invalid) {
		writeJSON(w, invalid.status, errorResponse{errorDetails{invalid.code, invalid.message}})
		return
	}
	mappings := []struct {
		target error
		status int
		code   string
	}{
		{application.ErrCategoryNotFound, http.StatusNotFound, "category_not_found"},
		{application.ErrExpenseNotFound, http.StatusNotFound, "expense_not_found"},
		{application.ErrCategoryInUse, http.StatusConflict, "category_in_use"},
		{application.ErrTotalOverflow, http.StatusUnprocessableEntity, "total_overflow"},
		{application.ErrInvalidID, http.StatusBadRequest, "invalid_id"},
		{application.ErrInvalidPeriod, http.StatusBadRequest, "invalid_period"},
		{domain.ErrEmptyCategoryName, http.StatusBadRequest, "empty_category_name"},
		{domain.ErrInvalidCategoryID, http.StatusBadRequest, "invalid_category_id"},
		{domain.ErrInvalidAmount, http.StatusBadRequest, "invalid_amount"},
		{domain.ErrInvalidDate, http.StatusBadRequest, "invalid_date"},
	}
	for _, mapping := range mappings {
		if errors.Is(err, mapping.target) {
			writeJSON(w, mapping.status, errorResponse{errorDetails{mapping.code, mapping.target.Error()}})
			return
		}
	}
	writeJSON(w, http.StatusInternalServerError, errorResponse{errorDetails{"internal_error", "Внутренняя ошибка сервера"}})
}
