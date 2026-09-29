package httpapi

import (
	"net/http"
	"strconv"

	"AccountingForExpenses/internal/domain"
)

type categoryInput struct {
	Name string `json:"name"`
}
type categoryResponse struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// categoryCollection обрабатывает получение списка и создание категорий.
func (api *handler) categoryCollection(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		categories, err := api.categories.List(r.Context())
		if err != nil {
			writeError(w, err)
			return
		}
		result := make([]categoryResponse, 0, len(categories))
		for _, category := range categories {
			result = append(result, categoryResponse{category.ID, category.Name})
		}
		writeJSON(w, http.StatusOK, result)
	case http.MethodPost:
		input, err := decodeJSON[categoryInput](w, r)
		if err != nil {
			writeError(w, err)
			return
		}
		category, err := api.categories.Create(r.Context(), input.Name)
		if err != nil {
			writeError(w, err)
			return
		}
		w.Header().Set("Location", "/api/categories/"+strconv.FormatInt(category.ID, 10))
		writeJSON(w, http.StatusCreated, categoryResponse{category.ID, category.Name})
	default:
		methodNotAllowed(w, "GET, POST")
	}
}

// categoryItem обрабатывает чтение, полное обновление и удаление категории.
func (api *handler) categoryItem(w http.ResponseWriter, r *http.Request) {
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
		category, err := api.categories.Get(r.Context(), id)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, categoryResponse{category.ID, category.Name})
	case http.MethodPut:
		input, err := decodeJSON[categoryInput](w, r)
		if err != nil {
			writeError(w, err)
			return
		}
		category, err := api.categories.Update(r.Context(), domain.Category{ID: id, Name: input.Name})
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, categoryResponse{category.ID, category.Name})
	case http.MethodDelete:
		if err := api.categories.Delete(r.Context(), id); err != nil {
			writeError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
