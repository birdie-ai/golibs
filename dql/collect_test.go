package dql_test

import (
	"testing"

	"github.com/birdie-ai/golibs/dql"
	"github.com/google/go-cmp/cmp"
)

func TestCollect(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []string
	}{
		{

			name: "no fields",
			in:   "SEARCH feedbacks;",
			want: []string{},
		},
		{

			name: "selected fields",
			in:   "SEARCH feedbacks id,text;",
			want: []string{"id", "text"},
		},
		{

			name: "selected custom_fields",
			in:   "SEARCH feedbacks custom_fields.a.b;",
			want: []string{"custom_fields.a.b"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := dql.Parse(tt.in)
			if err != nil {
				t.Fatalf("parsing test case input: %v", err)
			}

			for _, stmt := range p.Stmts {
				got := dql.Collect(stmt)
				if diff := cmp.Diff(got, tt.want); diff != "" {
					t.Fatalf("expected - got +:\n%v", diff)
				}
			}

		})
	}
}
