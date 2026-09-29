package httpapi

import (
	"net/http"
	"net/url"
	"strconv"
	"time"

	"AccountingForExpenses/internal/application"
	"AccountingForExpenses/internal/domain"
)

type expenseInput struct {
	CategoryID    int64  `json:"category_id"`
	AmountKopecks int64  `json:"amount_kopecks"`
	Date          string `json:"date"`
	Comment       string `json:"comment"`
}

type expenseResponse struct {
	ID            int64  `json:"id"`
	CategoryID    int64  `json:"category_id"`
	AmountKopecks int64  `json:"amount_kopecks"`
	Date          string `json:"date"`
	Comment       string `json:"comment"`
}

type expenseListResponse struct {
	Expenses     []expenseResponse `json:"expenses"`
	TotalKopecks int64             `json:"total_kopecks"`
}

// expense преобразует входные поля в доменную сущность, проверяя календарную дату.
func (input expenseInput) expense(id int64) (domain.Expense, error) {
	date, err := parseDate(input.Date)
	if err != nil {
		return domain.Expense{}, err
	}
	return domain.Expense{ID: id, CategoryID: input.CategoryID, AmountKopecks: input.AmountKopecks, Date: date, Comment: input.Comment}, nil
}

// expenseJSON формирует публичное представление расхода без времени суток в дате.
func expenseJSON(expense domain.Expense) expenseResponse {
	return expenseResponse{expense.ID, expense.CategoryID, expense.AmountKopecks, expense.Date.Format(time.DateOnly), expense.Comment}
}

// expenseCollection обрабатывает фильтрованный список расходов и создание записи.
func (api *handler) expenseCollection(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		filter, err := parseFilter(r.URL.RawQuery)
		if err != nil {
			writeError(w, err)
			return
		}
		list, err := api.expenses.List(r.Context(), filter)
		if err != nil {
			writeError(w, err)
			return
		}
		result := expenseListResponse{Expenses: make([]expenseResponse, 0, len(list.Expenses)), TotalKopecks: list.TotalKopecks}
		for _, expense := range list.Expenses {
			result.Expenses = append(result.Expenses, expenseJSON(expense))
		}
		writeJSON(w, http.StatusOK, result)
	case http.MethodPost:
		input, err := decodeJSON[expenseInput](w, r)
		if err != nil {
			writeError(w, err)
			return
		}
		expense, err := input.expense(0)
		if err != nil {
			writeError(w, err)
			return
		}
		expense, err = api.expenses.Create(r.Context(), expense)
		if err != nil {
			writeError(w, err)
			return
		}
		w.Header().Set("Location", "/api/expenses/"+strconv.FormatInt(expense.ID, 10))
		writeJSON(w, http.StatusCreated, expenseJSON(expense))
	default:
		methodNotAllowed(w, "GET, POST")
	}
}

// expenseItem обрабатывает чтение, полное обновление и удаление расхода.
func (api *handler) expenseItem(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPut && r.Method != http.MethodDelete {
		methodNotAllowed(w, "GET, PUT, DELETE")
		return
	}
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	switch r.Method {
	case http.MethodGet:
		expense, err := api.expenses.Get(r.Context(), id)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, expenseJSON(expense))
	case http.MethodPut:
		input, err := decodeJSON[expenseInput](w, r)
		if err != nil {
			writeError(w, err)
			return
		}
		expense, err := input.expense(id)
		if err != nil {
			writeError(w, err)
			return
		}
		expense, err = api.expenses.Update(r.Context(), expense)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, expenseJSON(expense))
	case http.MethodDelete:
		if err := api.expenses.Delete(r.Context(), id); err != nil {
			writeError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// parseFilter принимает единственные значения известных параметров и проверяет границы периода.
func parseFilter(rawQuery string) (application.ExpenseFilter, error) {
	filter := application.ExpenseFilter{}
	values, err := url.ParseQuery(rawQuery)
	if err != nil {
		return filter, badRequest("Некорректные параметры запроса")
	}
	for name, items := range values {
		if len(items) != 1 || items[0] == "" {
			return filter, badRequest("Параметр должен иметь одно непустое значение")
		}
		switch name {
		case "category_id":
			filter.CategoryID, err = parseID(items[0])
		case "date_from":
			filter.DateFrom, err = parseDate(items[0])
		case "date_to":
			filter.DateTo, err = parseDate(items[0])
		default:
			return filter, badRequest("Неизвестный параметр фильтра")
		}
		if err != nil {
			return filter, err
		}
	}
	return filter, filter.Validate()
}
