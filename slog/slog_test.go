package slog_test

import (
	"context"
	"testing"

	"github.com/birdie-ai/golibs/slog"
)

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
	ctx := slog.WithContext(context.Background(), want)
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

const (
	service     = "TEST"
	logLevelEnv = service + "_LOG_LEVEL"
	logFmtEnv   = service + "_LOG_FMT"
)
