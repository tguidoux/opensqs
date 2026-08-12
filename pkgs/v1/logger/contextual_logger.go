package logger

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"strings"
)

type Logger struct {
	logger *slog.Logger
	name   string
	extra  map[string]any
}

func cleanString(value string) string {
	cleanedValue := value
	if len(cleanedValue) > 1 && cleanedValue[0] == '"' && cleanedValue[len(cleanedValue)-1] == '"' {
		cleanedValue = cleanedValue[1 : len(cleanedValue)-1]
	}

	cleanedValue = strings.Trim(cleanedValue, "\n\r \t")

	return cleanedValue
}

// NewLogger creates a new instance of Logger with a custom handler.
func NewLogger(name string, level ...slog.Level) *Logger {
	var logLevel slog.Level
	if len(level) > 0 {
		logLevel = level[0]
	} else if os.Getenv("DEBUG") != "" {
		logLevel = slog.LevelDebug
	} else {
		logLevel = slog.LevelInfo
	}

	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
		ReplaceAttr: func(attrs []string, a slog.Attr) slog.Attr {
			// Custom formatting for log attributes
			if a.Key == slog.TimeKey {
				return slog.String("asctime", a.Value.String())
			} else if a.Key == slog.LevelKey {
				return slog.String("level", a.Value.String())
			} else if a.Key == "extra" {
				return slog.String("extra", a.Value.String())
			}
			return a
		},
	})

	logger := slog.New(handler)
	return &Logger{logger: logger, name: name, extra: make(map[string]any)}
}

// Log logs a message with the given level and additional context.
func (l *Logger) Log(ctx context.Context, level slog.Level, msg string, extra ...map[string]any) {
	attrs := []slog.Attr{
		slog.String("logger.name", l.name),
	}
	if len(extra) > 0 && extra[0] != nil {
		for k, v := range extra[0] {
			jsonValue, err := json.Marshal(v)
			if err == nil {
				attrs = append(attrs, slog.String(k, cleanString(string(jsonValue))))
			} else {
				attrs = append(attrs, slog.String(k, "<error marshaling>"))
			}
		}
	}

	// Add extra attributes from the logger's extra map
	for k, v := range l.extra {
		jsonValue, err := json.Marshal(v)
		if err == nil {
			attrs = append(attrs, slog.String(k, cleanString(string(jsonValue))))
		} else {
			attrs = append(attrs, slog.String(k, "<error marshaling>"))
		}
	}

	// Strip newlines from the message
	msg = cleanString(msg)

	l.logger.LogAttrs(ctx, level, msg, attrs...)
}

// Debug logs a debug-level message.
func (l *Logger) Debug(ctx context.Context, msg string, extra ...map[string]any) {
	l.Log(ctx, slog.LevelDebug, msg, extra...)
}

// Info logs an info-level message.
func (l *Logger) Info(ctx context.Context, msg string, extra ...map[string]any) {
	l.Log(ctx, slog.LevelInfo, msg, extra...)
}

// Error logs an error-level message.
func (l *Logger) Error(ctx context.Context, msg string, extra ...map[string]any) {
	l.Log(ctx, slog.LevelError, msg, extra...)
}

func (l *Logger) Fatal(ctx context.Context, msg string, extra ...map[string]any) {
	l.Log(ctx, slog.LevelError, msg, extra...)
	os.Exit(1)
}

// Warning logs a warning-level message.
func (l *Logger) Warning(ctx context.Context, msg string, extra ...map[string]any) {
	l.Log(ctx, slog.LevelWarn, msg, extra...)
}

// Debugf logs a debug-level message with formatted string.
func (l *Logger) Debugf(ctx context.Context, format string, args ...any) {
	l.Log(ctx, slog.LevelDebug, fmt.Sprintf(format, args...))
}

// Infof logs an info-level message with formatted string.
func (l *Logger) Infof(ctx context.Context, format string, args ...any) {
	l.Log(ctx, slog.LevelInfo, fmt.Sprintf(format, args...))
}

// Errorf logs an error-level message with formatted string.
func (l *Logger) Errorf(ctx context.Context, format string, args ...any) {
	l.Log(ctx, slog.LevelError, fmt.Sprintf(format, args...))
}

// Fatalf logs an error-level message with formatted string and exits.
func (l *Logger) Fatalf(ctx context.Context, format string, args ...any) {
	l.Log(ctx, slog.LevelError, fmt.Sprintf(format, args...))
	os.Exit(1)
}

// Warningf logs a warning-level message with formatted string.
func (l *Logger) Warningf(ctx context.Context, format string, args ...any) {
	l.Log(ctx, slog.LevelWarn, fmt.Sprintf(format, args...))
}

func (l *Logger) WithExtra(extra ...map[string]any) *Logger {
	newExtra := make(map[string]any, len(l.extra)+1)
	maps.Copy(newExtra, l.extra)
	if len(extra) > 0 && extra[0] != nil {
		maps.Copy(newExtra, extra[0])
	}

	return &Logger{
		logger: l.logger,
		name:   l.name,
		extra:  newExtra,
	}
}

// GetWriter returns the output writer (currently always os.Stdout)
func (l *Logger) GetWriter() *os.File {
	return os.Stdout
}

// Implement "github.com/rs/cors" Logger interface
func (l *Logger) Printf(msg string, args ...interface{}) {
	l.Log(context.Background(), slog.LevelInfo, fmt.Sprintf(msg, args...))
}
