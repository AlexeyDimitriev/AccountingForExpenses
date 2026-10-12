package logging

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"
	"time"
)

func TestJSONFields(t *testing.T) {
	for _, service := range []string{"expenses-app", "expenses-migrate"} {
		t.Run(service, func(t *testing.T) {
			var output bytes.Buffer
			logger := New(&output, service, slog.LevelInfo)
			logger.Error("Ошибка\nподключения", "error", errors.New("database unavailable"), "status", 503)
			if bytes.Count(output.Bytes(), []byte("\n")) != 1 {
				t.Fatalf("expected one JSON line: %s", &output)
			}
			var entry map[string]any
			if err := json.Unmarshal(output.Bytes(), &entry); err != nil {
				t.Fatal(err)
			}
			timestamp, ok := entry["timestamp"].(string)
			if !ok {
				t.Fatal("timestamp missing")
			}
			if _, err := time.Parse(time.RFC3339Nano, timestamp); err != nil {
				t.Fatal(err)
			}
			if entry["service.name"] != service || entry["level"] != "ERROR" || entry["message"] != "Ошибка\nподключения" || entry["error"] != "database unavailable" || entry["status"] != float64(503) {
				t.Fatal(output.String())
			}
			if _, exists := entry["time"]; exists {
				t.Fatal("unexpected time field")
			}
			if _, exists := entry["msg"]; exists {
				t.Fatal("unexpected msg field")
			}
		})
	}
}

func TestLevelFilter(t *testing.T) {
	var output bytes.Buffer
	logger := New(&output, "expenses-app", slog.LevelWarn)
	logger.Info("filtered")
	if output.Len() != 0 {
		t.Fatal("INFO must be filtered at WARN level")
	}
	logger.Warn("visible")
	if output.Len() == 0 {
		t.Fatal("WARN must be logged")
	}
}
