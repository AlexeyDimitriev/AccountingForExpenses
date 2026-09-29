package httpapi

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"
)

// TestHealth проверяет независимость liveness от БД и отказ readiness при сбое зависимости.
func TestHealth(t *testing.T) {
	calls := 0
	unavailable := false
	api := WithHealth(testRouter(), func(ctx context.Context) error {
		calls++
		if _, ok := ctx.Deadline(); !ok {
			t.Fatal("health check has no timeout")
		}
		if unavailable {
			return errors.New("database unavailable")
		}
		return nil
	}, time.Second)
	response := request(t, api, "GET", "/healthz", "", 200)
	if calls != 0 || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("liveness consulted database or was cacheable")
	}
	request(t, api, "GET", "/readyz", "", 200)
	if calls != 1 {
		t.Fatal(calls)
	}
	unavailable = true
	request(t, api, "GET", "/healthz", "", 200)
	request(t, api, "GET", "/readyz", "", 503)
	request(t, api, "POST", "/healthz", "", 405)
	request(t, api, "DELETE", "/readyz", "", 405)
	request(t, api, "GET", "/api/categories", "", 200)
}

// TestReadinessTimeout проверяет отмену зависшего обращения к зависимости.
func TestReadinessTimeout(t *testing.T) {
	api := WithHealth(http.NotFoundHandler(), func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	}, time.Millisecond)
	request(t, api, "GET", "/readyz", "", 503)
}
