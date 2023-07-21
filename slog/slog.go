// Package slog provides structured logging for our Go services
// It is a wrapper for the https://pkg.go.dev/golang.org/x/exp/slog
// With some extra functionality to configure levels and formatters.
package slog

import (
	"context"
	"fmt"
	"math"
	"os"
	"strings"

	"golang.org/x/exp/slog"
)

// It is a good idea to extract this package as an library that multiple
// services can use. For now we have only one service in Go :-).

// Level determines the importance or severity of a log record
type Level = slog.Level

// Logger represents a logger instance with its own context.
type Logger = slog.Logger

// Format determines the output format of the log records
type Format string

// All available log levels
const (
	LevelInfo    Level = slog.LevelInfo
	LevelDebug   Level = slog.LevelDebug
	LevelWarn    Level = slog.LevelWarn
	LevelError   Level = slog.LevelError
	LevelDisable Level = math.MaxInt
)

// All available log formats
const (
	FormatText   = "text"
	FormatGcloud = "gcloud"
)

// Default configurations
const (
	DefaultLevel  = slog.LevelInfo
	DefaultFormat = FormatGcloud
)

// Config represents log configuration.
type Config struct {
	Level  Level
	Format Format
}

// LoadConfig will load the log Config of the service from environment variables.
// The service name is used as a prefix for the environment variables.
// So a service "TEST" will load the log level from "TEST_LOG_LEVEL".
//
// Available log levels are: "debug", "info", "warn", "error"
// Available log fmts are: "gcloud", "text"
//
// If the environment variables are not found it will use default values.
func LoadConfig(service string) (Config, error) {
	level := os.Getenv(service + "_LOG_LEVEL")
	format := os.Getenv(service + "_LOG_FMT")

	logFormat, err := ParseFormat(format)
	if err != nil {
		return Config{}, err
	}

	logLevel, err := ParseLevel(level)
	if err != nil {
		return Config{}, err
	}

	return Config{
		Level:  logLevel,
		Format: logFormat,
	}, nil
}

// Configure will change the default logger configuration.
// It should be called as soon as possible, usually on the main of your program.
func Configure(cfg Config) error {
	th := &slog.HandlerOptions{
		Level: cfg.Level,
	}

	var handler slog.Handler

	switch cfg.Format {
	case FormatText:
		handler = slog.NewTextHandler(os.Stderr, th)
	case FormatGcloud:
		th.ReplaceAttr = func(groups []string, a slog.Attr) slog.Attr {
			// Customize the name of some fields to match Google Cloud expectations
			// More: https://cloud.google.com/logging/docs/agent/logging/configuration#process-payload
			if a.Key == slog.LevelKey {
				a.Key = "severity"
			}
			if a.Key == slog.MessageKey {
				a.Key = "message"
			}
			return a
		}
		handler = slog.NewJSONHandler(os.Stderr, th)
	default:
		return fmt.Errorf("unknown log format: %v", cfg.Format)
	}

	logger := slog.New(handler)

	slog.SetDefault(logger)
	return nil
}

// Info calls Logger.Info on the default logger.
func Info(msg string, args ...any) {
	slog.Info(msg, args...)
}

// Debug calls Logger.Debug on the default logger.
func Debug(msg string, args ...any) {
	slog.Debug(msg, args...)
}

// Warn calls Logger.Warn on the default logger.
func Warn(msg string, args ...any) {
	slog.Warn(msg, args...)
}

// Error calls Logger.Error on the default logger.
func Error(msg string, args ...any) {
	slog.Error(msg, args...)
}

// Fatal is equivalent to Error() followed by a call to os.Exit(1).
func Fatal(msg string, args ...any) {
	Error(msg, args...)
	os.Exit(1)
}

// InfoCtx calls Logger.InfoCtx on the default logger.
func InfoCtx(ctx context.Context, msg string, args ...any) {
	slog.InfoCtx(ctx, msg, args...)
}

// DebugCtx calls Logger.DebugCtx on the default logger.
func DebugCtx(ctx context.Context, msg string, args ...any) {
	slog.DebugCtx(ctx, msg, args...)
}

// WarnCtx calls Logger.WarnCtx on the default logger.
func WarnCtx(ctx context.Context, msg string, args ...any) {
	slog.WarnCtx(ctx, msg, args...)
}

// ErrorCtx calls Logger.ErrorCtx on the default logger.
func ErrorCtx(ctx context.Context, msg string, args ...any) {
	slog.ErrorCtx(ctx, msg, args...)
}

// FataCtx is equivalent to ErrorCtx() followed by a call to os.Exit(1).
func FataCtx(ctx context.Context, msg string, args ...any) {
	ErrorCtx(ctx, msg, args...)
	os.Exit(1)
}

// With calls Logger.With on the default logger returning a new Logger instance.
func With(args ...any) *Logger {
	return slog.With(args...)
}

// FromCtx gets the [Logger] associated with the given context. A default [Logger] is
// returned if the context has no [Logger] associated with it.
func FromCtx(ctx context.Context) *slog.Logger {
	val := ctx.Value(loggerKey)
	log, ok := val.(*slog.Logger)
	if !ok {
		return slog.Default()
	}
	return log
}

// NewContext creates a new [context.Context] with the given [Logger] associated with it.
// Call [FromCtx] to retrieve the [Logger].
func NewContext(ctx context.Context, log *slog.Logger) context.Context {
	return context.WithValue(ctx, loggerKey, log)
}

// key is the type used to store data on contexts.
type key int

const (
	loggerKey key = iota
)

// ParseLevel parses the string and returns the corresponding [Level].
func ParseLevel(level string) (Level, error) {
	level = strings.ToLower(level)
	switch level {
	case "info", "":
		return LevelInfo, nil
	case "debug":
		return LevelDebug, nil
	case "warn":
		return LevelWarn, nil
	case "error":
		return LevelError, nil
	case "disable":
		return LevelDisable, nil
	default:
		return Level(666), fmt.Errorf("invalid log level: %q", level)
	}
}

// ParseFormat parses the string and returns the corresponding [Format].
func ParseFormat(format string) (Format, error) {
	switch format {
	case "gcloud", "text":
		return Format(format), nil
	case "":
		return FormatGcloud, nil
	default:
		return "", fmt.Errorf("unknown log format %q", format)
	}
}
