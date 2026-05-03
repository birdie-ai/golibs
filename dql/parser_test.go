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
			name: "exists query",
			in: `SEARCH orders id WHERE {
				"$and": [
					{"$exists": "some.field"},
					{"other": 1},
					{
						"$or": [
							{"other": 1},
							{"$exists": "some.thing"}
						]
					}
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
									LHS: dql.Path("some", "field"),
									OP:  dql.Exists,
								},
								{
									LHS: dql.Path("other"),
									RHS: dql.NewNumberExpr(1),
									OP:  dql.Eq,
								},
								{
									Type: dql.OR,
									Children: []*dql.QueryExpr{
										{
											LHS: dql.Path("other"),
											RHS: dql.NewNumberExpr(1),
											OP:  dql.Eq,
										},
										{
											LHS: dql.Path("some", "thing"),
											OP:  dql.Exists,
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

			name: "$missing query",
			in: `SEARCH orders id WHERE {
				"$and": [
					{"$missing": "some.field"},
					{"other": 1},
					{
						"$or": [
							{"other": 1},
							{"$missing": "some.thing"}
						]
					}
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
									LHS: dql.Path("some", "field"),
									OP:  dql.Missing,
								},
								{
									LHS: dql.Path("other"),
									RHS: dql.NewNumberExpr(1),
									OP:  dql.Eq,
								},
								{
									Type: dql.OR,
									Children: []*dql.QueryExpr{
										{
											LHS: dql.Path("other"),
											RHS: dql.NewNumberExpr(1),
											OP:  dql.Eq,
										},
										{
											LHS: dql.Path("some", "thing"),
											OP:  dql.Missing,
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
			name: "lists in RHS become OP=In",
			in: `SEARCH orders id WHERE {
				"$and": [
					{"id": [1, 2, 3]}
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
									LHS: dql.Path("id"),
									OP:  dql.In,
									RHS: dql.NewListExpr([]dql.Expr{
										dql.NewNumberExpr(1),
										dql.NewNumberExpr(2),
										dql.NewNumberExpr(3),
									}),
								},
							},
						},
					},
				},
			},
		},
		{
			name: "explicit op=$in",
			in: `SEARCH orders id WHERE {
				"$and": [
					{"id": {
						"$in": myvar.docs
					}}
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
									LHS: dql.Path("id"),
									OP:  dql.In,
									RHS: dql.NewPathExpr(dql.NewVarExpr("myvar"), dql.NewFieldStep("docs")),
								},
							},
						},
					},
				},
			},
		},
		{
			name: "stmt with NOT and advanced legacy query",
			in: `SEARCH orders id WHERE {
				"$not": [
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
							Type: dql.NOT,
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
			name: "stmt with range individual range queries",
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
									OP:  dql.Range,
									Lower: dql.Bound{
										Set: true,
										OP:  dql.Gte,
										Val: dql.NewStringExpr("2022-08-12T15:30:00Z"),
									},
								},
								{
									LHS: dql.Path("posted_at"),
									OP:  dql.Range,
									Upper: dql.Bound{
										Set: true,
										OP:  dql.Lt,
										Val: dql.NewStringExpr("2026-08-12T15:30:00Z"),
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
        					"$gte": "2022-08-12T15:30:00Z",
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
									OP:  dql.Range,
									Lower: dql.Bound{
										Set: true,
										OP:  dql.Gte,
										Val: dql.NewStringExpr("2022-08-12T15:30:00Z"),
									},
									Upper: dql.Bound{
										Set: true,
										OP:  dql.Lt,
										Val: dql.NewStringExpr("2026-08-12T15:30:00Z"),
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "WITH CURSOR",
			in:   `SEARCH feedbacks LIMIT 10 WITH CURSOR;`,
			out: dql.Program{
				Stmts: dql.Stmts{
					{
						Entity:     "feedbacks",
						Limit:      10,
						WithCursor: true,
					},
				},
			},
		},
		{
			name: "ORDER BY field",
			in:   `SEARCH feedbacks LIMIT 10 ORDER BY posted_at WITH CURSOR;`,
			out: dql.Program{
				Stmts: dql.Stmts{
					{
						Entity:     "feedbacks",
						Limit:      10,
						WithCursor: true,
						OrderBy: []dql.OrderBy{
							{Field: dql.Path("posted_at")},
						},
					},
				},
			},
		},
		{
			name: "ORDER BY field DESC",
			in:   `SEARCH feedbacks LIMIT 10 ORDER BY posted_at DESC WITH CURSOR;`,
			out: dql.Program{
				Stmts: dql.Stmts{
					{
						Entity:     "feedbacks",
						Limit:      10,
						WithCursor: true,
						OrderBy: []dql.OrderBy{
							{Field: dql.Path("posted_at"), Sort: dql.DESC},
						},
					},
				},
			},
		},
		{
			name: "ORDER BY field with path",
			in:   `SEARCH orders LIMIT 10 ORDER BY feedbacks.posted_at DESC WITH CURSOR;`,
			out: dql.Program{
				Stmts: dql.Stmts{
					{
						Entity:     "orders",
						Limit:      10,
						WithCursor: true,
						OrderBy: []dql.OrderBy{
							{Field: dql.Path("feedbacks", "posted_at"), Sort: dql.DESC},
						},
					},
				},
			},
		},
		{
			name: "multiple ORDER BY clauses",
			in:   `SEARCH orders LIMIT 10 ORDER BY feedbacks.posted_at DESC, id WITH CURSOR;`,
			out: dql.Program{
				Stmts: dql.Stmts{
					{
						Entity:     "orders",
						Limit:      10,
						WithCursor: true,
						OrderBy: []dql.OrderBy{
							{Field: dql.Path("feedbacks", "posted_at"), Sort: dql.DESC},
							{Field: dql.Path("id")},
						},
					},
				},
			},
		},
		{
			name: "multiple ORDER BY clauses with direction",
			in:   `SEARCH orders LIMIT 10 ORDER BY feedbacks.posted_at DESC, id DESC WITH CURSOR;`,
			out: dql.Program{
				Stmts: dql.Stmts{
					{
						Entity:     "orders",
						Limit:      10,
						WithCursor: true,
						OrderBy: []dql.OrderBy{
							{Field: dql.Path("feedbacks", "posted_at"), Sort: dql.DESC},
							{Field: dql.Path("id"), Sort: dql.DESC},
						},
					},
				},
			},
		},
		{
			name: "WITH CURSOR WITHOUT LIMIT is an error",
			in:   `SEARCH feedbacks WITH CURSOR;`,
			err:  dql.ErrSyntax,
		},
		{
			name: "AFTER stringExpr",
			in:   `SEARCH feedbacks LIMIT 10 AFTER "test";`,
			out: dql.Program{
				Stmts: dql.Stmts{
					{
						Entity: "feedbacks",
						Limit:  10,
						After:  dql.NewStringExpr("test"),
					},
				},
			},
		},
		{
			name: "AFTER variable",
			in:   `SEARCH feedbacks LIMIT 10 AFTER first_page.next_cursor;`,
			out: dql.Program{
				Stmts: dql.Stmts{
					{
						Entity: "feedbacks",
						Limit:  10,
						After:  dql.NewPathExpr(dql.NewVarExpr("first_page"), dql.NewFieldStep("next_cursor")),
					},
				},
			},
		},
		{
			name: "AFTER WITHOUT LIMIT is an error",
			in:   `SEARCH feedbacks AFTER "test";`,
			err:  dql.ErrSyntax,
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

func TestReturn(t *testing.T) {
	type testcase struct {
		name string
		in   string
		out  dql.Program
		err  error
	}

	for _, tc := range []testcase{
		{
			name: "return literal",
			in:   `RETURN 1;`,
			out: dql.Program{
				Return: dql.Return{
					Expr: dql.NewNumberExpr(1),
				},
			},
		},
		{
			name: "return literal with format",
			in:   `RETURN format=json 1;`,
			out: dql.Program{
				Return: dql.Return{
					Format: "json",
					Expr:   dql.NewNumberExpr(1),
				},
			},
		},
		{
			name: "return empty object",
			in:   `RETURN {};`,
			out: dql.Program{
				Return: dql.Return{
					Expr: dql.NewObjectExpr(map[string]dql.Expr{}),
				},
			},
		},
		{
			name: "return object",
			in: `RETURN {
				"number": 1,
				"str": "some string",
				"list": [1, 2, 3],
				"obj": {
					"a": "a",
					"b": "b"
				}
			};`,
			out: dql.Program{
				Return: dql.Return{
					Expr: dql.NewObjectExpr(map[string]dql.Expr{
						"number": dql.NewNumberExpr(1),
						"str":    dql.NewStringExpr("some string"),
						"list": dql.NewListExpr([]dql.Expr{
							dql.NewNumberExpr(1),
							dql.NewNumberExpr(2),
							dql.NewNumberExpr(3),
						}),
						"obj": dql.NewObjectExpr(map[string]dql.Expr{
							"a": dql.NewStringExpr("a"),
							"b": dql.NewStringExpr("b"),
						}),
					}),
				},
			},
		},
		{
			name: "return funcall",
			in:   `RETURN myFunc(1, "test");`,
			out: dql.Program{
				Return: dql.Return{
					Expr: dql.NewFncallExpr("myFunc", dql.NewNumberExpr(1), dql.NewStringExpr("test")),
				},
			},
		},
		{
			name: "return mixed",
			in:   `RETURN myFunc([stmt1.docs, stmt2.docs], stmt3.aggs, {"a": stmt4.aggs});`,
			out: dql.Program{
				Return: dql.Return{
					Expr: dql.NewFncallExpr("myFunc",
						dql.NewListExpr([]dql.Expr{
							dql.NewPathExpr(dql.NewVarExpr("stmt1"), dql.NewFieldStep("docs")),
							dql.NewPathExpr(dql.NewVarExpr("stmt2"), dql.NewFieldStep("docs")),
						}),
						dql.NewPathExpr(dql.NewVarExpr("stmt3"), dql.NewFieldStep("aggs")),
						dql.NewObjectExpr(map[string]dql.Expr{
							"a": dql.NewPathExpr(dql.NewVarExpr("stmt4"), dql.NewFieldStep("aggs")),
						}),
					),
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
