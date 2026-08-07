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
		want []dql.StaticPath
	}{
		{

			name: "no fields",
			in:   "SEARCH feedbacks;",
			want: []dql.StaticPath{},
		},
		{

			name: "selected fields",
			in:   "SEARCH feedbacks id,text;",
			want: []dql.StaticPath{
				{"id"},
				{"text"},
			},
		},
		{

			name: "selected custom_fields",
			in:   "SEARCH feedbacks custom_fields.a.b;",
			want: []dql.StaticPath{
				{"custom_fields", "a", "b"},
			},
		},
		{

			name: "order by fields",
			in:   "SEARCH feedbacks id ORDER BY posted_at DESC;",
			want: []dql.StaticPath{
				{"id"},
				{"posted_at"},
			},
		},
		{
			name: "function call",
			in:   "SEARCH feedbacks a, fn(b, c, d);",
			want: []dql.StaticPath{
				{"a"},
				{"b"},
				{"c"},
				{"d"},
			},
		},
		{
			name: "nested function call",
			in:   "SEARCH feedbacks a, fn2(fn1(b), c);",
			want: []dql.StaticPath{
				{"a"},
				{"b"},
				{"c"},
			},
		},
		{
			name: "function call with select",
			in:   "SEARCH feedbacks fn(custom_fields).key;",
			want: []dql.StaticPath{
				{"custom_fields"},
			},
		},
		{
			name: "nested function call with select",
			in:   "SEARCH feedbacks fn(fn2(a).c, b).d, e;",
			want: []dql.StaticPath{
				{"a"},
				{"b"},
				{"e"},
			},
		},
		{
			name: "where fields",
			in: `
			SEARCH feedbacks
			WHERE id="1" AND
				custom_fields.text="x";`,
			want: []dql.StaticPath{
				{"id"},
				{"custom_fields", "text"},
			},
		},
		{
			name: "nested where fields",
			in: `SEARCH feedbacks id WHERE {
		    "$and": [
				{
					"posted_at": {
						"$gte": "2026-01-01T00:00:00Z",
						"$lt": "2027-01-01T00:00:00Z"
					}
				}
			]
			};`,
			want: []dql.StaticPath{
				{"id"},
				{"posted_at"},
			},
		},
		{
			name: "where fields rhs",
			in: `
			SEARCH feedbacks
			WHERE custom_fields.a=custom_fields.b AND 
				c=fn(custom_fields.d).e;`,
			want: []dql.StaticPath{
				{"custom_fields", "a"},
				{"custom_fields", "b"},
				{"c"},
				{"custom_fields", "d"},
			},
		},
		{
			name: "nested where fields rhs",
			in: `
			SEARCH feedbacks WHERE {
		    "$and": [
				{
					"posted_at": {
						"$gte": custom_fields.dt_start,
						"$lt": custom_fields.dt_end
					}
				},
				{
					"text": {
						"$in": [custom_fields.text_allow_list]
					}
				},
				{
					"custom_fields.category": {
						"$in": ["category a"]
					}
				}
			]
			};`,
			want: []dql.StaticPath{
				{"posted_at"},
				{"custom_fields", "dt_start"},
				{"custom_fields", "dt_end"},
				{"text"},
				{"custom_fields", "text_allow_list"},
				{"custom_fields", "category"},
			},
		},
		{
			name: "aggs",
			in: `
			SEARCH feedbacks AGGS {
				by_category: terms(category) LIMIT 10,
				by_month: date_histogram(custom_fields.some_date) {
					by_labels: terms("labels") LIMIT 3
				},
			};
			`,
			want: []dql.StaticPath{
				{"category"},
				{"custom_fields", "some_date"},
				// TODO(Gu): do we want to support this? How to disambiguate?
				// If the agg accepts constant inputs, we can't parse this.
				// {"labels"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := dql.Parse(tt.in)
			if err != nil {
				t.Fatalf("parsing test case input: %v", err)
			}

			for _, stmt := range p.Stmts {
				got := dql.CollectFields(stmt)
				orderOpt := cmpopts.SortSlices(func(a, b dql.StaticPath) bool {
					return a.String() < b.String()
				})
				if diff := cmp.Diff(tt.want, got, orderOpt); diff != "" {
					t.Fatalf("expected - got +:\n%v", diff)
				}
			}

		})
	}
}
