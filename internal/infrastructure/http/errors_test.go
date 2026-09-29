package httpapi

import (
	"context"
	"errors"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"AccountingForExpenses/internal/application"
	"AccountingForExpenses/internal/domain"
)

// TestErrorResponses проверяет HTTP-статусы и отсутствие внутренних деталей в JSON-ошибках.
func TestErrorResponses(t *testing.T) {
	tests := []struct {
		err    error
		status int
		code   string
	}{
		{application.ErrCategoryNotFound, 404, "category_not_found"},
		{application.ErrExpenseNotFound, 404, "expense_not_found"},
		{application.ErrCategoryInUse, 409, "category_in_use"},
		{application.ErrTotalOverflow, 422, "total_overflow"},
		{application.ErrInvalidID, 400, "invalid_id"},
		{application.ErrInvalidPeriod, 400, "invalid_period"},
		{domain.ErrEmptyCategoryName, 400, "empty_category_name"},
		{domain.ErrInvalidCategoryID, 400, "invalid_category_id"},
		{domain.ErrInvalidAmount, 400, "invalid_amount"},
		{domain.ErrInvalidDate, 400, "invalid_date"},
		{errors.New("password=secret"), 500, "internal_error"},
	}
	for _, test := range tests {
		response := httptest.NewRecorder()
		writeError(response, fmt.Errorf("password=secret: %w", test.err))
		result := decodeResponse[errorResponse](t, response)
		if response.Code != test.status || result.Error.Code != test.code {
			t.Fatalf("status %d, response %+v", response.Code, result)
		}
		if strings.Contains(response.Body.String(), "secret") {
			t.Fatal("internal error details exposed")
		}
	}
}

type failingCategoryList struct{ application.CategoryRepository }

// List имитирует внутреннюю ошибку хранилища.
func (repo failingCategoryList) List(context.Context) ([]domain.Category, error) {
	return nil, errors.New("password=secret")
}

// TestStorageFailure проверяет обработку сбоя хранилища через настоящий HTTP-маршрут.
func TestStorageFailure(t *testing.T) {
	router := NewRouter(application.NewCategoryService(failingCategoryList{}, nil), nil)
	response := request(t, router, "GET", "/api/categories", "", 500)
	if strings.Contains(response.Body.String(), "secret") {
		t.Fatal("internal error details exposed")
	}
}

type contextCategoryList struct {
	application.CategoryRepository
	expected context.Context
	t        *testing.T
}

// List проверяет передачу исходного контекста запроса до репозитория.
func (repo contextCategoryList) List(ctx context.Context) ([]domain.Category, error) {
	if ctx != repo.expected {
		repo.t.Fatal("request context was replaced")
	}
	return nil, nil
}

// TestRequestContext проверяет передачу контекста и сериализацию пустого списка как массива.
func TestRequestContext(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	repo := contextCategoryList{expected: ctx, t: t}
	router := NewRouter(application.NewCategoryService(repo, nil), nil)
	req := httptest.NewRequest("GET", "/api/categories", nil).WithContext(ctx)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, req)
	if response.Code != 200 || strings.TrimSpace(response.Body.String()) != "[]" {
		t.Fatal(response)
	}
}

// TestTotalOverflowResponse проверяет отказ вместо выдачи переполненной итоговой суммы.
func TestTotalOverflowResponse(t *testing.T) {
	router := testRouter()
	request(t, router, "POST", "/api/categories", `{"name":"Дом"}`, 201)
	request(t, router, "POST", "/api/expenses", `{"category_id":1,"amount_kopecks":9223372036854775807,"date":"2026-09-29"}`, 201)
	request(t, router, "POST", "/api/expenses", `{"category_id":1,"amount_kopecks":1,"date":"2026-09-29"}`, 201)
	response := request(t, router, "GET", "/api/expenses", "", 422)
	if decodeResponse[errorResponse](t, response).Error.Code != "total_overflow" {
		t.Fatal(response.Body.String())
	}
}
