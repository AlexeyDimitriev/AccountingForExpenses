package httpapi

import (
	"bytes"
	"net/http"
	"time"

	"AccountingForExpenses/web"
)

// WithFrontend раздаёт встроенные страницы и ресурсы, передавая остальные маршруты API.
func WithFrontend(next http.Handler) http.Handler {
	assets := map[string]struct{ file, contentType string }{
		"/":                  {"index.html", "text/html; charset=utf-8"},
		"/assets/styles.css": {"assets/styles.css", "text/css; charset=utf-8"},
		"/assets/app.mjs":    {"assets/app.mjs", "text/javascript; charset=utf-8"},
		"/assets/money.mjs":  {"assets/money.mjs", "text/javascript; charset=utf-8"},
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asset, exists := assets[r.URL.Path]
		if !exists {
			next.ServeHTTP(w, r)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			methodNotAllowed(w, "GET, HEAD")
			return
		}
		content, err := web.Files.ReadFile(asset.file)
		if err != nil {
			writeError(w, err)
			return
		}
		w.Header().Set("Content-Type", asset.contentType)
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		http.ServeContent(w, r, asset.file, time.Time{}, bytes.NewReader(content))
	})
}
