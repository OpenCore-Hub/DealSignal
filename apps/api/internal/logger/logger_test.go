package logger

import (
	"context"
	"log/slog"
	"testing"
)

func TestWithRequestID(t *testing.T) {
	ctx := WithRequestID(context.Background(), "req-123")
	if got := RequestIDFrom(ctx); got != "req-123" {
		t.Fatalf("expected request id req-123, got %s", got)
	}
}

func TestAttr(t *testing.T) {
	cases := []struct {
		name  string
		value interface{}
		want  slog.Kind
	}{
		{"string", "hello", slog.KindString},
		{"int", 42, slog.KindInt64},
		{"bool", true, slog.KindBool},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			attr := Attr("key", c.value)
			if attr.Value.Kind() != c.want {
				t.Fatalf("expected kind %v, got %v", c.want, attr.Value.Kind())
			}
		})
	}
}
