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
			err: dml.ErrNotIdent,
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
