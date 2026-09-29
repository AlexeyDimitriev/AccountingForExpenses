package config

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	DatabaseURL       string
	HTTPAddr          string
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	RequestTimeout    time.Duration
	ShutdownTimeout   time.Duration
	ConnectTimeout    time.Duration
	HealthTimeout     time.Duration
	LogLevel          slog.Level
}

// Load читает настройки окружения и проверяет обязательные значения и таймауты.
func Load() (Config, error) {
	settings := Config{
		DatabaseURL: os.Getenv("DATABASE_URL"), HTTPAddr: ":8080",
		ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second,
		WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second,
		RequestTimeout: 10 * time.Second, ShutdownTimeout: 10 * time.Second,
		ConnectTimeout: 5 * time.Second, HealthTimeout: 2 * time.Second,
	}
	if strings.TrimSpace(settings.DatabaseURL) == "" {
		return Config{}, errors.New("DATABASE_URL не задан")
	}
	if address, exists := os.LookupEnv("HTTP_ADDR"); exists {
		settings.HTTPAddr = address
	}
	_, port, err := net.SplitHostPort(settings.HTTPAddr)
	if err != nil {
		return Config{}, errors.New("HTTP_ADDR должен иметь формат host:port")
	}
	number, err := strconv.Atoi(port)
	if err != nil || number < 0 || number > 65535 {
		return Config{}, errors.New("порт HTTP_ADDR должен быть от 0 до 65535")
	}
	durations := []struct {
		name  string
		value *time.Duration
	}{
		{"HTTP_READ_HEADER_TIMEOUT", &settings.ReadHeaderTimeout},
		{"HTTP_READ_TIMEOUT", &settings.ReadTimeout},
		{"HTTP_WRITE_TIMEOUT", &settings.WriteTimeout},
		{"HTTP_IDLE_TIMEOUT", &settings.IdleTimeout},
		{"HTTP_REQUEST_TIMEOUT", &settings.RequestTimeout},
		{"HTTP_SHUTDOWN_TIMEOUT", &settings.ShutdownTimeout},
		{"DB_CONNECT_TIMEOUT", &settings.ConnectTimeout},
		{"HEALTH_CHECK_TIMEOUT", &settings.HealthTimeout},
	}
	for _, setting := range durations {
		if value, exists := os.LookupEnv(setting.name); exists {
			duration, err := time.ParseDuration(value)
			if err != nil || duration <= 0 {
				return Config{}, fmt.Errorf("%s должен быть положительной длительностью, например 5s", setting.name)
			}
			*setting.value = duration
		}
	}
	if settings.WriteTimeout <= settings.RequestTimeout {
		return Config{}, errors.New("HTTP_WRITE_TIMEOUT должен быть больше HTTP_REQUEST_TIMEOUT")
	}
	if settings.ReadHeaderTimeout > settings.ReadTimeout {
		return Config{}, errors.New("HTTP_READ_HEADER_TIMEOUT не должен превышать HTTP_READ_TIMEOUT")
	}
	if value, exists := os.LookupEnv("LOG_LEVEL"); exists {
		if err := settings.LogLevel.UnmarshalText([]byte(value)); err != nil {
			return Config{}, errors.New("некорректный LOG_LEVEL: используйте DEBUG, INFO, WARN или ERROR")
		}
	}
	return settings, nil
}
