package app

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/voronka/backend/internal/shared/config"
)

// NewLogger creates a configured zap logger based on app and logger config.
// Centralized logger initialization eliminates code duplication between API and workers.
func NewLogger(appCfg *config.AppConfig, logCfg *config.LoggerConfig) (*zap.Logger, error) {
	var zapCfg zap.Config

	// Use pretty console output for local environment
	if appCfg.IsLocal() {
		zapCfg = zap.NewDevelopmentConfig()
		zapCfg.Encoding = "console"
		zapCfg.EncoderConfig.EncodeLevel = customColorLevelEncoder
		zapCfg.EncoderConfig.EncodeTime = zapcore.TimeEncoderOfLayout("[15:04:05.000]")
		zapCfg.EncoderConfig.EncodeCaller = zapcore.ShortCallerEncoder
		zapCfg.EncoderConfig.ConsoleSeparator = " "
	} else if logCfg.Development {
		zapCfg = zap.NewDevelopmentConfig()
		zapCfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	} else {
		zapCfg = zap.NewProductionConfig()
	}

	// Set log level
	zapCfg.Level = zap.NewAtomicLevelAt(parseLogLevel(logCfg.Level))

	// Set encoding (only if not local, as local forces console)
	if !appCfg.IsLocal() && logCfg.Encoding != "" {
		zapCfg.Encoding = logCfg.Encoding
	}

	return zapCfg.Build()
}

// customColorLevelEncoder encodes log levels with custom colors for console output.
func customColorLevelEncoder(level zapcore.Level, enc zapcore.PrimitiveArrayEncoder) {
	var levelStr string
	switch level {
	case zapcore.DebugLevel:
		levelStr = "\x1b[35mDEBUG\x1b[0m" // Magenta
	case zapcore.InfoLevel:
		levelStr = "\x1b[34mINFO\x1b[0m" // Blue
	case zapcore.WarnLevel:
		levelStr = "\x1b[33mWARN\x1b[0m" // Yellow
	case zapcore.ErrorLevel, zapcore.DPanicLevel, zapcore.PanicLevel, zapcore.FatalLevel:
		levelStr = "\x1b[31m" + level.CapitalString() + "\x1b[0m" // Red
	default:
		levelStr = level.CapitalString()
	}
	enc.AppendString(levelStr)
}

// parseLogLevel converts string log level to zapcore.Level.
func parseLogLevel(level string) zapcore.Level {
	switch level {
	case "debug":
		return zap.DebugLevel
	case "info":
		return zap.InfoLevel
	case "warn":
		return zap.WarnLevel
	case "error":
		return zap.ErrorLevel
	default:
		return zap.InfoLevel
	}
}