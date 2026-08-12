// Package logger provides structured logging utilities for OpenSQS services,
// supporting both contextual and uncontextual loggers via slog.
package logger

import (
	"log/slog"
	"os"
)

// LoggerInterface defines the context-free logger interface.
// Both SyncLogger and a context-aware wrapper implement this interface.
type LoggerInterface interface {
	Debug(msg string, extra ...map[string]any) LoggerInterface
	Info(msg string, extra ...map[string]any) LoggerInterface
	Error(msg string, extra ...map[string]any) LoggerInterface
	Warning(msg string, extra ...map[string]any) LoggerInterface
	Fatal(msg string, extra ...map[string]any) LoggerInterface
	Debugf(format string, args ...any) LoggerInterface
	Infof(format string, args ...any) LoggerInterface
	Errorf(format string, args ...any) LoggerInterface
	Warningf(format string, args ...any) LoggerInterface
	Fatalf(format string, args ...any) LoggerInterface
	WithExtra(extra ...map[string]any) LoggerInterface
	GetWriter() *os.File
	Printf(msg string, args ...interface{})
}

// LoggerType determines which logger implementation to use.
type LoggerType int

const (
	// UncontextualLoggerType creates an UncontextualLogger (context-free)
	UncontextualLoggerType LoggerType = iota
	// ContextLoggerType creates a Logger wrapped to be context-free
	ContextLoggerType
)

// New creates a logger instance based on the specified type.
// Default is UncontextualLoggerType for easier use without context.
func New(name string, loggerType LoggerType, level ...slog.Level) LoggerInterface {
	switch loggerType {
	case ContextLoggerType:
		return NewUncontextualLogger(name, level...)
	case UncontextualLoggerType:
		fallthrough
	default:
		return NewUncontextualLogger(name, level...)
	}
}

// NewUncontextual creates a new UncontextualLogger (context-free).
func NewUncontextual(name string, level ...slog.Level) LoggerInterface {
	return NewUncontextualLogger(name, level...)
}

// NewContextual creates a new context-aware Logger.
// Note: This returns a *Logger which requires context.Context for all logging methods.
func NewContextual(name string, level ...slog.Level) *Logger {
	return NewLogger(name, level...)
}
