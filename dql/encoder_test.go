package dql_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/birdie-ai/golibs/dql"
	"github.com/google/go-cmp/cmp"
)

func TestEncoder(t *testing.T) {
	t.Parallel()
	type testcase struct {
		name string
		in   dql.Program
		out  string
		err  error
	}

	for _, tc := range []testcase{
		{
			name: "empty program",
			in:   dql.Program{},
		},
		{
			name: "mostly empty stmt",
			in: dql.Program{
				Stmts: dql.Stmts{
					{
						Entity: "test",
					},
				},
			},
			out: `SEARCH test LIMIT 0;`,
		},
		{
			name: "stmt with only columns",
			in: dql.Program{
				Stmts: dql.Stmts{
					{
						Entity: "test",
						Fields: []dql.Expr{
							dql.NewVarExpr("id"),
							dql.NewStringExpr("test"),
							dql.NewBoolExpr(false),
							dql.NewListExpr([]dql.Expr{dql.NewStringExpr("test")}),
						},
					},
				},
			},
			out: `SEARCH test id,"test",false,["test"] LIMIT 0;`,
		},
		{
			name: "stmt with simple WHERE",
			in: dql.Program{
				Stmts: dql.Stmts{
					{
						Entity: "test",
						Where: &dql.QueryExpr{
							LHS: dql.Path("text"),
							OP:  dql.Eq,
							RHS: dql.NewStringExpr("test"),
						},
					},
				},
			},
			out: `SEARCH test WHERE {"text":"test"} LIMIT 0;`,
		},
		{
			name: "stmt with advanced WHERE",
			in: dql.Program{
				Stmts: dql.Stmts{
					{
						Entity: "operating_systems",
						Where: &dql.QueryExpr{
							Type: dql.AND,
							Children: []*dql.QueryExpr{
								{
									Type: dql.NOT,
									Children: []*dql.QueryExpr{
										{
											LHS: dql.Path("type"),
											OP:  dql.Eq,
											RHS: dql.NewStringExpr("unix"),
										},
									},
								},
								{
									LHS: dql.Path("active"),
									OP:  dql.Eq,
									RHS: dql.NewBoolExpr(false),
								},
								{
									LHS: dql.Path("kernel"),
									OP:  dql.Eq,
									RHS: dql.NewStringExpr("hybrid"),
								},
								{
									Type: dql.OR,
									Children: []*dql.QueryExpr{
										{
											LHS: dql.Path("author"),
											OP:  dql.Eq,
											RHS: dql.NewStringExpr("Rob Pike"),
										},
										{
											LHS: dql.Path("author"),
											OP:  dql.Eq,
											RHS: dql.NewStringExpr("Ken Thompson"),
										},
									},
								},
							},
						},
					},
				},
			},
			out: `SEARCH operating_systems WHERE {"$and":[{"$not":[{"type":"unix"}]},{"active":false},{"kernel":"hybrid"},{"$or":[{"author":"Rob Pike"},{"author":"Ken Thompson"}]}]} LIMIT 0;`,
		},
		{
			name: "stmt with LIMIT",
			in: dql.Program{
				Stmts: dql.Stmts{
					{
						Entity: "test",
						Where: &dql.QueryExpr{
							LHS: dql.Path("text"),
							OP:  dql.Eq,
							RHS: dql.NewStringExpr("test"),
						},
						Limit: 100,
					},
				},
			},
			out: `SEARCH test WHERE {"text":"test"} LIMIT 100;`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			err := dql.Encode(&buf, tc.in)
			if !errors.Is(err, tc.err) {
				t.Fatalf("err mismatch want [%v] != expected [%v]", tc.err, err)
				return
			}
			if tc.err != nil {
				return
			}
			if diff := cmp.Diff(buf.String(), tc.out); diff != "" {
				t.Log(buf.String())
				t.Fatal(diff)
			}
		})
	}
}
