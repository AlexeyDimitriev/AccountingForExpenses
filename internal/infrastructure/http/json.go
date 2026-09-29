package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strconv"
	"time"
)

const maxBodySize = 64 << 10

// decodeJSON читает единственный JSON-объект, ограничивая размер и запрещая неизвестные поля.
func decodeJSON[T any](w http.ResponseWriter, r *http.Request) (T, error) {
	var zero T
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return zero, &requestError{http.StatusUnsupportedMediaType, "unsupported_media_type", "Ожидается Content-Type: application/json"}
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var value *T
	if err := decoder.Decode(&value); err != nil {
		return zero, jsonError(err)
	}
	if value == nil {
		return zero, badRequest("Ожидается JSON-объект")
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err != nil {
			return zero, jsonError(err)
		}
		return zero, badRequest("Допускается только один JSON-объект")
	}
	return *value, nil
}

// jsonError отличает слишком большой запрос от некорректного JSON.
func jsonError(err error) error {
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		return &requestError{http.StatusRequestEntityTooLarge, "body_too_large", "Тело запроса превышает 64 КиБ"}
	}
	return badRequest("Некорректный JSON или неизвестные поля")
}

// writeJSON отправляет ответ с указанным HTTP-статусом.
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

// parseID принимает только положительный целочисленный идентификатор.
func parseID(value string) (int64, error) {
	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id <= 0 {
		return 0, badRequest("Идентификатор должен быть положительным целым числом")
	}
	return id, nil
}

// parseDate проверяет календарную дату в формате ГГГГ-ММ-ДД.
func parseDate(value string) (time.Time, error) {
	date, err := time.Parse(time.DateOnly, value)
	if err != nil || date.Year() < 1 || date.IsZero() {
		return time.Time{}, badRequest("Дата должна быть корректной и иметь формат ГГГГ-ММ-ДД")
	}
	return date, nil
}
