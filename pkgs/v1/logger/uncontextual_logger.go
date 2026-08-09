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
func (s *UncontextualLogger) Debug(msg string, extra ...map[string]any) LoggerInterface {
	s.logger.Debug(context.Background(), msg, extra...)
	return s
}

// Info logs an info-level message.
func (s *UncontextualLogger) Info(msg string, extra ...map[string]any) LoggerInterface {
	s.logger.Info(context.Background(), msg, extra...)
	return s
}

// Error logs an error-level message.
func (s *UncontextualLogger) Error(msg string, extra ...map[string]any) LoggerInterface {
	s.logger.Error(context.Background(), msg, extra...)
	return s
}

// Fatal logs an error-level message and exits.
func (s *UncontextualLogger) Fatal(msg string, extra ...map[string]any) LoggerInterface {
	s.logger.Fatal(context.Background(), msg, extra...)
	return s
}

// Warning logs a warning-level message.
func (s *UncontextualLogger) Warning(msg string, extra ...map[string]any) LoggerInterface {
	s.logger.Warning(context.Background(), msg, extra...)
	return s
}

// Debugf logs a debug-level message with formatted string.
func (s *UncontextualLogger) Debugf(format string, args ...any) LoggerInterface {
	s.logger.Debugf(context.Background(), format, args...)
	return s
}

// Infof logs an info-level message with formatted string.
func (s *UncontextualLogger) Infof(format string, args ...any) LoggerInterface {
	s.logger.Infof(context.Background(), format, args...)
	return s
}

// Errorf logs an error-level message with formatted string.
func (s *UncontextualLogger) Errorf(format string, args ...any) LoggerInterface {
	s.logger.Errorf(context.Background(), format, args...)
	return s
}

// Fatalf logs an error-level message with formatted string and exits.
func (s *UncontextualLogger) Fatalf(format string, args ...any) LoggerInterface {
	s.logger.Fatalf(context.Background(), format, args...)
	return s
}

// Warningf logs a warning-level message with formatted string.
func (s *UncontextualLogger) Warningf(format string, args ...any) LoggerInterface {
	s.logger.Warningf(context.Background(), format, args...)
	return s
}

// WithExtra adds extra fields to the logger.
func (s *UncontextualLogger) WithExtra(extra ...map[string]any) LoggerInterface {
	s.logger.WithExtra(extra...)
	return s
}

// GetWriter returns the output writer (currently always os.Stdout).
func (s *UncontextualLogger) GetWriter() *os.File {
	return s.logger.GetWriter()
}

// Printf implements the "github.com/rs/cors" Logger interface.
func (s *UncontextualLogger) Printf(msg string, args ...interface{}) {
	s.logger.Printf(msg, args...)
}

// GetLogger returns the underlying Logger if context is needed.
func (s *UncontextualLogger) GetLogger() *Logger {
	return s.logger
}
