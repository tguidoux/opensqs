package logger_test

import (
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tguidoux/opensqs/pkgs/v1/logger"
)

func TestNew_UncontextualLoggerType(t *testing.T) {
	log := logger.New("test", logger.UncontextualLoggerType)
	require.NotNil(t, log)

	// Should be UncontextualLogger
	_, ok := log.(*logger.UncontextualLogger)
	require.True(t, ok)
}

func TestNew_ContextLoggerType(t *testing.T) {
	log := logger.New("test", logger.ContextLoggerType)
	require.NotNil(t, log)

	// Should be UncontextualLogger (both types return UncontextualLogger in current implementation)
	_, ok := log.(*logger.UncontextualLogger)
	require.True(t, ok)
}

func TestNew_DefaultIsUncontextualLogger(t *testing.T) {
	// Default (no type specified) should be UncontextualLogger
	log := logger.New("test", logger.LoggerType(-1)) // invalid type should default to UncontextualLogger
	require.NotNil(t, log)

	_, ok := log.(*logger.UncontextualLogger)
	require.True(t, ok)
}

func TestNew_WithCustomLevel(t *testing.T) {
	syncLogger := logger.New("test", logger.UncontextualLoggerType, slog.LevelDebug)
	require.NotNil(t, syncLogger)

	contextLogger := logger.New("test", logger.ContextLoggerType, slog.LevelDebug)
	require.NotNil(t, contextLogger)
}

func TestNewUncontextual(t *testing.T) {
	log := logger.NewUncontextual("test")
	require.NotNil(t, log)

	_, ok := log.(*logger.UncontextualLogger)
	require.True(t, ok)
}

func TestNewUncontextualWithLevel(t *testing.T) {
	log := logger.NewUncontextual("test", slog.LevelDebug)
	require.NotNil(t, log)
}

func TestNewContextual(t *testing.T) {
	log := logger.NewContextual("test")
	require.NotNil(t, log)
}

func TestNewContextualWithLevel(t *testing.T) {
	log := logger.NewContextual("test", slog.LevelDebug)
	require.NotNil(t, log)
}

func TestLoggerInterfaceImplementation_SyncLogger(t *testing.T) {
	log := logger.New("test", logger.UncontextualLoggerType)
	require.NotNil(t, log)

	// Verify it implements LoggerInterface
	var _ logger.LoggerInterface = log
}

func TestSyncLoggerMethods(t *testing.T) {
	log := logger.New("test", logger.UncontextualLoggerType, slog.LevelDebug)
	require.NotNil(t, log)

	// All methods should be callable and return LoggerInterface
	result := log.Debug("debug")
	assert.NotNil(t, result)

	result = log.Info("info")
	assert.NotNil(t, result)

	result = log.Warning("warning")
	assert.NotNil(t, result)

	result = log.Error("error")
	assert.NotNil(t, result)

	result = log.Debugf("debugf %s", "format")
	assert.NotNil(t, result)

	result = log.Infof("infof %s", "format")
	assert.NotNil(t, result)

	result = log.Warningf("warningf %s", "format")
	assert.NotNil(t, result)

	result = log.Errorf("errorf %s", "format")
	assert.NotNil(t, result)
}

func TestLoggerInterfaceWithExtra(t *testing.T) {
	log := logger.New("test", logger.UncontextualLoggerType)
	require.NotNil(t, log)

	result := log.WithExtra(map[string]any{"key": "value"})
	require.NotNil(t, result)
}

func TestLoggerInterfaceGetWriter(t *testing.T) {
	log := logger.New("test", logger.UncontextualLoggerType)
	require.NotNil(t, log)

	writer := log.GetWriter()
	require.NotNil(t, writer)
}

func TestLoggerInterfacePrintf(t *testing.T) {
	log := logger.New("test", logger.UncontextualLoggerType)
	require.NotNil(t, log)

	// Should not panic
	log.Printf("test %s", "message")
}

func TestMethodChaining(t *testing.T) {
	log := logger.New("test", logger.UncontextualLoggerType, slog.LevelDebug)
	require.NotNil(t, log)

	// Should support method chaining
	log.
		WithExtra(map[string]any{"key": "value"}).
		Info("test message").
		Debugf("formatted %s", "message").
		WithExtra(map[string]any{"another": "field"}).
		Warning("warning message")
}

func TestSwitchBetweenLoggerTypes(t *testing.T) {
	// Simulate switching between logger types
	var log logger.LoggerInterface

	// Start with SyncLogger
	log = logger.New("test", logger.UncontextualLoggerType, slog.LevelDebug)
	require.NotNil(t, log)
	log.Info("using sync logger")

	// Switch to ContextLogger
	log = logger.New("test", logger.ContextLoggerType, slog.LevelDebug)
	require.NotNil(t, log)
	log.Info("using context logger")

	// Back to SyncLogger
	log = logger.New("test", logger.UncontextualLoggerType, slog.LevelDebug)
	require.NotNil(t, log)
	log.Info("back to sync logger")
}

func TestLoggerTypeConstants(t *testing.T) {
	// Verify constants are defined
	assert.Equal(t, logger.LoggerType(0), logger.UncontextualLoggerType)
	assert.Equal(t, logger.LoggerType(1), logger.ContextLoggerType)
	assert.NotEqual(t, logger.UncontextualLoggerType, logger.ContextLoggerType)
}

func TestContextualLoggerUsagePattern(t *testing.T) {
	// Context-aware logger for cases where context is important
	log := logger.NewContextual("test", slog.LevelDebug)
	require.NotNil(t, log)

	// This logger requires context for all operations
	ctx := context.Background()
	log.Info(ctx, "requires context")
	log.Error(ctx, "also requires context")
}

func TestMixingLoggerTypes(t *testing.T) {
	// You can mix logger types in the same application
	syncLogger := logger.NewUncontextual("sync", slog.LevelDebug)
	contextualLogger := logger.NewContextual("contextual", slog.LevelDebug)

	require.NotNil(t, syncLogger)
	require.NotNil(t, contextualLogger)

	// Use sync logger
	syncLogger.Info("no context needed")

	// Use contextual logger
	ctx := context.Background()
	contextualLogger.Info(ctx, "context provided")
}
