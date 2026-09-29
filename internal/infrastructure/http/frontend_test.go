package httpapi

import (
	"net/http/httptest"
	"strings"
	"testing"
)

// TestFrontend проверяет встроенные ресурсы, типы содержимого и совместимость с API.
func TestFrontend(t *testing.T) {
	handler := WithFrontend(testRouter())
	for _, test := range []struct{ path, contentType, fragment string }{
		{"/", "text/html", "Личный бюджет"},
		{"/assets/styles.css", "text/css", ":root"},
		{"/assets/app.mjs", "text/javascript", "initialize"},
		{"/assets/money.mjs", "text/javascript", "parseAmount"},
	} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest("GET", test.path, nil))
		if response.Code != 200 || !strings.HasPrefix(response.Header().Get("Content-Type"), test.contentType) || !strings.Contains(response.Body.String(), test.fragment) {
			t.Fatal(test.path, response)
		}
		response = httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest("HEAD", test.path, nil))
		if response.Code != 200 || response.Body.Len() != 0 {
			t.Fatal("invalid HEAD response", test.path)
		}
		request(t, handler, "POST", test.path, "", 405)
	}
	request(t, handler, "GET", "/api/categories", "", 200)
	for _, path := range []string{"/assets/", "/assets/missing.mjs", "/embed.go", "/money_test.mjs", "/unknown"} {
		request(t, handler, "GET", path, "", 404)
	}
}
