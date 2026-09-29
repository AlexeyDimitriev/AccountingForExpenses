package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"AccountingForExpenses/internal/application"
	"AccountingForExpenses/internal/infrastructure/config"
	httpapi "AccountingForExpenses/internal/infrastructure/http"
	"AccountingForExpenses/internal/infrastructure/postgres"
)

// main запускает сервер и переводит SIGINT и SIGTERM в корректное завершение приложения.
func main() {
	settings, err := config.Load()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: settings.LogLevel}))
	slog.SetDefault(logger)
	if err != nil {
		logger.Error("Ошибка конфигурации", "error", err)
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	err = run(ctx, settings, logger)
	stop()
	if err != nil {
		logger.Error("Сервер завершился с ошибкой", "error", err)
		os.Exit(1)
	}
}

// run подключает БД, собирает зависимости и закрывает пул после остановки HTTP-сервера.
func run(ctx context.Context, settings config.Config, logger *slog.Logger) error {
	connectContext, cancel := context.WithTimeout(ctx, settings.ConnectTimeout)
	pool, err := postgres.Open(connectContext, settings.DatabaseURL)
	cancel()
	if err != nil {
		return err
	}
	defer pool.Close()
	categories := postgres.NewCategoryRepository(pool)
	expenses := postgres.NewExpenseRepository(pool)
	api := httpapi.NewRouter(application.NewCategoryService(categories, expenses), application.NewExpenseService(expenses, categories))
	api = httpapi.WithFrontend(api)
	ready := func(checkContext context.Context) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		return pool.Ping(checkContext)
	}
	api = httpapi.WithHealth(api, ready, settings.HealthTimeout)
	api = httpapi.WithRequestLogging(api, logger, settings.RequestTimeout)
	server := &http.Server{
		Addr: settings.HTTPAddr, Handler: api,
		ReadHeaderTimeout: settings.ReadHeaderTimeout, ReadTimeout: settings.ReadTimeout,
		WriteTimeout: settings.WriteTimeout, IdleTimeout: settings.IdleTimeout,
		ErrorLog: slog.NewLogLogger(logger.Handler(), slog.LevelError),
	}
	var listenConfig net.ListenConfig
	listener, err := listenConfig.Listen(ctx, "tcp", settings.HTTPAddr)
	if err != nil {
		return fmt.Errorf("открыть HTTP-порт: %w", err)
	}
	return httpapi.Serve(ctx, server, listener, settings.ShutdownTimeout, logger)
}
