package logging

import (
	"io"
	"log/slog"
)

// Создаёт JSON-логгер с общими полями сервиса.
func New(output io.Writer, service string, level slog.Level) *slog.Logger {
	handler := slog.NewJSONHandler(output, &slog.HandlerOptions{
		Level: level,
		ReplaceAttr: func(groups []string, attr slog.Attr) slog.Attr {
			if len(groups) == 0 {
				switch attr.Key {
				case slog.TimeKey:
					attr.Key = "timestamp"
				case slog.MessageKey:
					attr.Key = "message"
				}
			}
			return attr
		},
	})
	return slog.New(handler).With("service.name", service)
}
