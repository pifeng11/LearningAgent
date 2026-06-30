package observability

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"learning-agent/internal/model"
)

type AppError struct {
	Code    string
	Message string
	Cause   error
	Fields  map[string]any
}

func (e *AppError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return e.Code
}

func (e *AppError) Unwrap() error {
	return e.Cause
}

func NewError(code string, message string, fields ...any) error {
	return &AppError{
		Code:    code,
		Message: message,
		Fields:  fieldsFromPairs(fields...),
	}
}

func Wrap(err error, code string, message string, fields ...any) error {
	if err == nil {
		return nil
	}

	var appErr *AppError
	if errors.As(err, &appErr) {
		merged := fieldsFromPairs(fields...)
		for key, value := range appErr.Fields {
			merged[key] = value
		}
		return &AppError{
			Code:    firstNonEmpty(appErr.Code, code),
			Message: firstNonEmpty(appErr.Message, message),
			Cause:   appErr.Cause,
			Fields:  merged,
		}
	}

	return &AppError{
		Code:    code,
		Message: message,
		Cause:   err,
		Fields:  fieldsFromPairs(fields...),
	}
}

func Code(err error) string {
	var appErr *AppError
	if errors.As(err, &appErr) && appErr.Code != "" {
		return appErr.Code
	}
	return "internal_error"
}

func Message(err error) string {
	var appErr *AppError
	if errors.As(err, &appErr) && appErr.Message != "" {
		return appErr.Message
	}
	if err != nil {
		return err.Error()
	}
	return ""
}

func UserError(ctx context.Context, err error) map[string]any {
	return map[string]any{
		"code":     Code(err),
		"message":  Message(err),
		"trace_id": TraceID(ctx),
	}
}

func UserErrorText(ctx context.Context, err error) string {
	if traceID := TraceID(ctx); traceID != "" {
		return fmt.Sprintf("%s: %s (trace_id=%s)", Code(err), Message(err), traceID)
	}
	return fmt.Sprintf("%s: %s", Code(err), Message(err))
}

// LogError 是错误日志的统一出口，避免每一层都 slog.Error 导致重复日志。
// 内层只负责 Wrap 错误并补字段，边界层调用这里一次即可拿到 code、trace_id 和 cause。
func LogError(ctx context.Context, logger *slog.Logger, message string, err error, fields ...any) {
	if logger == nil {
		logger = slog.Default()
	}

	attrs := []slog.Attr{
		slog.String("trace_id", TraceID(ctx)),
		slog.String("error_code", Code(err)),
		slog.String("error", err.Error()),
	}
	if cause := errors.Unwrap(err); cause != nil {
		attrs = append(attrs, slog.String("cause", cause.Error()))
	}
	var modelErr *model.ModelError
	if errors.As(err, &modelErr) {
		attrs = append(attrs,
			slog.String("model_provider", modelErr.Provider),
			slog.String("model", modelErr.Model),
			slog.Int("model_status_code", modelErr.StatusCode),
			slog.Bool("model_retryable", modelErr.Retryable),
		)
		for key, value := range modelErr.Metadata {
			attrs = append(attrs, slog.Any("model_"+key, value))
		}
	}

	var appErr *AppError
	if errors.As(err, &appErr) {
		for key, value := range appErr.Fields {
			attrs = append(attrs, slog.Any(key, value))
		}
	}
	attrs = append(attrs, attrsFromPairs(fields...)...)

	logger.LogAttrs(ctx, slog.LevelError, message, attrs...)
}

func fieldsFromPairs(fields ...any) map[string]any {
	result := map[string]any{}
	for i := 0; i+1 < len(fields); i += 2 {
		key, ok := fields[i].(string)
		if !ok || key == "" {
			continue
		}
		result[key] = fields[i+1]
	}
	return result
}

func attrsFromPairs(fields ...any) []slog.Attr {
	attrs := make([]slog.Attr, 0, len(fields)/2)
	for i := 0; i+1 < len(fields); i += 2 {
		key, ok := fields[i].(string)
		if !ok || key == "" {
			continue
		}
		attrs = append(attrs, slog.Any(key, fields[i+1]))
	}
	return attrs
}

func firstNonEmpty(left string, right string) string {
	if left != "" {
		return left
	}
	return right
}
