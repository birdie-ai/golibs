// Package slog provides structured logging for our Go services
// It is a wrapper for the https://pkg.go.dev/golang.org/x/exp/slog
// With some extra functionality to configure levels and formatters.
package slog

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"math"
	"os"
	"strings"
)

type (
	// A Handler handles log records produced by a Logger.
	Handler = slog.Handler

	// HandlerOptions are options for a [TextHandler] or [JSONHandler].
	// A zero HandlerOptions consists entirely of default values.
	HandlerOptions = slog.HandlerOptions

	// Level determines the importance or severity of a log record
	Level = slog.Level

	// Logger represents a logger instance with its own context.
	// It extends Go's slog.Logger by adding new methods, like [Logger.Fatal].
	Logger struct {
		*slog.Logger
	}

	// Format determines the output format of the log records
	Format string
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

// Fatal is equivalent to [Logger.Error] followed by a call to os.Exit(1).
func (l *Logger) Fatal(msg string, args ...any) {
	l.Error(msg, args...)
	os.Exit(1)
}

// With calls Logger.With on the default logger returning a new Logger instance.
func (l *Logger) With(args ...any) *Logger {
	return &Logger{l.Logger.With(args...)}
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

// New creates a new Logger with the given non-nil Handler.
func New(h Handler) *Logger {
	return &Logger{slog.New(h)}
}

// NewGoogleCloudHandler creates a [JSONHandler] that writes to w in a format that works well with Google Cloud Logging.
func NewGoogleCloudHandler(w io.Writer, opts *slog.HandlerOptions) *slog.JSONHandler {
	opts.ReplaceAttr = func(groups []string, a slog.Attr) slog.Attr {
		// Customize the name of some fields to match Google Cloud expectations
		// More: https://cloud.google.com/logging/docs/agent/logging/configuration#process-payload
		if len(groups) > 0 {
			return a
		}
		switch a.Key {
		case slog.LevelKey:
			a.Key = "severity"
		case slog.MessageKey:
			a.Key = "message"
		case "http_request":
			a.Key, a.Value = convertHTTPRequest(a.Key, a.Value)
		}
		return a
	}
	return slog.NewJSONHandler(w, opts)
}

// Configure will change the default logger configuration.
// It should be called as soon as possible, usually on the main of your program.
func Configure(cfg Config) error {
	opts := &slog.HandlerOptions{
		Level: cfg.Level,
	}

	var handler slog.Handler

	switch cfg.Format {
	case FormatText:
		handler = slog.NewTextHandler(os.Stderr, opts)
	case FormatGcloud:
		handler = NewGoogleCloudHandler(os.Stderr, opts)
	default:
		return fmt.Errorf("unknown log format: %v", cfg.Format)
	}

	logger := slog.New(handler)
	slog.SetDefault(logger)
	return nil
}

// Customize the http request fields
// More: https://cloud.google.com/logging/docs/reference/v2/rest/v2/LogEntry#HttpRequest
func convertHTTPRequest(origKey string, origValue slog.Value) (string, slog.Value) {
	var attrs []slog.Attr
	value, ok := origValue.Any().(map[string]any)
	if !ok {
		return origKey, origValue
	}
	for key, value := range value {
		switch key {
		case "method":
			attrs = append(attrs, slog.Any("requestMethod", value))
		case "url":
			attrs = append(attrs, slog.Any("requestUrl", value))
		case "request_size":
			attrs = append(attrs, slog.Any("requestSize", value))
		case "status_code":
			attrs = append(attrs, slog.Any("status", value))
		case "response_size":
			attrs = append(attrs, slog.Any("responseSize", value))
		case "user_agent":
			attrs = append(attrs, slog.Any("userAgent", value))
		case "elapsed":
			attrs = append(attrs, slog.Any("latency", value))
		default:
			attrs = append(attrs, slog.Any(key, value))
		}
	}
	return "httpRequest", slog.GroupValue(attrs...)
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

// With calls Logger.With on the default logger returning a new Logger instance.
func With(args ...any) *Logger {
	return &Logger{slog.With(args...)}
}

// Default creates a new [Logger] with default configurations.
func Default() *Logger {
	return &Logger{slog.Default()}
}

// FromCtx gets the [Logger] associated with the given context. A default [Logger] is
// returned if the context has no [Logger] associated with it.
func FromCtx(ctx context.Context) *Logger {
	val := ctx.Value(loggerKey)
	log, ok := val.(*Logger)
	if !ok {
		return Default()
	}
	return log
}

// NewContext creates a new [context.Context] with the given [Logger] associated with it.
// Call [FromCtx] to retrieve the [Logger].
func NewContext(ctx context.Context, log *Logger) context.Context {
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
