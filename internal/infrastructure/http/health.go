package httpapi

import (
	"context"
	"net/http"
	"time"
)

// WithHealth добавляет проверки процесса и готовности зависимостей к существующему API.
func WithHealth(next http.Handler, checkReady func(context.Context) error, timeout time.Duration) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/healthz" && r.URL.Path != "/readyz" {
			next.ServeHTTP(w, r)
			return
		}
		if r.Method != http.MethodGet {
			methodNotAllowed(w, "GET")
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		if r.URL.Path == "/readyz" {
			ctx, cancel := context.WithTimeout(r.Context(), timeout)
			defer cancel()
			if err := checkReady(ctx); err != nil {
				writeError(w, &requestError{http.StatusServiceUnavailable, "not_ready", "Сервис не готов к обработке запросов"})
				return
			}
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
}
