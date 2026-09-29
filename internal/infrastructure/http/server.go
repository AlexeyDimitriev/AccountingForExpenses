package httpapi

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"
)

// Serve обслуживает HTTP до отмены контекста, затем ждёт активные запросы в пределах таймаута.
func Serve(ctx context.Context, server *http.Server, listener net.Listener, shutdownTimeout time.Duration, logger *slog.Logger) error {
	requests, cancelRequests := context.WithCancel(context.Background())
	defer cancelRequests()
	server.BaseContext = func(net.Listener) context.Context { return requests }
	result := make(chan error, 1)
	go func() { result <- server.Serve(listener) }()
	logger.Info("HTTP-сервер запущен", "address", listener.Addr().String())
	select {
	case err := <-result:
		cancelRequests()
		_ = server.Close()
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("обслуживание HTTP: %w", err)
	case <-ctx.Done():
		logger.Info("Завершение HTTP-сервера")
	}
	shutdownContext, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := server.Shutdown(shutdownContext); err != nil {
		cancelRequests()
		_ = server.Close()
		<-result
		return fmt.Errorf("завершение HTTP-сервера: %w", err)
	}
	cancelRequests()
	err := <-result
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	logger.Info("HTTP-сервер остановлен")
	return nil
}
