package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"AccountingForExpenses/internal/infrastructure/logging"
)

// Проверяет структурированный журнал и отсутствие query-параметров в нём.
func TestRequestLogging(t *testing.T) {
	var output bytes.Buffer
	logger := logging.New(&output, "expenses-app", slog.LevelInfo)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := r.Context().Deadline(); !ok {
			t.Fatal("request has no deadline")
		}
		w.WriteHeader(201)
		w.WriteHeader(500)
		_, _ = w.Write([]byte("ok"))
	})
	api := WithRequestLogging(next, logger, time.Second)
	response := httptest.NewRecorder()
	started := time.Now()
	api.ServeHTTP(response, httptest.NewRequest("GET", "/path?secret=value", nil))
	elapsedMS := float64(time.Since(started)) / float64(time.Millisecond)
	var entry map[string]any
	if err := json.Unmarshal(output.Bytes(), &entry); err != nil {
		t.Fatal(err)
	}
	if entry["status"] != float64(201) || entry["bytes"] != float64(2) || entry["path"] != "/path" || strings.Contains(output.String(), "secret") {
		t.Fatal(output.String())
	}
	duration, ok := entry["duration_ms"].(float64)
	if !ok || duration < 0 || duration > elapsedMS {
		t.Fatalf("invalid duration_ms: %v (elapsed %v ms)", entry["duration_ms"], elapsedMS)
	}
	if entry["service.name"] != "expenses-app" || entry["message"] != "HTTP-запрос" || entry["method"] != "GET" {
		t.Fatal(output.String())
	}
	if _, exists := entry["duration"]; exists {
		t.Fatal("unexpected duration field")
	}
}

// TestRequestTimeout проверяет отмену контекста обработчика и JSON-ответ при истечении времени.
func TestRequestTimeout(t *testing.T) {
	var output bytes.Buffer
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done(); writeError(w, r.Context().Err()) })
	api := WithRequestLogging(next, slog.New(slog.NewJSONHandler(&output, nil)), time.Millisecond)
	response := request(t, api, "GET", "/slow", "", 504)
	if decodeResponse[errorResponse](t, response).Error.Code != "request_timeout" {
		t.Fatal(response.Body.String())
	}
	if !strings.Contains(output.String(), `"level":"ERROR"`) {
		t.Fatal(output.String())
	}
}

// TestPanicRecovery проверяет единый ответ при панике до отправки заголовков.
func TestPanicRecovery(t *testing.T) {
	var output bytes.Buffer
	api := WithRequestLogging(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("private failure") }), slog.New(slog.NewJSONHandler(&output, nil)), time.Second)
	response := request(t, api, "GET", "/panic", "", 500)
	if strings.Contains(response.Body.String(), "private failure") {
		t.Fatal(response.Body.String())
	}
	if !strings.Contains(output.String(), "private failure") {
		t.Fatal("panic missing from logs")
	}
}

// TestPanicAfterWrite проверяет обрыв ответа вместо дописывания JSON к частично отправленному телу.
func TestPanicAfterWrite(t *testing.T) {
	var output bytes.Buffer
	api := WithRequestLogging(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("partial")); panic("failure") }), slog.New(slog.NewJSONHandler(&output, nil)), time.Second)
	defer func() {
		if value := recover(); value != http.ErrAbortHandler {
			t.Fatalf("panic = %v", value)
		}
	}()
	api.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/panic", nil))
}

// TestResponseWriter проверяет доступ к исходному writer и стандартный статус пустого ответа.
func TestResponseWriter(t *testing.T) {
	underlying := httptest.NewRecorder()
	writer := &responseWriter{ResponseWriter: underlying}
	if writer.Unwrap() != underlying {
		t.Fatal("unexpected underlying writer")
	}
	var output bytes.Buffer
	api := WithRequestLogging(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}), slog.New(slog.NewJSONHandler(&output, nil)), time.Second)
	api.ServeHTTP(underlying, httptest.NewRequest("GET", "/empty", nil))
	if !strings.Contains(output.String(), `"status":200`) {
		t.Fatal(output.String())
	}
	response := httptest.NewRecorder()
	writeError(response, context.Canceled)
	if response.Code != 408 {
		t.Fatal(response.Code)
	}
}

func TestHealthRequestLogLevels(t *testing.T) {
	for _, path := range []string{"/healthz", "/readyz", "/api/categories"} {
		for _, status := range []int{200, 204, 400, 503} {
			for _, threshold := range []slog.Level{slog.LevelInfo, slog.LevelDebug} {
				var output bytes.Buffer
				next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(status) })
				api := WithRequestLogging(next, logging.New(&output, "expenses-app", threshold), time.Second)
				api.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", path, nil))
				want := "INFO"
				if path != "/api/categories" && status < 300 {
					want = "DEBUG"
				}
				if status >= 500 {
					want = "ERROR"
				}
				if want == "DEBUG" && threshold == slog.LevelInfo {
					if output.Len() != 0 {
						t.Fatalf("successful health check logged at INFO: %s", output.String())
					}
					continue
				}
				var entry map[string]any
				if err := json.Unmarshal(output.Bytes(), &entry); err != nil {
					t.Fatal(err)
				}
				if entry["level"] != want {
					t.Fatalf("%s status %d: got %v, want %s", path, status, entry["level"], want)
				}
			}
		}
	}
}
