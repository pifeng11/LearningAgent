package observability

import (
	"context"
	"crypto/rand"
	"encoding/hex"
)

type traceIDKey struct{}

// EnsureTraceID 在请求入口创建 trace_id，并通过 context 向下传递。
// 后续业务代码只需要返回带上下文的错误，入口层即可把 trace_id 透传给日志和用户。
func EnsureTraceID(ctx context.Context) context.Context {
	if TraceID(ctx) != "" {
		return ctx
	}
	return WithTraceID(ctx, NewTraceID())
}

func WithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, traceIDKey{}, traceID)
}

func TraceID(ctx context.Context) string {
	value, _ := ctx.Value(traceIDKey{}).(string)
	return value
}

func NewTraceID() string {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "trace-unknown"
	}
	return hex.EncodeToString(bytes[:])
}
