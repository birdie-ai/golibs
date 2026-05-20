package dql_test

import (
	"errors"
	"testing"

	"github.com/birdie-ai/golibs/dql"
	"github.com/google/go-cmp/cmp"
)

func TestParserAggs(t *testing.T) {
	type testcase struct {
		name string
		in   string
		out  dql.Program
		err  error
	}

	for _, tc := range []testcase{
		{
			name: "empty aggs",
			in:   `SEARCH feedbacks AGGS {};`, // valid stmt
			out: dql.Program{
				Stmts: dql.Stmts{
					{
						Op:     dql.SEARCH,
						Entity: "feedbacks",
						Aggs:   dql.Aggs{},
					},
				},
			},
		},
		{
			name: "empty aggs with LIMIT",
			in:   `SEARCH feedbacks LIMIT 0 AGGS {};`, // valid stmt
			out: dql.Program{
				Stmts: dql.Stmts{
					{
						Op:     dql.SEARCH,
						Entity: "feedbacks",
						Limit:  ptr(0),
						Aggs:   dql.Aggs{},
					},
				},
			},
		},
		{
			name: "agg with LIMIT",
			in: `SEARCH feedbacks AGGS {
				by_labels: terms(labels) LIMIT 1000
			};`,
			out: dql.Program{
				Stmts: dql.Stmts{
					{
						Op:     dql.SEARCH,
						Entity: "feedbacks",
						Aggs: dql.Aggs{
							"by_labels": dql.Agg{
								Name:  "by_labels",
								Func:  dql.NewFncallExpr("terms", dql.NewVarExpr("labels")),
								Limit: ptr(1000),
							},
						},
					},
				},
			},
		},
		{
			name: "multiple aggs",
			in: `SEARCH feedbacks AGGS {
				labels: terms("labels") LIMIT 1,
				"string name": date_histogram("posted_at"),
				sample_id: terms("sample_id") LIMIT 2 {
					nested: terms("labels") LIMIT 3
				},
				by_month: date_histogram(posted_at, {"interval": "month"})
			};`,
			out: dql.Program{
				Stmts: dql.Stmts{
					{
						Op:     dql.SEARCH,
						Entity: "feedbacks",
						Aggs: dql.Aggs{
							"labels": {
								Name:  "labels",
								Func:  dql.NewFncallExpr("terms", dql.NewStringExpr("labels")),
								Limit: ptr(1),
							},
							"string name": {
								Name: "string name",
								Func: dql.NewFncallExpr("date_histogram", dql.NewStringExpr("posted_at")),
							},
							"sample_id": {
								Name:  "sample_id",
								Func:  dql.NewFncallExpr("terms", dql.NewStringExpr("sample_id")),
								Limit: ptr(2),
								Children: dql.Aggs{
									"nested": {
										Name:  "nested",
										Func:  dql.NewFncallExpr("terms", dql.NewStringExpr("labels")),
										Limit: ptr(3),
									},
								},
							},
							"by_month": {
								Name: "by_month",
								Func: dql.NewFncallExpr("date_histogram", dql.NewVarExpr("posted_at"), dql.NewObjectExpr(map[string]dql.Expr{
									"interval": dql.NewStringExpr("month"),
								})),
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
