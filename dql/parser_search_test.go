package dql_test

import (
	"errors"
	"testing"

	"github.com/birdie-ai/golibs/dql"
	"github.com/google/go-cmp/cmp"
)

func TestParserSearch(t *testing.T) {
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
			name: "minimal stmt with LIMIT",
			in:   `SEARCH feedbacks LIMIT 10;`,
			out: dql.Program{
				Stmts: dql.Stmts{
					{
						Entity: "feedbacks",
						Limit:  10,
					},
				},
			},
		},
		{
			name: "multiple stmts",
			in:   `SEARCH feedbacks; SEARCH orders;`,
			out: dql.Program{
				Stmts: dql.Stmts{
					{
						Entity: "feedbacks",
					},
					{
						Entity: "orders",
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
						Where: &dql.QueryExpr{
							Type: dql.AND,
							Children: []*dql.QueryExpr{
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
		{
			name: "WHERE clause with LIMIT",
			in:   `SEARCH feedbacks id WHERE id=1 AND text="value" LIMIT 10;`,
			out: dql.Program{
				Stmts: dql.Stmts{
					{
						Entity: "feedbacks",
						Fields: []dql.Expr{
							dql.NewVarExpr("id"),
						},
						Where: &dql.QueryExpr{
							Type: dql.AND,
							Children: []*dql.QueryExpr{
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
						Limit: 10,
					},
				},
			},
		},
		{
			name: "stmt with single WHERE predicate must avoid logical predicate",
			in: `SEARCH feedbacks
					id
				 WHERE id="abc"
				 LIMIT 10;`,
			out: dql.Program{
				Stmts: dql.Stmts{
					{
						Entity: "feedbacks",
						Fields: []dql.Expr{
							dql.NewVarExpr("id"),
						},
						Where: &dql.QueryExpr{
							LHS: dql.Path("id"),
							RHS: dql.NewStringExpr("abc"),
							OP:  dql.Eq,
						},
						Limit: 10,
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
						Where: &dql.QueryExpr{
							LHS: dql.Path("feedbacks", "text"),
							RHS: dql.NewStringExpr("value"),
							OP:  dql.Eq,
						},
					},
				},
			},
		},
		{
			name: "stmt with legacy query",
			in: `SEARCH orders id WHERE {
				"$and": [
					{"feedbacks.text": "value"},
					{"other": 1}
				]
			};`,
			out: dql.Program{
				Stmts: dql.Stmts{
					{
						Entity: "orders",
						Fields: []dql.Expr{
							dql.NewVarExpr("id"),
						},
						Where: &dql.QueryExpr{
							Type: dql.AND,
							Children: []*dql.QueryExpr{
								{
									LHS: dql.Path("feedbacks", "text"),
									RHS: dql.NewStringExpr("value"),
									OP:  dql.Eq,
								},
								{
									LHS: dql.Path("other"),
									RHS: dql.NewNumberExpr(1),
									OP:  dql.Eq,
								},
							},
						},
					},
				},
			},
		},
		{
			name: "stmt with advanced legacy query",
			in: `SEARCH orders id WHERE {
				"$or": [
					{"feedbacks.text": "value"},
					{"$and": [
						{"custom_fields.abc": "test"},
						{"test": 1}
					]}
				]
			};`,
			out: dql.Program{
				Stmts: dql.Stmts{
					{
						Entity: "orders",
						Fields: []dql.Expr{
							dql.NewVarExpr("id"),
						},
						Where: &dql.QueryExpr{
							Type: dql.OR,
							Children: []*dql.QueryExpr{
								{
									LHS: dql.Path("feedbacks", "text"),
									RHS: dql.NewStringExpr("value"),
									OP:  dql.Eq,
								},
								{
									Type: dql.AND,
									Children: []*dql.QueryExpr{
										{
											LHS: dql.Path("custom_fields", "abc"),
											RHS: dql.NewStringExpr("test"),
											OP:  dql.Eq,
										},
										{
											LHS: dql.Path("test"),
											RHS: dql.NewNumberExpr(1),
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
			name: "stmt with range query",
			in: `SEARCH feedbacks id WHERE {
				"$and": [
					{
    					"posted_at": {
        					"$gte": "2022-08-12T15:30:00Z"
    					}
					},
					{
    					"posted_at": {
        					"$lt": "2026-08-12T15:30:00Z"
    					}
					}
				]
			};`,
			out: dql.Program{
				Stmts: dql.Stmts{
					{
						Entity: "feedbacks",
						Fields: []dql.Expr{
							dql.NewVarExpr("id"),
						},
						Where: &dql.QueryExpr{
							Type: dql.AND,
							Children: []*dql.QueryExpr{
								{
									LHS: dql.Path("posted_at"),
									OP:  dql.Gte,
									RHS: dql.NewStringExpr("2022-08-12T15:30:00Z"),
								},
								{
									LHS: dql.Path("posted_at"),
									OP:  dql.Lt,
									RHS: dql.NewStringExpr("2026-08-12T15:30:00Z"),
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
