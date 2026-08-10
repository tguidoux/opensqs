package logger_test

import (
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tguidoux/opensqs/pkgs/v1/logger"
)

func TestNewUncontextualLogger_WithDefaultLevel(t *testing.T) {
	// Unset DEBUG env var
	oldDebug, debugSet := os.LookupEnv("DEBUG")
	if debugSet {
		os.Unsetenv("DEBUG")
	}
	defer func() {
		if debugSet {
			os.Setenv("DEBUG", oldDebug)
		}
	}()

	simpleLogger := logger.NewUncontextualLogger("test")
	require.NotNil(t, simpleLogger)
}

func TestNewUncontextualLogger_WithCustomLevel(t *testing.T) {
	simpleLogger := logger.NewUncontextualLogger("test", slog.LevelDebug)
	require.NotNil(t, simpleLogger)
}

func TestUncontextualLoggerDebug(t *testing.T) {
	simpleLogger := logger.NewUncontextualLogger("test", slog.LevelDebug)
	require.NotNil(t, simpleLogger)

	// Just ensure no panic
	simpleLogger.Debug("test message")
}

func TestUncontextualLoggerInfo(t *testing.T) {
	simpleLogger := logger.NewUncontextualLogger("test", slog.LevelInfo)
	require.NotNil(t, simpleLogger)

	simpleLogger.Info("test message")
}

func TestUncontextualLoggerError(t *testing.T) {
	simpleLogger := logger.NewUncontextualLogger("test", slog.LevelError)
	require.NotNil(t, simpleLogger)

	simpleLogger.Error("test message")
}

func TestUncontextualLoggerWarning(t *testing.T) {
	simpleLogger := logger.NewUncontextualLogger("test", slog.LevelWarn)
	require.NotNil(t, simpleLogger)

	simpleLogger.Warning("test message")
}

func TestUncontextualLoggerDebugf(t *testing.T) {
	simpleLogger := logger.NewUncontextualLogger("test", slog.LevelDebug)
	require.NotNil(t, simpleLogger)

	simpleLogger.Debugf("test message %s", "formatted")
}

func TestUncontextualLoggerInfof(t *testing.T) {
	simpleLogger := logger.NewUncontextualLogger("test", slog.LevelInfo)
	require.NotNil(t, simpleLogger)

	simpleLogger.Infof("test message %s", "formatted")
}

func TestUncontextualLoggerErrorf(t *testing.T) {
	simpleLogger := logger.NewUncontextualLogger("test", slog.LevelError)
	require.NotNil(t, simpleLogger)

	simpleLogger.Errorf("test message %s", "formatted")
}

func TestUncontextualLoggerWarningf(t *testing.T) {
	simpleLogger := logger.NewUncontextualLogger("test", slog.LevelWarn)
	require.NotNil(t, simpleLogger)

	simpleLogger.Warningf("test message %s", "formatted")
}

func TestUncontextualLoggerWithoutContext(t *testing.T) {
	simpleLogger := logger.NewUncontextualLogger("test", slog.LevelInfo)
	require.NotNil(t, simpleLogger)

	// UncontextualLogger doesn't need context
	simpleLogger.Info("test message without context")
}

func TestUncontextualLoggerWithExtra(t *testing.T) {
	simpleLogger := logger.NewUncontextualLogger("test", slog.LevelInfo)
	require.NotNil(t, simpleLogger)

	// Add extra fields
	simpleLoggerWithExtra := simpleLogger.WithExtra(map[string]any{"key": "value", "number": 42})
	require.NotNil(t, simpleLoggerWithExtra)
	assert.NotEqual(t, simpleLoggerWithExtra, simpleLogger) // Should return a new instance

	// Log should include extra
	simpleLoggerWithExtra.Info("test message")
}

func TestUncontextualLoggerWithExtraMultipleCalls(t *testing.T) {
	simpleLogger := logger.NewUncontextualLogger("test", slog.LevelInfo)
	require.NotNil(t, simpleLogger)

	simpleLogger.WithExtra(map[string]any{"key1": "value1"})
	simpleLogger.WithExtra(map[string]any{"key2": "value2"})
	// Verify no panic - internal field access removed for external test
}

func TestUncontextualLoggerWithExtraNil(t *testing.T) {
	simpleLogger := logger.NewUncontextualLogger("test", slog.LevelInfo)
	require.NotNil(t, simpleLogger)

	// Should not panic with nil
	result := simpleLogger.WithExtra(nil)
	require.NotNil(t, result)
}

func TestUncontextualLoggerGetWriter(t *testing.T) {
	simpleLogger := logger.NewUncontextualLogger("test", slog.LevelInfo)
	require.NotNil(t, simpleLogger)

	writer := simpleLogger.GetWriter()
	assert.Equal(t, os.Stdout, writer)
}

func TestUncontextualLoggerPrintf(t *testing.T) {
	simpleLogger := logger.NewUncontextualLogger("test", slog.LevelInfo)
	require.NotNil(t, simpleLogger)

	// Should not panic
	simpleLogger.Printf("test message %s", "formatted")
}

func TestUncontextualLoggerGetLogger(t *testing.T) {
	simpleLogger := logger.NewUncontextualLogger("test", slog.LevelInfo)
	require.NotNil(t, simpleLogger)

	log := simpleLogger.GetLogger()
	require.NotNil(t, log)
}

func TestUncontextualLoggerLogWithExtra(t *testing.T) {
	simpleLogger := logger.NewUncontextualLogger("test", slog.LevelInfo)
	require.NotNil(t, simpleLogger)

	// Test with simple types
	simpleLogger.Info("test message", map[string]any{
		"string": "value",
		"number": 123,
		"bool":   true,
		"float":  3.14,
	})

	// Test with complex types
	simpleLogger.Error("test message", map[string]any{
		"nested": map[string]string{"key": "value"},
		"list":   []int{1, 2, 3},
	})
}

func TestUncontextualLoggerDebugWithExtra(t *testing.T) {
	simpleLogger := logger.NewUncontextualLogger("test", slog.LevelDebug)
	require.NotNil(t, simpleLogger)

	simpleLogger.Debug("debug message", map[string]any{"key": "value"})
}

func TestUncontextualLoggerErrorWithExtra(t *testing.T) {
	simpleLogger := logger.NewUncontextualLogger("test", slog.LevelError)
	require.NotNil(t, simpleLogger)

	simpleLogger.Error("error message", map[string]any{"error_code": 500})
}

func TestUncontextualLoggerWarningWithExtra(t *testing.T) {
	simpleLogger := logger.NewUncontextualLogger("test", slog.LevelWarn)
	require.NotNil(t, simpleLogger)

	simpleLogger.Warning("warning message", map[string]any{"warning_type": "deprecation"})
}

func TestUncontextualLoggerMultipleLevels(t *testing.T) {
	simpleLogger := logger.NewUncontextualLogger("test", slog.LevelDebug)
	require.NotNil(t, simpleLogger)

	// Log at all levels
	simpleLogger.Debug("debug")
	simpleLogger.Info("info")
	simpleLogger.Warning("warning")
	simpleLogger.Error("error")
}

func TestUncontextualLoggerContextFree(t *testing.T) {
	// The main point of UncontextualLogger is that it doesn't require context
	simpleLogger := logger.NewUncontextualLogger("test", slog.LevelInfo)

	// Should be able to call all methods without context
	simpleLogger.Info("no context needed")
	simpleLogger.Infof("formatted %s", "message")
	simpleLogger.Debug("debug without context")
	simpleLogger.Warning("warning without context")
	simpleLogger.Error("error without context")
}

func TestUncontextualLoggerChaining(t *testing.T) {
	simpleLogger := logger.NewUncontextualLogger("test", slog.LevelInfo)

	// WithExtra should return UncontextualLogger for chaining
	simpleLogger.WithExtra(map[string]any{"key1": "value1"}).
		WithExtra(map[string]any{"key2": "value2"}).
		Info("chained call")
}

func TestUncontextualLoggerUsesContextBackground(t *testing.T) {
	// Ensure that UncontextualLogger properly uses context.Background()
	// by verifying it doesn't panic with nil context
	simpleLogger := logger.NewUncontextualLogger("test", slog.LevelInfo)

	// These should not panic since context.Background() is used internally
	simpleLogger.Info("test")
	simpleLogger.Error("error")
	simpleLogger.Debug("debug")
}
