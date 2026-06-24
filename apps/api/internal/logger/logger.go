package logger

import (
	"context"
	"log/slog"
	"os"
	"sync"
)

type contextKey int

const requestIDKey contextKey = iota

var (
	once sync.Once
	log  *slog.Logger
)

// Init initializes the global JSON logger. It is safe to call multiple times;
// only the first call has effect.
func Init(level string) {
	once.Do(func() {
		var lvl slog.Level
		switch level {
		case "debug":
			lvl = slog.LevelDebug
		case "warn":
			lvl = slog.LevelWarn
		case "error":
			lvl = slog.LevelError
		default:
			lvl = slog.LevelInfo
		}
		opts := &slog.HandlerOptions{Level: lvl}
		log = slog.New(slog.NewJSONHandler(os.Stdout, opts))
	})
}

// L returns the global logger. Init must have been called first.
func L() *slog.Logger {
	if log == nil {
		Init("info")
	}
	return log
}

// WithRequestID returns a context carrying the request ID for structured logging.
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey, requestID)
}

// RequestIDFrom extracts the request ID from the context.
func RequestIDFrom(ctx context.Context) string {
	v, _ := ctx.Value(requestIDKey).(string)
	return v
}

// InfoCtx logs an info message with optional attributes.
func InfoCtx(ctx context.Context, msg string, attrs ...slog.Attr) {
	L().LogAttrs(ctx, slog.LevelInfo, msg, attrs...)
}

// ErrorCtx logs an error message with optional attributes.
func ErrorCtx(ctx context.Context, msg string, err error, attrs ...slog.Attr) {
	if err != nil {
		attrs = append(attrs, slog.String("error", err.Error()))
	}
	L().LogAttrs(ctx, slog.LevelError, msg, attrs...)
}

// Attr is a convenience wrapper for constructing slog attributes.
func Attr(key string, value interface{}) slog.Attr {
	switch v := value.(type) {
	case string:
		return slog.String(key, v)
	case int:
		return slog.Int(key, v)
	case int64:
		return slog.Int64(key, v)
	case bool:
		return slog.Bool(key, v)
	case float64:
		return slog.Float64(key, v)
	case error:
		return slog.String(key, v.Error())
	default:
		return slog.Any(key, v)
	}
}
