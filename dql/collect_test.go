package dql_test

import (
	"testing"

	"github.com/birdie-ai/golibs/dql"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
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
		{

			name: "order by fields",
			in:   "SEARCH feedbacks id ORDER BY posted_at DESC;",
			want: []string{"id", "posted_at"},
		},
		{
			name: "function call",
			in:   "SEARCH feedbacks a, fn(b, c, d);",
			want: []string{"a", "b", "c", "d"},
		},
		{
			name: "nested function call",
			in:   "SEARCH feedbacks a, fn2(fn1(b), c);",
			want: []string{"a", "b", "c"},
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
				orderOpt := cmpopts.SortSlices(func(a, b string) bool {
					return a < b
				})
				if diff := cmp.Diff(got, tt.want, orderOpt); diff != "" {
					t.Fatalf("expected - got +:\n%v", diff)
				}
			}

		})
	}
}
