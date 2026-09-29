package main

import (
	"strings"
	"testing"
)

// TestRunWithoutDatabaseURL проверяет отказ команды при отсутствии обязательной настройки.
func TestRunWithoutDatabaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	err := run(t.Context())
	if err == nil || !strings.Contains(err.Error(), "DATABASE_URL") {
		t.Fatalf("expected DATABASE_URL error, got %v", err)
	}
}
