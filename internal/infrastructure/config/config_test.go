package config

import (
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"
)

// cleanEnvironment изолирует настройки теста от переменных запускающего процесса.
func cleanEnvironment(t *testing.T) {
	t.Helper()
	for _, name := range []string{"DATABASE_URL", "HTTP_ADDR", "HTTP_READ_HEADER_TIMEOUT", "HTTP_READ_TIMEOUT", "HTTP_WRITE_TIMEOUT", "HTTP_IDLE_TIMEOUT", "HTTP_REQUEST_TIMEOUT", "HTTP_SHUTDOWN_TIMEOUT", "DB_CONNECT_TIMEOUT", "HEALTH_CHECK_TIMEOUT", "LOG_LEVEL"} {
		t.Setenv(name, "")
		if err := os.Unsetenv(name); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("DATABASE_URL", "postgres://localhost/expenses")
}

// TestDefaults проверяет минимальную конфигурацию и значения таймаутов по умолчанию.
func TestDefaults(t *testing.T) {
	cleanEnvironment(t)
	settings, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if settings.HTTPAddr != ":8080" || settings.DatabaseURL != "postgres://localhost/expenses" || settings.LogLevel != slog.LevelInfo {
		t.Fatal(settings)
	}
	if settings.ReadHeaderTimeout != 5*time.Second || settings.ReadTimeout != 10*time.Second || settings.WriteTimeout != 15*time.Second || settings.IdleTimeout != time.Minute || settings.RequestTimeout != 10*time.Second || settings.ShutdownTimeout != 10*time.Second || settings.ConnectTimeout != 5*time.Second || settings.HealthTimeout != 2*time.Second {
		t.Fatal(settings)
	}
}

// TestOverrides проверяет применение настроек из окружения.
func TestOverrides(t *testing.T) {
	cleanEnvironment(t)
	t.Setenv("HTTP_ADDR", "127.0.0.1:9000")
	t.Setenv("HTTP_READ_HEADER_TIMEOUT", "1s")
	t.Setenv("HTTP_READ_TIMEOUT", "2s")
	t.Setenv("HTTP_WRITE_TIMEOUT", "4s")
	t.Setenv("HTTP_IDLE_TIMEOUT", "5s")
	t.Setenv("HTTP_REQUEST_TIMEOUT", "3s")
	t.Setenv("HTTP_SHUTDOWN_TIMEOUT", "6s")
	t.Setenv("DB_CONNECT_TIMEOUT", "7s")
	t.Setenv("HEALTH_CHECK_TIMEOUT", "500ms")
	t.Setenv("LOG_LEVEL", "DEBUG")
	settings, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if settings.HTTPAddr != "127.0.0.1:9000" || settings.ReadHeaderTimeout != time.Second || settings.ReadTimeout != 2*time.Second || settings.WriteTimeout != 4*time.Second || settings.IdleTimeout != 5*time.Second || settings.RequestTimeout != 3*time.Second || settings.ShutdownTimeout != 6*time.Second || settings.ConnectTimeout != 7*time.Second || settings.HealthTimeout != 500*time.Millisecond || settings.LogLevel != slog.LevelDebug {
		t.Fatal(settings)
	}
}

// TestInvalidConfiguration проверяет отказ при пропущенных и некорректных настройках без раскрытия адреса БД.
func TestInvalidConfiguration(t *testing.T) {
	tests := []struct{ name, value string }{
		{"DATABASE_URL", " "}, {"HTTP_ADDR", "localhost"}, {"HTTP_ADDR", ":70000"}, {"HTTP_ADDR", ":abc"}, {"HTTP_ADDR", ""},
		{"HTTP_READ_HEADER_TIMEOUT", "11s"}, {"HTTP_WRITE_TIMEOUT", "10s"}, {"LOG_LEVEL", "verbose"},
	}
	for _, name := range []string{"HTTP_READ_HEADER_TIMEOUT", "HTTP_READ_TIMEOUT", "HTTP_WRITE_TIMEOUT", "HTTP_IDLE_TIMEOUT", "HTTP_REQUEST_TIMEOUT", "HTTP_SHUTDOWN_TIMEOUT", "DB_CONNECT_TIMEOUT", "HEALTH_CHECK_TIMEOUT"} {
		for _, value := range []string{"", "bad", "0s", "-1s"} {
			tests = append(tests, struct{ name, value string }{name, value})
		}
	}
	for _, test := range tests {
		t.Run(test.name+"="+test.value, func(t *testing.T) {
			cleanEnvironment(t)
			t.Setenv(test.name, test.value)
			_, err := Load()
			if err == nil || !strings.Contains(err.Error(), test.name) {
				t.Fatalf("expected %s error, got %v", test.name, err)
			}
		})
	}
}
