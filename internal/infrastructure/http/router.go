package httpapi

import (
	"net/http"

	"AccountingForExpenses/internal/application"
)

type handler struct {
	categories *application.CategoryService
	expenses   *application.ExpenseService
}

// NewRouter связывает REST-маршруты с прикладными сервисами.
func NewRouter(categories *application.CategoryService, expenses *application.ExpenseService) http.Handler {
	api := &handler{categories: categories, expenses: expenses}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/categories", api.categoryCollection)
	mux.HandleFunc("/api/categories/{id}", api.categoryItem)
	mux.HandleFunc("/api/expenses", api.expenseCollection)
	mux.HandleFunc("/api/expenses/{id}", api.expenseItem)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, &requestError{http.StatusNotFound, "not_found", "Маршрут не найден"})
	})
	return mux
}

// methodNotAllowed возвращает единый JSON-ответ и список допустимых методов.
func methodNotAllowed(w http.ResponseWriter, allowed string) {
	w.Header().Set("Allow", allowed)
	writeError(w, &requestError{http.StatusMethodNotAllowed, "method_not_allowed", "Метод не поддерживается"})
}
