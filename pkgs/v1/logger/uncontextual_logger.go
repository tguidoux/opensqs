package logger

import (
	"context"
	"log/slog"
	"os"
)

// UncontextualLogger provides a context-free wrapper around Logger.
// All methods use context.Background() internally.
type UncontextualLogger struct {
	logger *Logger
}

// NewUncontextualLogger creates a new UncontextualLogger with a custom handler.
func NewUncontextualLogger(name string, level ...slog.Level) *UncontextualLogger {
	return &UncontextualLogger{
		logger: NewLogger(name, level...),
	}
}

// Debug logs a debug-level message.
func (l *UncontextualLogger) Debug(msg string, extra ...map[string]any) LoggerInterface {
	l.logger.Debug(context.Background(), msg, extra...)
	return l
}

// Info logs an info-level message.
func (l *UncontextualLogger) Info(msg string, extra ...map[string]any) LoggerInterface {
	l.logger.Info(context.Background(), msg, extra...)
	return l
}

// Error logs an error-level message.
func (l *UncontextualLogger) Error(msg string, extra ...map[string]any) LoggerInterface {
	l.logger.Error(context.Background(), msg, extra...)
	return l
}

// Fatal logs an error-level message and exits.
func (l *UncontextualLogger) Fatal(msg string, extra ...map[string]any) LoggerInterface {
	l.logger.Fatal(context.Background(), msg, extra...)
	return l
}

// Warning logs a warning-level message.
func (l *UncontextualLogger) Warning(msg string, extra ...map[string]any) LoggerInterface {
	l.logger.Warning(context.Background(), msg, extra...)
	return l
}

// Debugf logs a debug-level message with formatted string.
func (l *UncontextualLogger) Debugf(format string, args ...any) LoggerInterface {
	l.logger.Debugf(context.Background(), format, args...)
	return l
}

// Infof logs an info-level message with formatted string.
func (l *UncontextualLogger) Infof(format string, args ...any) LoggerInterface {
	l.logger.Infof(context.Background(), format, args...)
	return l
}

// Errorf logs an error-level message with formatted string.
func (l *UncontextualLogger) Errorf(format string, args ...any) LoggerInterface {
	l.logger.Errorf(context.Background(), format, args...)
	return l
}

// Fatalf logs an error-level message with formatted string and exits.
func (l *UncontextualLogger) Fatalf(format string, args ...any) LoggerInterface {
	l.logger.Fatalf(context.Background(), format, args...)
	return l
}

// Warningf logs a warning-level message with formatted string.
func (l *UncontextualLogger) Warningf(format string, args ...any) LoggerInterface {
	l.logger.Warningf(context.Background(), format, args...)
	return l
}

// WithExtra adds extra fields to the logger.
func (l *UncontextualLogger) WithExtra(extra ...map[string]any) LoggerInterface {
	newLogger := l.logger.WithExtra(extra...)
	return &UncontextualLogger{logger: newLogger}
}

// GetWriter returns the output writer (currently always os.Stdout).
func (l *UncontextualLogger) GetWriter() *os.File {
	return l.logger.GetWriter()
}

// Printf implements the "github.com/rs/cors" Logger interface.
func (l *UncontextualLogger) Printf(msg string, args ...interface{}) {
	l.logger.Printf(msg, args...)
}

// GetLogger returns the underlying Logger if context is needed.
func (l *UncontextualLogger) GetLogger() *Logger {
	return l.logger
}
