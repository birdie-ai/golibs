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
						Entity: "feedbacks",
						Aggs:   dql.Aggs{},
					},
				},
			},
		},
		{
			name: "multiple aggs",
			in: `SEARCH feedbacks AGGS {
				labels: terms("labels"),
				"string name": date_histogram("posted_at"),
				sample_id: terms("sample_id") {
					nested: terms("labels")
				},
			};`,
			out: dql.Program{
				Stmts: dql.Stmts{
					{
						Entity: "feedbacks",
						Aggs: dql.Aggs{
							"labels": {
								Name: "labels",
								Func: dql.NewFncallExpr("terms", dql.NewStringExpr("labels")),
							},
							"string name": {
								Name: "string name",
								Func: dql.NewFncallExpr("date_histogram", dql.NewStringExpr("posted_at")),
							},
							"sample_id": {
								Name: "sample_id",
								Func: dql.NewFncallExpr("terms", dql.NewStringExpr("sample_id")),
								Children: dql.Aggs{
									"nested": {
										Name: "nested",
										Func: dql.NewFncallExpr("terms", dql.NewStringExpr("labels")),
									},
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
