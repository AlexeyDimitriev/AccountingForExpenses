package httpapi

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"AccountingForExpenses/internal/application"
	"AccountingForExpenses/internal/domain"
	"AccountingForExpenses/internal/infrastructure/logging"
)

// testRouter создаёт обработчик с настоящими сервисами и изолированными репозиториями в памяти.
func testRouter() http.Handler {
	categories := &categoryMemory{items: make(map[int64]domain.Category)}
	expenses := &expenseMemory{items: make(map[int64]domain.Expense)}
	return NewRouter(application.NewCategoryService(categories, expenses), application.NewExpenseService(expenses, categories))
}

// request выполняет HTTP-запрос и проверяет статус и общий формат ответа.
func request(t *testing.T, router http.Handler, method, path, body string, status int) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, req)
	if response.Code != status {
		t.Fatalf("%s %s: status %d, want %d; body %s", method, path, response.Code, status, response.Body.String())
	}
	if status == http.StatusNoContent {
		if response.Body.Len() != 0 {
			t.Fatal("204 response has body")
		}
	} else {
		if response.Header().Get("Content-Type") != "application/json; charset=utf-8" {
			t.Fatal(response.Header())
		}
		if !json.Valid(response.Body.Bytes()) {
			t.Fatal("invalid JSON response")
		}
	}
	if status >= 400 {
		var result errorResponse
		if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if result.Error.Code == "" || result.Error.Message == "" {
			t.Fatal("incomplete error response")
		}
	}
	return response
}

// decodeResponse проверяет JSON и возвращает типизированный ответ обработчика.
func decodeResponse[T any](t *testing.T, response *httptest.ResponseRecorder) T {
	t.Helper()
	var result T
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	return result
}

// TestCRUD проверяет оба ресурса через HTTP, включая конфликт удаления используемой категории.
func TestCRUD(t *testing.T) {
	router := testRouter()
	empty := request(t, router, "GET", "/api/categories", "", 200)
	if strings.TrimSpace(empty.Body.String()) != "[]" {
		t.Fatal(empty.Body.String())
	}
	created := request(t, router, "POST", "/api/categories", `{"name":"  Еда  "}`, 201)
	category := decodeResponse[categoryResponse](t, created)
	if category.ID != 1 || category.Name != "Еда" || created.Header().Get("Location") != "/api/categories/1" {
		t.Fatal(created)
	}
	request(t, router, "POST", "/api/categories", `{"name":"Дом"}`, 201)
	got := decodeResponse[categoryResponse](t, request(t, router, "GET", "/api/categories/1", "", 200))
	if got != category {
		t.Fatal(got)
	}
	updated := decodeResponse[categoryResponse](t, request(t, router, "PUT", "/api/categories/1", `{"name":"Продукты"}`, 200))
	if updated.Name != "Продукты" || updated.ID != 1 {
		t.Fatal(updated)
	}
	list := decodeResponse[[]categoryResponse](t, request(t, router, "GET", "/api/categories", "", 200))
	if len(list) != 2 || list[0] != updated {
		t.Fatal(list)
	}
	created = request(t, router, "POST", "/api/expenses", `{"category_id":1,"amount_kopecks":12345,"date":"2026-09-29","comment":"Обед"}`, 201)
	expense := decodeResponse[expenseResponse](t, created)
	if expense.ID != 1 || expense.AmountKopecks != 12345 || expense.Date != "2026-09-29" || expense.Comment != "Обед" || created.Header().Get("Location") != "/api/expenses/1" {
		t.Fatal(expense)
	}
	gotExpense := decodeResponse[expenseResponse](t, request(t, router, "GET", "/api/expenses/1", "", 200))
	if gotExpense != expense {
		t.Fatal(gotExpense)
	}
	request(t, router, "DELETE", "/api/categories/1", "", 409)
	changed := decodeResponse[expenseResponse](t, request(t, router, "PUT", "/api/expenses/1", `{"category_id":2,"amount_kopecks":1,"date":"2026-09-30"}`, 200))
	if changed.CategoryID != 2 || changed.AmountKopecks != 1 || changed.Date != "2026-09-30" || changed.Comment != "" {
		t.Fatal(changed)
	}
	request(t, router, "DELETE", "/api/categories/1", "", 204)
	request(t, router, "GET", "/api/categories/1", "", 404)
	request(t, router, "PUT", "/api/categories/1", `{"name":"Нет"}`, 404)
	request(t, router, "DELETE", "/api/categories/1", "", 404)
	request(t, router, "DELETE", "/api/expenses/1", "", 204)
	request(t, router, "GET", "/api/expenses/1", "", 404)
	request(t, router, "PUT", "/api/expenses/1", `{"category_id":2,"amount_kopecks":1,"date":"2026-09-30"}`, 404)
	request(t, router, "DELETE", "/api/expenses/1", "", 404)
	request(t, router, "DELETE", "/api/categories/2", "", 204)
	emptyExpenses := decodeResponse[expenseListResponse](t, request(t, router, "GET", "/api/expenses", "", 200))
	if emptyExpenses.Expenses == nil || len(emptyExpenses.Expenses) != 0 || emptyExpenses.TotalKopecks != 0 {
		t.Fatal(emptyExpenses)
	}
}

// TestExpenseFilters проверяет передачу всех фильтров в сервис и точный итог ответа.
func TestExpenseFilters(t *testing.T) {
	router := testRouter()
	request(t, router, "POST", "/api/categories", `{"name":"Дом"}`, 201)
	request(t, router, "POST", "/api/categories", `{"name":"Еда"}`, 201)
	for _, body := range []string{
		`{"category_id":1,"amount_kopecks":101,"date":"2026-09-28"}`,
		`{"category_id":1,"amount_kopecks":202,"date":"2026-09-29"}`,
		`{"category_id":2,"amount_kopecks":303,"date":"2026-09-29"}`,
	} {
		request(t, router, "POST", "/api/expenses", body, 201)
	}
	tests := []struct {
		query string
		count int
		total int64
	}{
		{"", 3, 606},
		{"?category_id=1", 2, 303},
		{"?date_from=2026-09-29", 2, 505},
		{"?date_to=2026-09-28", 1, 101},
		{"?category_id=1&date_from=2026-09-29&date_to=2026-09-29", 1, 202},
		{"?category_id=999", 0, 0},
	}
	for _, test := range tests {
		result := decodeResponse[expenseListResponse](t, request(t, router, "GET", "/api/expenses"+test.query, "", 200))
		if len(result.Expenses) != test.count || result.TotalKopecks != test.total {
			t.Fatal(result)
		}
	}
}

// TestInvalidRequests проверяет JSON, идентификаторы, календарные даты, суммы и параметры фильтра.
func TestInvalidRequests(t *testing.T) {
	router := testRouter()
	request(t, router, "POST", "/api/categories", `{"name":"Дом"}`, 201)
	tests := []struct {
		method, path, body string
		status             int
	}{
		{"POST", "/api/categories", "", 400},
		{"POST", "/api/categories", "null", 400},
		{"POST", "/api/categories", "[]", 400},
		{"POST", "/api/categories", `{"name":`, 400},
		{"POST", "/api/categories", `{"name":"Дом","unknown":1}`, 400},
		{"POST", "/api/categories", `{"name":"Дом"} {}`, 400},
		{"POST", "/api/categories", `{"name":42}`, 400},
		{"POST", "/api/categories", `{"name":"  "}`, 400},
		{"PUT", "/api/categories/1", `{}`, 400},
		{"POST", "/api/expenses", `{"category_id":1,"amount_kopecks":1,"date":"2026-02-29"}`, 400},
		{"POST", "/api/expenses", `{"category_id":1,"amount_kopecks":1,"date":"2026-09-29T00:00:00Z"}`, 400},
		{"POST", "/api/expenses", `{"category_id":1,"amount_kopecks":1}`, 400},
		{"POST", "/api/expenses", `{"category_id":1,"amount_kopecks":1.5,"date":"2026-09-29"}`, 400},
		{"POST", "/api/expenses", `{"category_id":1,"amount_kopecks":9223372036854775808,"date":"2026-09-29"}`, 400},
		{"POST", "/api/expenses", `{"category_id":1,"amount_kopecks":0,"date":"2026-09-29"}`, 400},
		{"POST", "/api/expenses", `{"category_id":0,"amount_kopecks":1,"date":"2026-09-29"}`, 400},
		{"POST", "/api/expenses", `{"category_id":999,"amount_kopecks":1,"date":"2026-09-29"}`, 404},
		{"PUT", "/api/expenses/1", `{"id":2}`, 400},
		{"PUT", "/api/expenses/1", `{"date":"bad"}`, 400},
		{"GET", "/api/expenses?date_from=2026-09-30&date_to=2026-09-29", "", 400},
		{"GET", "/api/expenses?category_id=0", "", 400},
		{"GET", "/api/expenses?category_id=1&category_id=2", "", 400},
		{"GET", "/api/expenses?date_from=", "", 400},
		{"GET", "/api/expenses?date_to=2026-02-30", "", 400},
		{"GET", "/api/expenses?unknown=1", "", 400},
		{"GET", "/api/expenses?date_from=%zz", "", 400},
		{"GET", "/unknown", "", 404},
	}
	for _, test := range tests {
		request(t, router, test.method, test.path, test.body, test.status)
	}
	for _, resource := range []string{"categories", "expenses"} {
		for _, id := range []string{"0", "-1", "abc", "9223372036854775808"} {
			for _, method := range []string{"GET", "PUT", "DELETE"} {
				request(t, router, method, "/api/"+resource+"/"+id, `{}`, 400)
			}
		}
		for _, path := range []string{"/api/" + resource, "/api/" + resource + "/1"} {
			response := request(t, router, "PATCH", path, `{}`, 405)
			if response.Header().Get("Allow") == "" {
				t.Fatal("missing Allow")
			}
		}
	}
}

// TestBodyLimitsAndContentType проверяет ограничение размера, включая пробелы после JSON.
func TestBodyLimitsAndContentType(t *testing.T) {
	router := testRouter()
	for _, body := range []string{`{"name":"` + strings.Repeat("x", maxBodySize) + `"}`, `{"name":"Дом"}` + strings.Repeat(" ", maxBodySize)} {
		request(t, router, "POST", "/api/categories", body, 413)
	}
	for _, contentType := range []string{"", "text/plain", "application/json; invalid"} {
		req := httptest.NewRequest("POST", "/api/categories", strings.NewReader(`{"name":"Дом"}`))
		req.Header.Set("Content-Type", contentType)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, req)
		if response.Code != 415 {
			t.Fatal(response.Code)
		}
	}
	request(t, router, "GET", "/api/categories", "", 200)
}

func TestMutationEvents(t *testing.T) {
	var output bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(logging.New(&output, "expenses-app", slog.LevelInfo))
	t.Cleanup(func() { slog.SetDefault(previous) })
	router := testRouter()
	tests := []struct {
		method, path, body string
		status             int
		event              string
		fields             map[string]any
	}{
		{"POST", "/api/categories", `{"name":"  Продукты  "}`, 201, "category.created", map[string]any{"category_id": float64(1), "category_name": "Продукты"}},
		{"PUT", "/api/categories/1", `{"name":"  Еда  "}`, 200, "category.updated", map[string]any{"category_id": float64(1), "old_name": "Продукты", "new_name": "Еда"}},
		{"POST", "/api/categories", `{"name":"Дом"}`, 201, "category.created", map[string]any{"category_id": float64(2), "category_name": "Дом"}},
		{"POST", "/api/expenses", `{"category_id":1,"amount_kopecks":987654,"date":"2026-10-12","comment":"private-comment"}`, 201, "expense.created", map[string]any{"expense_id": float64(1), "category_id": float64(1), "category_name": "Еда", "amount_kopecks": float64(987654), "date": "2026-10-12"}},
		{"DELETE", "/api/categories/1", "", 409, "", nil},
		{"PUT", "/api/expenses/1", `{"category_id":2,"amount_kopecks":12500,"date":"2026-10-13","comment":"private-updated-comment"}`, 200, "expense.updated", map[string]any{"expense_id": float64(1), "old_category_id": float64(1), "new_category_id": float64(2), "old_category_name": "Еда", "new_category_name": "Дом", "old_amount_kopecks": float64(987654), "new_amount_kopecks": float64(12500), "old_date": "2026-10-12", "new_date": "2026-10-13", "comment_changed": true}},
		{"PUT", "/api/expenses/1", `{"category_id":2,"amount_kopecks":12600,"date":"2026-10-13","comment":"private-updated-comment"}`, 200, "expense.updated", map[string]any{"expense_id": float64(1), "old_amount_kopecks": float64(12500), "new_amount_kopecks": float64(12600)}},
		{"POST", "/api/categories", `{"name":""}`, 400, "", nil},
		{"POST", "/api/expenses", `{"category_id":999,"amount_kopecks":1,"date":"2026-10-12"}`, 404, "", nil},
		{"PUT", "/api/expenses/1", `{"category_id":999,"amount_kopecks":1,"date":"2026-10-12"}`, 404, "", nil},
		{"DELETE", "/api/expenses/1", "", 204, "expense.deleted", map[string]any{"expense_id": float64(1), "category_id": float64(2), "category_name": "Дом", "amount_kopecks": float64(12600), "date": "2026-10-13"}},
		{"PUT", "/api/expenses/1", `{"category_id":1,"amount_kopecks":1,"date":"2026-10-12"}`, 404, "", nil},
		{"DELETE", "/api/expenses/1", "", 404, "", nil},
		{"DELETE", "/api/categories/1", "", 204, "category.deleted", map[string]any{"category_id": float64(1), "category_name": "Еда"}},
		{"PUT", "/api/categories/1", `{"name":"Нет"}`, 404, "", nil},
		{"DELETE", "/api/categories/1", "", 404, "", nil},
	}
	for _, test := range tests {
		output.Reset()
		request(t, router, test.method, test.path, test.body, test.status)
		if test.event == "" {
			if output.Len() != 0 {
				t.Fatalf("failed operation logged success: %s", output.String())
			}
			continue
		}
		var entry map[string]any
		if err := json.Unmarshal(output.Bytes(), &entry); err != nil {
			t.Fatal(err)
		}
		if entry["event"] != test.event || entry["message"] == "" || entry["level"] != "INFO" || entry["service.name"] != "expenses-app" {
			t.Fatal(output.String())
		}
		for key, want := range test.fields {
			if entry[key] != want {
				t.Fatalf("%s field %s: got %v, want %v", test.event, key, entry[key], want)
			}
		}
		if len(entry) != len(test.fields)+5 {
			t.Fatalf("unexpected event fields: %s", output.String())
		}
		if strings.Contains(output.String(), "private-") {
			t.Fatal(output.String())
		}
	}
}
