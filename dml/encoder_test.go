package dml_test

import (
	"bytes"
	"errors"
	"testing"
	"unique"

	dml "github.com/birdie-ai/golibs/dml"
	"github.com/google/go-cmp/cmp"
)

func TestEncode(t *testing.T) {
	t.Parallel()
	type testcase struct {
		ast  dml.Stmts
		want string
		err  error
	}

	for _, tc := range []testcase{
		{
			ast: dml.Stmts{
				{},
			},
			err: dml.ErrInvalidOperation,
		},
		{
			ast: dml.Stmts{
				{},
			},
			err: dml.ErrMissingAssign,
		},
		{
			ast: dml.Stmts{
				{},
			},
			err: dml.ErrMissingEntity,
		},
		{
			ast: dml.Stmts{
				{},
			},
			err: dml.ErrMissingWhereClause,
		},
		{
			ast: dml.Stmts{
				{Op: dml.SET},
			},
			err: dml.ErrMissingEntity,
		},
		{
			ast: dml.Stmts{
				{Op: dml.SET, Entity: u("test")},
			},
			err: dml.ErrMissingAssign,
		},
		{
			ast: dml.Stmts{
				{Op: dml.SET, Entity: u("test"), Assign: dml.Assign{".": map[string]any{}}},
			},
			err: dml.ErrMissingWhereClause,
		},
		{
			ast: dml.Stmts{
				{
					Op:     dml.SET,
					Entity: u("test"),
					Assign: dml.Assign{"$a": map[string]any{}},
					Where: dml.Where{
						"id": "abc",
					},
				},
			},
			err: dml.ErrInvalidAssignKey,
		},
		{
			ast: dml.Stmts{
				{
					Op:     dml.SET,
					Entity: u("$bleh"),
					Assign: dml.Assign{"a": map[string]any{}},
					Where: dml.Where{
						"id": "abc",
					},
				},
			},
			err: dml.ErrNotIdent,
		},
		{
			ast: dml.Stmts{
				{
					Op:     dml.SET,
					Entity: u("bleh"),
					Assign: dml.Assign{"a": map[string]any{}},
					Where: dml.Where{
						"$id": "abc",
					},
				},
			},
			err: dml.ErrNotIdent,
		},
		{
			ast: dml.Stmts{
				{
					Op:     dml.SET,
					Entity: u("bleh"),
					Assign: dml.Assign{`a."incomplete`: 1},
					Where: dml.Where{
						"id": "abc",
					},
				},
			},
			err: dml.ErrInvalidAssignKey,
		},
		{
			ast: dml.Stmts{
				{
					Op:     dml.SET,
					Entity: u("bleh"),
					Assign: dml.Assign{`a.""`: 1},
					Where: dml.Where{
						"id": "abc",
					},
				},
			},
			err: dml.ErrInvalidAssignKey,
		},
		{
			ast: dml.Stmts{
				{
					Op:     dml.SET,
					Entity: u("bleh"),
					Assign: dml.Assign{`a.'test'`: 1},
					Where: dml.Where{
						"id": "abc",
					},
				},
			},
			err: dml.ErrInvalidAssignKey,
		},
		{
			ast: dml.Stmts{
				{
					Op:     dml.SET,
					Entity: u("bleh"),
					Assign: dml.Assign{`"test"`: 1},
					Where: dml.Where{
						"id": "abc",
					},
				},
			},
			err: dml.ErrInvalidAssignKey,
		},
		{
			ast: dml.Stmts{
				{
					Op:     dml.SET,
					Entity: u("bleh"),
					Assign: dml.Assign{`-test`: 1},
					Where: dml.Where{
						"id": "abc",
					},
				},
			},
			err: dml.ErrInvalidAssignKey,
		},
		{
			ast: dml.Stmts{
				{
					Op:     dml.SET,
					Entity: u("bleh"),
					Assign: dml.Assign{`..`: 1},
					Where: dml.Where{
						"id": "abc",
					},
				},
			},
			err: dml.ErrInvalidAssignKey,
		},
		{
			ast: dml.Stmts{
				{
					Op:     dml.SET,
					Entity: u("bleh"),
					Assign: dml.Assign{`test-`: 1},
					Where: dml.Where{
						"id": "abc",
					},
				},
			},
			err: dml.ErrInvalidAssignKey,
		},
		{
			ast: dml.Stmts{
				{
					Op:     dml.SET,
					Entity: u("feedbacks"),
					Assign: dml.Assign{"a": 1},
					Where: dml.Where{
						"id": 1,
					},
				},
			},
			want: "SET feedbacks a=1 WHERE id=1;",
		},
		{
			ast: dml.Stmts{
				{
					Op:     dml.SET,
					Entity: u("feedbacks"),
					Assign: dml.Assign{"a": false},
					Where: dml.Where{
						"id": false,
					},
				},
			},
			want: "SET feedbacks a=false WHERE id=false;",
		},
		{
			ast: dml.Stmts{
				{
					Op:     dml.SET,
					Entity: u("feedbacks"),
					Assign: dml.Assign{"a": map[string]string{"k1": "v1", "k2": "v2"}},
					Where: dml.Where{
						"id": "abc",
					},
				},
			},
			want: `SET feedbacks a={"k1":"v1","k2":"v2"} WHERE id="abc";`,
		},
		{
			ast: dml.Stmts{
				{
					Op:     dml.SET,
					Entity: u("feedbacks"),
					Assign: dml.Assign{
						"a.b.c": []string{"a", "b", "c"},
					},
					Where: dml.Where{
						"id": "abc",
					},
				},
			},
			want: `SET feedbacks a.b.c=["a","b","c"] WHERE id="abc";`,
		},
		{
			ast: dml.Stmts{
				{
					Op:     dml.SET,
					Entity: u("organizations"),
					Assign: dml.Assign{
						"name":   "some org",
						"config": map[string]any{},
					},
					Where: dml.Where{
						"id": "abc",
					},
				},
			},
			want: `SET organizations config={},name="some org" WHERE id="abc";`,
		},
		{
			ast: dml.Stmts{
				{
					Op:     dml.SET,
					Entity: u("organizations"),
					Assign: dml.Assign{
						".": map[string]string{
							"name": "some org",
							"test": "abc",
						},
					},
					Where: dml.Where{
						"id": "abc",
					},
				},
			},
			want: `SET organizations .={"name":"some org","test":"abc"} WHERE id="abc";`,
		},
		{
			ast: dml.Stmts{
				{
					Op:     dml.SET,
					Entity: u("feedbacks"),
					Assign: dml.Assign{"a": 1},
					Where: dml.Where{
						"id": 1,
					},
				},
				{
					Op:     dml.SET,
					Entity: u("organizations"),
					Assign: dml.Assign{"abc": 1},
					Where: dml.Where{
						"id": "abc",
					},
				},
			},
			// For now there's no "pretty" format support.
			want: `SET feedbacks a=1 WHERE id=1;SET organizations abc=1 WHERE id="abc";`,
		},
		{
			ast: dml.Stmts{
				{
					Op:     dml.SET,
					Entity: u("feedbacks"),
					Assign: dml.Assign{"some-field": 1},
					Where: dml.Where{
						"id": 1,
					},
				},
			},
			want: "SET feedbacks some-field=1 WHERE id=1;",
		},
		{
			ast: dml.Stmts{
				{
					Op:     dml.SET,
					Entity: u("feedbacks"),
					Assign: dml.Assign{"abc.some-field": 1},
					Where: dml.Where{
						"id": 1,
					},
				},
			},
			want: "SET feedbacks abc.some-field=1 WHERE id=1;",
		},
		{
			ast: dml.Stmts{
				{
					Op:     dml.SET,
					Entity: u("feedbacks"),
					Assign: dml.Assign{"some-field.some-other-field": 1},
					Where: dml.Where{
						"id": 1,
					},
				},
			},
			want: "SET feedbacks some-field.some-other-field=1 WHERE id=1;",
		},
		{
			ast: dml.Stmts{
				{
					Op:     dml.SET,
					Entity: u("feedbacks"),
					Assign: dml.Assign{`a."some_field"`: 1},
					Where: dml.Where{
						"id": 1,
					},
				},
			},
			want: `SET feedbacks a."some_field"=1 WHERE id=1;`,
		},
		{
			ast: dml.Stmts{
				{
					Op:     dml.SET,
					Entity: u("feedbacks"),
					Assign: dml.Assign{"a.\"some-other-field\"": 1},
					Where: dml.Where{
						"id": 1,
					},
				},
			},
			want: `SET feedbacks a."some-other-field"=1 WHERE id=1;`,
		},
		{
			ast: dml.Stmts{
				{
					Op:     dml.SET,
					Entity: u("feedbacks"),
					Assign: dml.Assign{`a."some field".test."other field"`: 1},
					Where: dml.Where{
						"id": 1,
					},
				},
			},
			want: `SET feedbacks a."some field".test."other field"=1 WHERE id=1;`,
		},
		{
			ast: dml.Stmts{
				{
					Op:     dml.SET,
					Entity: u("feedbacks"),
					Assign: dml.Assign{
						"a.b.c": []string{"a", "b", "c"},
					},
					Where: dml.Where{
						"id":     "abc",
						"org_id": "xyz",
					},
				},
			},
			want: `SET feedbacks a.b.c=["a","b","c"] WHERE {"id":"abc","org_id":"xyz"};`,
		},
		{
			ast: dml.Stmts{
				{
					Op:     dml.SET,
					Entity: u("feedbacks"),
					Assign: dml.Assign{
						"a.b.c": dml.Append[string]{Values: []string{"a", "b", "c"}},
					},
					Where: dml.Where{
						"id":     "abc",
						"org_id": "xyz",
					},
				},
			},
			want: `SET feedbacks a.b.c=...["a","b","c"] WHERE {"id":"abc","org_id":"xyz"};`,
		},
		{
			ast: dml.Stmts{
				{
					Op:     dml.SET,
					Entity: u("feedbacks"),
					Assign: dml.Assign{
						"a.b.c": dml.Prepend[string]{Values: []string{"a", "b", "c"}},
					},
					Where: dml.Where{
						"id":     "abc",
						"org_id": "xyz",
					},
				},
			},
			want: `SET feedbacks a.b.c=["a","b","c"]... WHERE {"id":"abc","org_id":"xyz"};`,
		},
		{
			ast: dml.Stmts{
				{
					Op:     dml.SET,
					Entity: u("feedbacks"),
					Assign: dml.Assign{
						"d":       1,
						"i.j.k":   dml.Append[float64]{Values: []float64{1, 2, 3}},
						"s.t.r.a": dml.Prepend[string]{Values: []string{"a", "b", "c"}},
					},
					Where: dml.Where{
						"id":     "abc",
						"org_id": "xyz",
					},
				},
			},
			want: `SET feedbacks d=1,i.j.k=...[1,2,3],s.t.r.a=["a","b","c"]... WHERE {"id":"abc","org_id":"xyz"};`,
		},
		{
			ast: dml.Stmts{
				{
					Op:     dml.SET,
					Entity: u("feedbacks"),
					Assign: dml.Assign{
						"a.b.c": dml.Append[float64]{Values: []float64{}},
					},
					Where: dml.Where{
						"id":     "abc",
						"org_id": "xyz",
					},
				},
			},
			err: dml.ErrMissingArrayValues,
		},
		{
			ast: dml.Stmts{
				{
					Op:     dml.SET,
					Entity: u("feedbacks"),
					Assign: dml.Assign{
						"a.b.c": dml.Prepend[float64]{Values: []float64{}},
					},
					Where: dml.Where{
						"id":     "abc",
						"org_id": "xyz",
					},
				},
			},
			err: dml.ErrMissingArrayValues,
		},
		{
			ast: dml.Stmts{
				{
					Op:     dml.DELETE,
					Entity: u("feedbacks"),
					Assign: dml.Assign{
						"obj": dml.KeyFilter{Keys: []string{"a"}},
					},
					Where: dml.Where{
						"id":     "abc",
						"org_id": "xyz",
					},
				},
			},
			want: `DELETE feedbacks obj.a WHERE {"id":"abc","org_id":"xyz"};`,
		},
		{
			ast: dml.Stmts{
				{
					Op:     dml.DELETE,
					Entity: u("feedbacks"),
					Assign: dml.Assign{
						"obj": dml.KeyFilter{Keys: []string{"a", "b"}},
					},
					Where: dml.Where{
						"id":     "abc",
						"org_id": "xyz",
					},
				},
			},
			want: `DELETE feedbacks k IN obj WHERE k IN ["a","b"] WHERE {"id":"abc","org_id":"xyz"};`,
		},
		{
			ast: dml.Stmts{
				{
					Op:     dml.DELETE,
					Entity: u("feedbacks"),
					Assign: dml.Assign{
						"labels": dml.ValueFilter[string]{Values: []string{"label-1"}},
					},
					Where: dml.Where{
						"id":     "abc",
						"org_id": "xyz",
					},
				},
			},
			want: `DELETE feedbacks _,v IN labels WHERE v="label-1" WHERE {"id":"abc","org_id":"xyz"};`,
		},
		{
			ast: dml.Stmts{
				{
					Op:     dml.DELETE,
					Entity: u("feedbacks"),
					Assign: dml.Assign{
						"custom_fields": dml.KeyValueFilter[string]{Key: "country", Values: []string{"us"}},
					},
					Where: dml.Where{
						"id":     "abc",
						"org_id": "xyz",
					},
				},
			},
			want: `DELETE feedbacks k,v IN custom_fields WHERE k="country" AND v="us" WHERE {"id":"abc","org_id":"xyz"};`,
		},
	} {
		var buf bytes.Buffer
		err := dml.Encode(&buf, tc.ast)
		if !errors.Is(err, tc.err) {
			t.Fatal(err)
		}
		if err != nil {
			continue
		}
		got := buf.String()
		if diff := cmp.Diff(tc.want, got); diff != "" {
			t.Fatalf("got [+], want [-]: %s", diff)
		}
	}
}

func u(s string) unique.Handle[string] {
	return unique.Make(s)
}
