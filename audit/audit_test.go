package audit_test

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/birdie-ai/golibs/audit"
	"github.com/birdie-ai/golibs/slog"
)

func TestLog(t *testing.T) {
	cases := []struct {
		name     string
		ctxAttrs []any
		want     map[string]string
		notWant  []string
	}{
		{
			name:     "with identity in ctx",
			ctxAttrs: []any{"user_id", "usr-1", "user_email", "ana@birdie.ai"},
			want: map[string]string{
				"message":    "audit",
				"action":     "credential.create",
				"target_id":  "cred-42",
				"user_id":    "usr-1",
				"user_email": "ana@birdie.ai",
			},
		},
		{
			name: "without identity in ctx",
			want: map[string]string{
				"message":   "audit",
				"action":    "credential.create",
				"target_id": "cred-42",
			},
			notWant: []string{"user_id", "user_email"},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			buf := new(bytes.Buffer)
			logger := slog.New(slog.NewGoogleCloudHandler(buf, &slog.HandlerOptions{}))
			if len(c.ctxAttrs) > 0 {
				logger = logger.With(c.ctxAttrs...)
			}
			ctx := slog.NewContext(context.Background(), logger)

			audit.Log(ctx, "credential.create", "cred-42")

			var entry map[string]any
			err := json.Unmarshal(buf.Bytes(), &entry)
			if err != nil {
				t.Fatalf("invalid log entry: %v\n%s", err, buf.String())
			}

			for k, want := range c.want {
				got, _ := entry[k].(string)
				if got != want {
					t.Errorf("entry[%q] = %q, want %q", k, got, want)
				}
			}
			for _, k := range c.notWant {
				if _, ok := entry[k]; ok {
					t.Errorf("unexpected key %q in entry", k)
				}
			}
		})
	}
}
