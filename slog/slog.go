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

// Key represents a logging key/field
type Key int

// Tracing related keys
const (
	TraceID Key = iota
	OrgID
)

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

	logFormat, err := validateFormat(format)
	if err != nil {
		return Config{}, err
	}

	logLevel, err := validateLogLevel(level)
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
	th := slog.HandlerOptions{
		Level: cfg.Level,
	}

	var handler slog.Handler

	switch cfg.Format {
	case FormatText:
		handler = th.NewTextHandler(os.Stderr)
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
		handler = th.NewJSONHandler(os.Stderr)
	default:
		return fmt.Errorf("unknown log format: %v", cfg.Format)
	}

	logger := slog.New(&tracedHandler{
		handler: handler,
	})
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

// With calls Logger.With on the default logger returning a new Logger instance.
func With(args ...any) *Logger {
	return slog.With(args...)
}

type tracedHandler struct {
	handler slog.Handler
}

func (t *tracedHandler) Enabled(ctx context.Context, l slog.Level) bool {
	return t.handler.Enabled(ctx, l)
}

func (t *tracedHandler) Handle(ctx context.Context, record slog.Record) error {
	if traceID, ok := getValue(ctx, TraceID); ok {
		record.Add("trace_id", traceID)
	}
	if orgID, ok := getValue(ctx, OrgID); ok {
		record.Add("organization_id", orgID)
	}
	return t.handler.Handle(ctx, record)
}

func getValue(ctx context.Context, key any) (string, bool) {
	val := ctx.Value(key)
	if val == nil {
		return "", false
	}
	str, ok := val.(string)
	if !ok {
		return "", false
	}
	return str, true
}

func (t *tracedHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return t.handler.WithAttrs(attrs)
}

func (t *tracedHandler) WithGroup(name string) slog.Handler {
	return t.handler.WithGroup(name)
}

func validateLogLevel(level string) (Level, error) {
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

func validateFormat(format string) (Format, error) {
	switch format {
	case "gcloud", "text":
		return Format(format), nil
	case "":
		return FormatGcloud, nil
	default:
		return "", fmt.Errorf("unknown log format %q", format)
	}
}
