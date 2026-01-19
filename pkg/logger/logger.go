package logger

import (
	"fmt"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var globalLogger *zap.SugaredLogger

// Initialize sets up the global logger
func Initialize(level, format string) error {
	var config zap.Config

	if format == "json" {
		config = zap.NewProductionConfig()
	} else {
		config = zap.NewDevelopmentConfig()
		config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}

	// Parse log level
	var zapLevel zapcore.Level
	if err := zapLevel.UnmarshalText([]byte(level)); err != nil {
		return fmt.Errorf("invalid log level %q: %w", level, err)
	}
	config.Level = zap.NewAtomicLevelAt(zapLevel)

	logger, err := config.Build()
	if err != nil {
		return fmt.Errorf("failed to build logger: %w", err)
	}

	globalLogger = logger.Sugar()
	return nil
}

// Get returns the global logger
func Get() *zap.SugaredLogger {
	if globalLogger == nil {
		// Fallback to a default logger if not initialized
		logger, _ := zap.NewProduction()
		globalLogger = logger.Sugar()
	}
	return globalLogger
}

// Sync flushes any buffered log entries
func Sync() error {
	if globalLogger != nil {
		return globalLogger.Sync()
	}
	return nil
}

// Helper functions for convenient logging
func Debug(args ...interface{}) {
	Get().Debug(args...)
}

func Debugf(template string, args ...interface{}) {
	Get().Debugf(template, args...)
}

func Info(args ...interface{}) {
	Get().Info(args...)
}

func Infof(template string, args ...interface{}) {
	Get().Infof(template, args...)
}

func Warn(args ...interface{}) {
	Get().Warn(args...)
}

func Warnf(template string, args ...interface{}) {
	Get().Warnf(template, args...)
}

func Error(args ...interface{}) {
	Get().Error(args...)
}

func Errorf(template string, args ...interface{}) {
	Get().Errorf(template, args...)
}

func Fatal(args ...interface{}) {
	Get().Fatal(args...)
}

func Fatalf(template string, args ...interface{}) {
	Get().Fatalf(template, args...)
}
