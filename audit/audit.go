// Package audit emits structured audit log lines for mutating actions.
// Caller identity is read from the slog context (set by [tracing.InstrumentHTTP]).
// The request body is never logged.
package audit

import (
	"context"

	"github.com/birdie-ai/golibs/slog"
)

// Log emits a structured audit line with the given action and target id.
func Log(ctx context.Context, action, targetID string) {
	slog.FromCtx(ctx).Info("audit",
		"action", action,
		"target_id", targetID,
	)
}
