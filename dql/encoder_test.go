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
		name   string
		in     dql.Program
		out    string
		shape  string
		values []dql.Expr
		err    error
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
			out:   `SEARCH test LIMIT 0;`,
			shape: `SEARCH test LIMIT 0;`,
		},
		{
			name: "stmt with only columns",
			in: dql.Program{
				Stmts: dql.Stmts{
					{
						Entity: "test",
						Fields: []dql.Expr{
							dql.NewVarExpr("id"),
							dql.NewPathExpr(dql.NewVarExpr("feedbacks"), dql.NewFieldStep("text")),
							dql.NewStringExpr("test"),
							dql.NewBoolExpr(false),
							dql.NewListExpr([]dql.Expr{dql.NewStringExpr("test")}),
						},
					},
				},
			},
			out:   `SEARCH test id,feedbacks.text,"test",false,["test"] LIMIT 0;`,
			shape: `SEARCH test id,feedbacks.text,"test",false,["test"] LIMIT 0;`,
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
			out:   `SEARCH test WHERE {"text":{"$eq":"test"}} LIMIT 0;`,
			shape: `SEARCH test WHERE {"text":{"$eq":$1}} LIMIT 0;`,
			values: []dql.Expr{
				dql.NewStringExpr("test"),
			},
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
									LHS: dql.Path("release_date"),
									OP:  dql.Range,
									Lower: dql.Bound{
										Set: true,
										OP:  dql.Gte,
										Val: dql.NewStringExpr("1990-01-01"),
									},
								},
								{
									LHS: dql.Path("release_date"),
									OP:  dql.Range,
									Upper: dql.Bound{
										Set: true,
										OP:  dql.Lte,
										Val: dql.NewStringExpr("1991-01-01"),
									},
								},
								{
									Type: dql.OR,
									Children: []*dql.QueryExpr{
										{
											LHS: dql.Path("author", "name"),
											OP:  dql.Eq,
											RHS: dql.NewStringExpr("Rob Pike"),
										},
										{
											LHS: dql.Path("author", "name"),
											OP:  dql.Eq,
											RHS: dql.NewStringExpr("Ken Thompson"),
										},
										{
											LHS: dql.Path("author", "name"),
											OP:  dql.Eq,
											RHS: dql.NewPathExpr(dql.NewVarExpr("other"), dql.NewFieldStep("name")),
										},
										{
											LHS: dql.Path("release_date"),
											OP:  dql.Range,
											Lower: dql.Bound{
												Set: true,
												OP:  dql.Gte,
												Val: dql.NewStringExpr("1990-01-01"),
											},
											Upper: dql.Bound{
												Set: true,
												OP:  dql.Lte,
												Val: dql.NewStringExpr("1991-01-01"),
											},
										},
									},
								},
							},
						},
					},
				},
			},
			out:   `SEARCH operating_systems WHERE {"$and":[{"$not":[{"type":{"$eq":"unix"}}]},{"active":{"$eq":false}},{"kernel":{"$eq":"hybrid"}},{"release_date":{"$gte":"1990-01-01"}},{"release_date":{"$lte":"1991-01-01"}},{"$or":[{"author.name":{"$eq":"Rob Pike"}},{"author.name":{"$eq":"Ken Thompson"}},{"author.name":{"$eq":other.name}},{"release_date":{"$gte":"1990-01-01","$lte":"1991-01-01"}}]}]} LIMIT 0;`,
			shape: `SEARCH operating_systems WHERE {"$and":[{"$not":[{"type":{"$eq":$1}}]},{"active":{"$eq":$2}},{"kernel":{"$eq":$3}},{"release_date":{"$gte":$4}},{"release_date":{"$lte":$5}},{"$or":[{"author.name":{"$eq":$6}},{"author.name":{"$eq":$7}},{"author.name":{"$eq":$8}},{"release_date":{"$gte":$9,"$lte":$10}}]}]} LIMIT 0;`,
			values: []dql.Expr{
				dql.NewStringExpr("unix"),
				dql.NewBoolExpr(false),
				dql.NewStringExpr("hybrid"),
				dql.NewStringExpr("1990-01-01"),
				dql.NewStringExpr("1991-01-01"),
				dql.NewStringExpr("Rob Pike"),
				dql.NewStringExpr("Ken Thompson"),
				dql.NewPathExpr(dql.NewVarExpr("other"), dql.NewFieldStep("name")),
				dql.NewStringExpr("1990-01-01"),
				dql.NewStringExpr("1991-01-01"),
			},
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
			out:   `SEARCH test WHERE {"text":{"$eq":"test"}} LIMIT 100;`,
			shape: `SEARCH test WHERE {"text":{"$eq":$1}} LIMIT 100;`,
			values: []dql.Expr{
				dql.NewStringExpr("test"),
			},
		},
	} {
		// normal encoding
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			enc := dql.NewEncoder(&buf)
			err := enc.Encode(tc.in)
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

		// shape encoding
		t.Run(tc.name+"(shape)", func(t *testing.T) {
			var buf bytes.Buffer
			enc := dql.NewEncoder(&buf, dql.OnlyShape())
			err := enc.Encode(tc.in)
			if !errors.Is(err, tc.err) {
				t.Fatalf("err mismatch want [%v] != expected [%v]", tc.err, err)
				return
			}
			if tc.err != nil {
				return
			}
			if diff := cmp.Diff(buf.String(), tc.shape); diff != "" {
				t.Log(buf.String())
				t.Fatal(diff)
			}
			if diff := cmp.Diff(enc.Values(), tc.values); diff != "" {
				t.Fatal(diff)
			}
		})
	}
}
