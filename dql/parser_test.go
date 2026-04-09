package dql_test

import (
	"errors"
	"testing"

	"github.com/birdie-ai/golibs/dql"
	"github.com/google/go-cmp/cmp"
)

func TestParser(t *testing.T) {
	type testcase struct {
		name string
		in   string
		out  dql.Program
		err  error
	}

	for _, tc := range []testcase{
		{
			name: "minimal stmt",
			in:   `SEARCH feedbacks;`, // valid stmt
			out: dql.Program{
				Stmts: dql.Stmts{
					{
						Entity: "feedbacks",
					},
				},
			},
		},
		{
			name: "stmt with columns",
			in:   `SEARCH feedbacks id, UPPER(text);`,
			out: dql.Program{
				Stmts: dql.Stmts{
					{
						Entity: "feedbacks",
						Fields: []dql.Expr{
							dql.NewVarExpr("id"),
							dql.NewFncallExpr("UPPER", dql.NewVarExpr("text")),
						},
					},
				},
			},
		},
		// TODO(i4k): test columns with literals
		{
			name: "stmt with simple WHERE clause",
			in:   `SEARCH feedbacks id, UPPER(text) WHERE id=1 AND text="value";`,
			out: dql.Program{
				Stmts: dql.Stmts{
					{
						Entity: "feedbacks",
						Fields: []dql.Expr{
							dql.NewVarExpr("id"),
							dql.NewFncallExpr("UPPER", dql.NewVarExpr("text")),
						},
						Where: map[string]dql.Expr{
							"id":   dql.NewNumberExpr(1),
							"text": dql.NewStringExpr("value"),
						},
					},
				},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := dql.Parse(tc.in)
			if !errors.Is(err, tc.err) {
				t.Fatalf("err mismatch: [%v] != [%v]", err, tc.err)
			}
			if tc.err != nil {
				return
			}
			if diff := cmp.Diff(got, tc.out); diff != "" {
				t.Fatal(diff)
			}
		})
	}
}
