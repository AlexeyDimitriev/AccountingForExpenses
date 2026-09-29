package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"time"
)

type responseWriter struct {
	http.ResponseWriter
	status int
	bytes  int
}

// WriteHeader запоминает конечный статус, не заменяя уже отправленный ответ.
func (writer *responseWriter) WriteHeader(status int) {
	if writer.status != 0 {
		return
	}
	if status >= 200 {
		writer.status = status
	}
	writer.ResponseWriter.WriteHeader(status)
}

// Write учитывает размер ответа и стандартный статус при записи без WriteHeader.
func (writer *responseWriter) Write(body []byte) (int, error) {
	if writer.status == 0 {
		writer.WriteHeader(http.StatusOK)
	}
	written, err := writer.ResponseWriter.Write(body)
	writer.bytes += written
	return written, err
}

// Unwrap позволяет стандартному ResponseController обращаться к исходному writer.
func (writer *responseWriter) Unwrap() http.ResponseWriter { return writer.ResponseWriter }

// WithRequestLogging ограничивает время работы запроса, обрабатывает панику и пишет итог в журнал.
func WithRequestLogging(next http.Handler, logger *slog.Logger, timeout time.Duration) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		writer := &responseWriter{ResponseWriter: w}
		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()
		defer func() {
			panicValue := recover()
			committed := writer.status != 0
			if panicValue != nil {
				logger.Error("Паника при обработке запроса", "method", r.Method, "path", r.URL.Path, "panic", panicValue)
				if writer.status == 0 {
					writeError(writer, &requestError{http.StatusInternalServerError, "internal_error", "Внутренняя ошибка сервера"})
				}
			}
			status := writer.status
			if status == 0 {
				status = http.StatusOK
			}
			level := slog.LevelInfo
			if status >= 500 {
				level = slog.LevelError
			}
			logger.Log(r.Context(), level, "HTTP-запрос", "method", r.Method, "path", r.URL.Path, "status", status, "bytes", writer.bytes, "duration", time.Since(started))
			if panicValue != nil && committed {
				panic(http.ErrAbortHandler)
			}
		}()
		next.ServeHTTP(writer, r.WithContext(ctx))
	})
}
