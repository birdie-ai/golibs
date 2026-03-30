package event_test

import "github.com/birdie-ai/golibs/slog"

func init() {
	if err := slog.Configure(slog.Config{
		Level:  slog.LevelDisable,
		Format: slog.FormatText,
	}); err != nil {
		panic(err)
	}
}
