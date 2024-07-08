package slog_test

import (
	"context"
	"os"
	"testing"

	"github.com/birdie-ai/golibs/slog"
)

func ExampleNew() {
	logger := slog.New(slog.NewGoogleCloudHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelWarn,
	}))
	logger.Info("omit", "a", 666)
	logger.Warn("yeah", "b", "yeah")
}

func TestLoadConfigDefault(t *testing.T) {
	config, err := slog.LoadConfig("DEFAULT")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if config.Level != slog.DefaultLevel {
		t.Errorf("got %v, want default level %v", config.Level, slog.DefaultLevel)
	}

	if config.Format != slog.DefaultFormat {
		t.Errorf("got %v, want default fmt %v", config.Format, slog.DefaultFormat)
	}
}

func TestLoadConfig(t *testing.T) {
	t.Setenv(logLevelEnv, "debug")
	t.Setenv(logFmtEnv, "text")

	config, err := slog.LoadConfig(service)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if config.Level != slog.LevelDebug {
		t.Errorf("got %v, want level %v", config.Level, slog.LevelDebug)
	}

	if config.Format != slog.FormatText {
		t.Errorf("got %v, want fmt %v", config.Format, slog.FormatText)
	}
}

func TestLoadConfigErr(t *testing.T) {
	t.Setenv(logLevelEnv, "debug")
	t.Setenv(logFmtEnv, "wrong")

	config, err := slog.LoadConfig(service)
	if err == nil {
		t.Fatalf("expected error, got config: %v", config)
	}

	t.Setenv(logLevelEnv, "wrong")
	t.Setenv(logFmtEnv, "text")

	config, err = slog.LoadConfig(service)
	if err == nil {
		t.Fatalf("expected error, got config: %v", config)
	}
}

func TestContextIntegration(t *testing.T) {
	want := &slog.Logger{}
	ctx := slog.NewContext(context.Background(), want)
	got := slog.FromCtx(ctx)

	if want != got {
		t.Fatalf("got %+v != want %+v", got, want)
	}
}

func TestDefaultLoggerFromContext(t *testing.T) {
	got := slog.FromCtx(context.Background())
	if got == nil {
		t.Fatal("want valid logger, got nil")
	}
}

func TestParseLevel(t *testing.T) {
	testcases := []struct {
		Input  string
		Output slog.Level
	}{
		{Input: "", Output: slog.LevelInfo},
		{Input: "info", Output: slog.LevelInfo},
		{Input: "debug", Output: slog.LevelDebug},
		{Input: "warn", Output: slog.LevelWarn},
		{Input: "error", Output: slog.LevelError},
		{Input: "disable", Output: slog.LevelDisable},
	}
	for _, tc := range testcases {
		t.Run(tc.Input, func(t *testing.T) {
			level, err := slog.ParseLevel(tc.Input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if level != tc.Output {
				t.Errorf("got %v, want %v", level, tc.Output)
			}
		})
	}
}

func TestParseLevel_Invalid(t *testing.T) {
	_, err := slog.ParseLevel("invalid")
	if err == nil {
		t.Fatal("want error, got nil")
	}
}

func TestParseFormat(t *testing.T) {
	testcases := []struct {
		Input  string
		Output slog.Format
	}{
		{Input: "", Output: slog.FormatGcloud},
		{Input: "gcloud", Output: slog.FormatGcloud},
		{Input: "text", Output: slog.FormatText},
	}
	for _, tc := range testcases {
		t.Run(tc.Input, func(t *testing.T) {
			level, err := slog.ParseFormat(tc.Input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if level != tc.Output {
				t.Errorf("got %v, want %v", level, tc.Output)
			}
		})
	}
}

func TestParseFormat_Invalid(t *testing.T) {
	_, err := slog.ParseFormat("invalid")
	if err == nil {
		t.Fatal("want error, got nil")
	}
}

func ExampleLoadConfig() {
	_ = slog.Configure(slog.Config{
		Level:  slog.LevelDebug,
		Format: slog.FormatText,
	})

	slog.Info("info msg", "key", "val", "key2", 666)

	_ = slog.Configure(slog.Config{
		Level:  slog.LevelDebug,
		Format: slog.FormatGcloud,
	})

	slog.Debug("debug msg", "key", "val", "key2", 666)
}

func ExampleLogger() {
	log := slog.Default()
	log = log.With("a", "val")
	log.Debug("debug", "b", 666)
	log.Info("info", "b", 666)
	log.Warn("warn", "b", 666)
	log.Error("error", "b", 666)
}

const (
	service     = "TEST"
	logLevelEnv = service + "_LOG_LEVEL"
	logFmtEnv   = service + "_LOG_FMT"
)
