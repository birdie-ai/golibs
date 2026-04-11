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
						Where: &dql.Query{
							Type: dql.OR,
							Children: []*dql.Query{
								{
									Type: dql.AND,
									Children: []*dql.Query{
										{
											LHS: dql.Path("id"),
											RHS: dql.NewNumberExpr(1),
											OP:  dql.Eq,
										},
										{
											LHS: dql.Path("text"),
											RHS: dql.NewStringExpr("value"),
											OP:  dql.Eq,
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "stmt dot-traversal paths",
			in:   `SEARCH orders id, feedbacks.id, feedbacks.text WHERE feedbacks.text="value";`,
			out: dql.Program{
				Stmts: dql.Stmts{
					{
						Entity: "orders",
						Fields: []dql.Expr{
							dql.NewVarExpr("id"),
							dql.NewPathExpr(dql.NewVarExpr("feedbacks"), dql.NewFieldStep("id")),
							dql.NewPathExpr(dql.NewVarExpr("feedbacks"), dql.NewFieldStep("text")),
						},
						Where: &dql.Query{
							Type: dql.OR,
							Children: []*dql.Query{
								{
									LHS: dql.Path("feedbacks", "text"),
									RHS: dql.NewStringExpr("value"),
									OP:  dql.Eq,
								},
							},
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
