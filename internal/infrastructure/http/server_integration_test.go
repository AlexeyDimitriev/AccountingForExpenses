//go:build integration

package httpapi

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"testing"
	"time"
)

// startServer запускает настоящий HTTP-сервер на свободном локальном порту.
func startServer(t *testing.T, handler http.Handler, shutdownTimeout time.Duration) (string, context.CancelFunc, <-chan error, *http.Server) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	server := &http.Server{Handler: handler, ReadHeaderTimeout: time.Second}
	t.Cleanup(func() { _ = server.Close() })
	result := make(chan error, 1)
	go func() {
		result <- Serve(ctx, server, listener, shutdownTimeout, slog.New(slog.NewTextHandler(io.Discard, nil)))
	}()
	return "http://" + listener.Addr().String(), cancel, result, server
}

// awaitResult ограничивает ожидание завершения сервера в тесте.
func awaitResult(t *testing.T, result <-chan error) error {
	t.Helper()
	select {
	case err := <-result:
		return err
	case <-time.After(5 * time.Second):
		t.Fatal("server did not stop")
		return nil
	}
}

// getResponse выполняет запрос с ограниченным временем и освобождает соединение.
func getResponse(address string) error {
	client := &http.Client{Timeout: 5 * time.Second}
	response, err := client.Get(address)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	_, err = io.Copy(io.Discard, response.Body)
	if err == nil && response.StatusCode != 200 {
		return errors.New("unexpected response status")
	}
	return err
}

// TestGracefulShutdown проверяет, что сигнал завершения не прерывает активный запрос.
func TestGracefulShutdown(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(entered)
		select {
		case <-release:
			_, _ = w.Write([]byte("ok"))
		case <-r.Context().Done():
			t.Error("request canceled before grace period")
		}
	})
	address, stop, result, server := startServer(t, handler, 2*time.Second)
	shutdownStarted := make(chan struct{})
	server.RegisterOnShutdown(func() { close(shutdownStarted) })
	response := make(chan error, 1)
	go func() { response <- getResponse(address) }()
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("handler did not start")
	}
	stop()
	select {
	case <-shutdownStarted:
	case <-time.After(3 * time.Second):
		t.Fatal("shutdown did not start")
	}
	select {
	case err := <-result:
		t.Fatalf("server stopped before request finished: %v", err)
	default:
	}
	close(release)
	if err := awaitResult(t, response); err != nil {
		t.Fatal(err)
	}
	if err := awaitResult(t, result); err != nil {
		t.Fatal(err)
	}
}

// TestShutdownTimeout проверяет принудительную отмену запроса после истечения времени ожидания.
func TestShutdownTimeout(t *testing.T) {
	entered := make(chan struct{})
	canceled := make(chan struct{})
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { close(entered); <-r.Context().Done(); close(canceled) })
	address, stop, result, _ := startServer(t, handler, 20*time.Millisecond)
	response := make(chan error, 1)
	go func() { response <- getResponse(address) }()
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("handler did not start")
	}
	stop()
	if err := awaitResult(t, result); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v", err)
	}
	select {
	case <-canceled:
	case <-time.After(3 * time.Second):
		t.Fatal("request context not canceled")
	}
	_ = awaitResult(t, response)
}

// TestServeFailure проверяет возврат ошибки listener без зависания процесса.
func TestServeFailure(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	_ = listener.Close()
	err = Serve(t.Context(), &http.Server{}, listener, time.Second, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err == nil {
		t.Fatal("expected listener error")
	}
}
