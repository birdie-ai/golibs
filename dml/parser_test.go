package dml_test

import (
	"errors"
	"testing"
	"unique"

	"github.com/birdie-ai/golibs/dml"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestParser(t *testing.T) {
	t.Parallel()

	type testcase struct {
		text string
		want dml.Stmts
		err  error
	}

	for _, tc := range []testcase{
		{
			text: "",
		},
		{
			text: "SET",
			err:  dml.ErrSyntax,
		},
		{
			text: "DELETE",
			err:  dml.ErrSyntax,
		},
		{
			text: "    SET",
			err:  dml.ErrSyntax,
		},
		{
			text: "    DELETE",
			err:  dml.ErrSyntax,
		},
		{
			text: "    SET         ",
			err:  dml.ErrSyntax,
		},
		{
			text: "    DELETE     ",
			err:  dml.ErrSyntax,
		},
		{
			text: `SET feedbacks .={} WHERE ["1", "2"];`,
			err:  dml.ErrSyntax,
		},
		{
			text: `SET feedbacks .=1 WHERE id=1;`,
			err:  dml.ErrSyntax,
		},
		{
			text: `SET feedbacks .= ["a", "b"] WHERE id=1;`,
			err:  dml.ErrSyntax,
		},
		{
			text: `SET feedbacks a= [invalid syntax] WHERE id=1;`,
			err:  dml.ErrSyntax,
		},
		{
			text: `SET feedbacks a.b.c = [invalid syntax] WHERE id=1;`,
			err:  dml.ErrSyntax,
		},
		{
			text: `SET feedbacks .={} WHERE {invalid JSON};`,
			err:  dml.ErrSyntax,
		},
		{
			text: `SET feedbacks .={} WHERE {};`, // expect WHERE object with values
			err:  dml.ErrSyntax,
		},
		{
			text: `SET feedbacks .={} WHERE {"not valid id": 1};`, // expect WHERE object identifier keys
			err:  dml.ErrSyntax,
		},
		{
			text: `SET feedbacks a=1 WHERE id=[invalid syntax];`,
			err:  dml.ErrSyntax,
		},
		{
			// expect root assign to have object with ident keys.
			text: `SET feedbacks .={"not valid key": "test"} WHERE id=1;`,
			err:  dml.ErrSyntax,
		},
		{
			text: `SET feedbacks -abc={} WHERE id=1;`,
			err:  dml.ErrSyntax,
		},
		{
			text: `SET feedbacks abc-={} WHERE id=1;`,
			err:  dml.ErrSyntax,
		},
		{
			text: `SET feedbacks custom_fields.abc- ={} WHERE id=1;`,
			err:  dml.ErrSyntax,
		},
		{
			text: `SET feedbacks - ={} WHERE id=1;`,
			err:  dml.ErrSyntax,
		},
		{
			text: `SET feedbacks .. ={} WHERE id=1;`,
			err:  dml.ErrSyntax,
		},
		{
			text: `SET feedbacks a..b ={} WHERE id=1;`,
			err:  dml.ErrSyntax,
		},
		{
			text: `SET feedbacks .={} WHERE {"id": 1};`,
			want: dml.Stmts{
				{
					Op:     dml.SET,
					Entity: u("feedbacks"),
					Assign: dml.Assign{".": map[string]any{}},
					Where: dml.Where{
						"id": 1.0,
					},
				},
			},
		},
		{
			text: `SET feedbacks .={} WHERE id=1;`,
			want: dml.Stmts{
				{
					Op:     dml.SET,
					Entity: u("feedbacks"),
					Assign: dml.Assign{".": map[string]any{}},
					Where: dml.Where{
						"id": 1.0,
					},
				},
			},
		},
		{
			text: `SET feedbacks .={} WHERE id    =   1;`,
			want: dml.Stmts{
				{
					Op:     dml.SET,
					Entity: u("feedbacks"),
					Assign: dml.Assign{".": map[string]any{}},
					Where: dml.Where{
						"id": 1.0,
					},
				},
			},
		},
		{
			text: `
			SET
			feedbacks
			.    =      {}
		 WHERE
		 id
			=
			1
			;`,
			want: dml.Stmts{
				{
					Op:     dml.SET,
					Entity: u("feedbacks"),
					Assign: dml.Assign{".": map[string]any{}},
					Where: dml.Where{
						"id": 1.0,
					},
				},
			},
		},
		{
			text: `SET feedbacks some-field=1 WHERE id = 1;`,
			want: dml.Stmts{
				{
					Op:     dml.SET,
					Entity: u("feedbacks"),
					Assign: dml.Assign{
						"some-field": 1.0,
					},
					Where: dml.Where{
						"id": 1.0,
					},
				},
			},
		},
		{
			text: `SET feedbacks-2025-01-01 my-field=1 WHERE my-id = 1;`,
			want: dml.Stmts{
				{
					Op:     dml.SET,
					Entity: u("feedbacks-2025-01-01"),
					Assign: dml.Assign{
						"my-field": 1.0,
					},
					Where: dml.Where{
						"my-id": 1.0,
					},
				},
			},
		},
		{
			text: `SET feedbacks custom_fields={} WHERE id    =   1;`,
			want: dml.Stmts{
				{
					Op:     dml.SET,
					Entity: u("feedbacks"),
					Assign: dml.Assign{"custom_fields": map[string]any{}},
					Where: dml.Where{
						"id": 1.0,
					},
				},
			},
		},
		{
			text: `SET feedbacks custom-fields={} WHERE id = 1;`,
			want: dml.Stmts{
				{
					Op:     dml.SET,
					Entity: u("feedbacks"),
					Assign: dml.Assign{"custom-fields": map[string]any{}},
					Where: dml.Where{
						"id": 1.0,
					},
				},
			},
		},
		{
			text: `SET feedbacks a=1 WHERE id=1 and test=2;`,
			want: dml.Stmts{
				{
					Op:     dml.SET,
					Entity: u("feedbacks"),
					Assign: dml.Assign{"a": 1.0},
					Where: dml.Where{
						"id":   1.0,
						"test": 2.0,
					},
				},
			},
		},
		{
			text: `SET feedbacks a=1 WHERE a=1 and b=2 and c=3;`,
			want: dml.Stmts{
				{
					Op:     dml.SET,
					Entity: u("feedbacks"),
					Assign: dml.Assign{"a": 1.0},
					Where: dml.Where{
						"a": 1.0,
						"b": 2.0,
						"c": 3.0,
					},
				},
			},
		},
		{
			text: `SET feedbacks a=1 WHERE {"a": 1} and {"b": 2};`,
			want: dml.Stmts{
				{
					Op:     dml.SET,
					Entity: u("feedbacks"),
					Assign: dml.Assign{"a": 1.0},
					Where: dml.Where{
						"a": 1.0,
						"b": 2.0,
					},
				},
			},
		},
		{
			text: `SET feedbacks custom_fields={"a": 1, "b":"abc"} WHERE id    =   1;`,
			want: dml.Stmts{
				{
					Op:     dml.SET,
					Entity: u("feedbacks"),
					Assign: dml.Assign{
						"custom_fields": map[string]any{
							"a": 1.0,
							"b": "abc",
						},
					},
					Where: dml.Where{
						"id": 1.0,
					},
				},
			},
		},
		{
			text: `SET feedbacks a=1,b="b",c={},d=["a","b"],e=false WHERE id = "test";`,
			want: dml.Stmts{
				{
					Op:     dml.SET,
					Entity: u("feedbacks"),
					Assign: dml.Assign{
						"a": 1.0,
						"b": "b",
						"c": map[string]any{},
						"d": []any{"a", "b"},
						"e": false,
					},
					Where: dml.Where{
						"id": "test",
					},
				},
			},
		},
		{
			text: `SET feedbacks a."  "=1, a.b."c"=1,a."this is a test"=1,a."╚(•⌂•)╝".shout=1,a."b".ident."c"."d".test=1 WHERE id = "test";`,
			want: dml.Stmts{
				{
					Op:     dml.SET,
					Entity: u("feedbacks"),
					Assign: dml.Assign{
						`a."  "`:                   1.0,
						`a.b."c"`:                  1.0,
						`a."this is a test"`:       1.0,
						`a."╚(•⌂•)╝".shout`:        1.0,
						`a."b".ident."c"."d".test`: 1.0,
					},
					Where: dml.Where{
						"id": "test",
					},
				},
			},
		},
		{
			text: `SET feedbacks a.b=1,b.test_test.c.d="b",c.d.eee.e={},x."something"="test",d.e.f.ggg.h=["a","b"],e.f.g.h.i.j.k=false WHERE id = "test";`,
			want: dml.Stmts{
				{
					Op:     dml.SET,
					Entity: u("feedbacks"),
					Assign: dml.Assign{
						"a.b":             1.0,
						"b.test_test.c.d": "b",
						"c.d.eee.e":       map[string]any{},
						"d.e.f.ggg.h":     []any{"a", "b"},
						"e.f.g.h.i.j.k":   false,
						`x."something"`:   "test",
					},
					Where: dml.Where{
						"id": "test",
					},
				},
			},
		},
		{
			text: `SET feedbacks a.b = ... ["some", "thing"] WHERE id = "test";`,
			want: dml.Stmts{
				{
					Op:     dml.SET,
					Entity: u("feedbacks"),
					Assign: dml.Assign{
						"a.b": dml.Append[string]{
							Values: []string{"some", "thing"},
						},
					},
					Where: dml.Where{
						"id": "test",
					},
				},
			},
		},
		{
			text: `SET feedbacks a.b = ... [1, 2, 3] WHERE id = "test";`,
			want: dml.Stmts{
				{
					Op:     dml.SET,
					Entity: u("feedbacks"),
					Assign: dml.Assign{
						"a.b": dml.Append[float64]{
							Values: []float64{1, 2, 3},
						},
					},
					Where: dml.Where{
						"id": "test",
					},
				},
			},
		},
		{
			text: `SET feedbacks a.b = ... [1, "string"] WHERE id = "test";`,
			err:  dml.ErrArrayWithMixedTypes,
		},
		{
			text: `SET feedbacks a.b = ... [] WHERE id = "test";`,
			err:  dml.ErrMissingArrayValues,
		},
		{
			text: `SET feedbacks a.b = ["some", "thing"] ... WHERE id = "test";`,
			want: dml.Stmts{
				{
					Op:     dml.SET,
					Entity: u("feedbacks"),
					Assign: dml.Assign{
						"a.b": dml.Prepend[string]{
							Values: []string{"some", "thing"},
						},
					},
					Where: dml.Where{
						"id": "test",
					},
				},
			},
		},
		{
			text: `SET feedbacks a.b = [1, 2] ... WHERE id = "test";`,
			want: dml.Stmts{
				{
					Op:     dml.SET,
					Entity: u("feedbacks"),
					Assign: dml.Assign{
						"a.b": dml.Prepend[float64]{
							Values: []float64{1, 2},
						},
					},
					Where: dml.Where{
						"id": "test",
					},
				},
			},
		},
		{
			text: `SET feedbacks a.b = [1, "string"] ... WHERE id = "test";`,
			err:  dml.ErrArrayWithMixedTypes,
		},
		{
			text: `
			SET feedbacks
				a = ["a"] ...,
				b = ... [1, 2],
				c = ["a", "b"]
		 	WHERE id = "test";
		`,
			want: dml.Stmts{
				{
					Op:     dml.SET,
					Entity: u("feedbacks"),
					Assign: dml.Assign{
						"a": dml.Prepend[string]{
							Values: []string{"a"},
						},
						"b": dml.Append[float64]{
							Values: []float64{1, 2},
						},
						"c": []any{"a", "b"},
					},
					Where: dml.Where{
						"id": "test",
					},
				},
			},
		},
		{
			text: `SET feedbacks a.b = [] ... WHERE id = "test";`,
			err:  dml.ErrMissingArrayValues,
		},
		{
			text: `SET feedbacks a.b = [null, null, null] ... WHERE id = "test";`,
			err:  dml.ErrUnsupportedArrayValue,
		},
		{
			text: `SET feedbacks a.b = ["test", "string"] ... WHERE id = "test";`,
			want: dml.Stmts{
				{
					Op:     dml.SET,
					Entity: u("feedbacks"),
					Assign: dml.Assign{
						"a.b": dml.Prepend[string]{
							Values: []string{"test", "string"},
						},
					},
					Where: dml.Where{
						"id": "test",
					},
				},
			},
		},
		{
			text: `SET feedbacks a.b = ... ["test", "string"] WHERE id = "test";`,
			want: dml.Stmts{
				{
					Op:     dml.SET,
					Entity: u("feedbacks"),
					Assign: dml.Assign{
						"a.b": dml.Append[string]{
							Values: []string{"test", "string"},
						},
					},
					Where: dml.Where{
						"id": "test",
					},
				},
			},
		},
		{
			text: `DELETE feedbacks . WHERE id="abc";`,
			want: dml.Stmts{
				{
					Op:     dml.DELETE,
					Entity: u("feedbacks"),
					Assign: dml.Assign{
						".": dml.DeleteKey{},
					},
					Where: dml.Where{"id": "abc"},
				},
			},
		},
		{
			text: `DELETE feedbacks a WHERE id="abc";`,
			want: dml.Stmts{
				{
					Op:     dml.DELETE,
					Entity: u("feedbacks"),
					Assign: dml.Assign{
						"a": dml.DeleteKey{},
					},
					Where: dml.Where{"id": "abc"},
				},
			},
		},
		{
			text: `DELETE feedbacks a.b WHERE id="abc";`,
			want: dml.Stmts{
				{
					Op:     dml.DELETE,
					Entity: u("feedbacks"),
					Assign: dml.Assign{
						"a.b": dml.DeleteKey{},
					},
					Where: dml.Where{"id": "abc"},
				},
			},
		},
		{
			text: `DELETE feedbacks custom_fields[k]=>v : {"k":"test","v":"test"} WHERE id="abc";`,
			want: dml.Stmts{
				{
					Op:     dml.DELETE,
					Entity: u("feedbacks"),
					Assign: dml.Assign{
						"custom_fields": dml.KeyValueFilter[string]{
							Key:    "test",
							Values: []string{"test"},
						},
					},
					Where: dml.Where{"id": "abc"},
				},
			},
		},
		{
			text: `DELETE feedbacks a[b] : b IN ["a","b"] WHERE id="abc";`,
			want: dml.Stmts{
				{
					Op:     dml.DELETE,
					Entity: u("feedbacks"),
					Assign: dml.Assign{
						"a": dml.KeyFilter{
							Keys: []string{"a", "b"},
						},
					},
					Where: dml.Where{"id": "abc"},
				},
			},
		},
		{
			text: `DELETE feedbacks a[b] => c : b="a" and c IN ["a", "b"] WHERE id="abc";`,
			want: dml.Stmts{
				{
					Op:     dml.DELETE,
					Entity: u("feedbacks"),
					Assign: dml.Assign{
						"a": dml.KeyValueFilter[string]{
							Key:    "a",
							Values: []string{"a", "b"},
						},
					},
					Where: dml.Where{"id": "abc"},
				},
			},
		},
		{
			text: `DELETE feedbacks a[b] : b IN {} WHERE id="abc";`,
			err:  dml.ErrSyntax,
		},
		{
			// IN requires a list
			text: `DELETE feedbacks a[b] : b IN "10" WHERE id="abc";`,
			err:  dml.ErrSyntax,
		},
		{
			// IN requires a []string
			text: `DELETE feedbacks a[b] : b IN ["test", 10] WHERE id="abc";`,
			err:  dml.ErrTypeCheck,
		},
		{
			// IN requires a []string
			text: `DELETE feedbacks a[b] : b IN ["test", false] WHERE id="abc";`,
			err:  dml.ErrTypeCheck,
		},
		{
			// IN requires a []string
			text: `DELETE feedbacks a[b] : b IN ["test", []] WHERE id="abc";`,
			err:  dml.ErrTypeCheck,
		},
		{
			// value requires: IN ["str", ...]
			text: `DELETE feedbacks a[b]=>c : b="test" and c IN ["test", "a", 10] WHERE id="abc";`,
			err:  dml.ErrTypeCheck,
		},
		{
			text: `DELETE a . WHERE b=1 and b=2'`,
			err:  dml.ErrClauseDuplicated,
		},
		{
			text: `DELETE a b[k] : a=1 and a=2 WHERE id=1;`,
			err:  dml.ErrClauseDuplicated,
		},
		{
			text: `DELETE a b[k] : k={} WHERE id=1;`,
			err:  dml.ErrTypeCheck,
		},
		{
			text: `DELETE feedbacks . WHERE id IN [];`,
			err:  dml.ErrSyntax,
		},
		{
			text: `DELETE feedbacks . WHERE id IN [];`,
			err:  dml.ErrSyntax,
		},
		{
			text: `DELETE feedbacks custom_fields[k]=>v : v=1 WHERE id="abc";`,
			err:  dml.ErrUnusedVariable,
		},
		{
			text: `DELETE feedbacks custom_fields[k]=>v : k="test" and v=1 and c=2 WHERE id="abc";`,
			err:  dml.ErrUnknownVariable,
		},
	} {
		got, err := dml.Parse([]byte(tc.text))
		if !errors.Is(err, tc.err) {
			t.Fatalf("%v: while parsing %s", err, tc.text)
		}
		if err != nil {
			continue
		}
		if diff := cmp.Diff(tc.want, got, cmpopts.EquateComparable(unique.Handle[string]{})); diff != "" {
			t.Fatalf("got [+], want [-]: %s", diff)
		}
	}
}

func FuzzParse(f *testing.F) {
	for _, valid := range [][]byte{
		[]byte(`SET a a=1 WHERE a=1;`),                          // simple where
		[]byte(`SET a a=1 WHERE {"a":1};`),                      // object where
		[]byte(`SET a .={"a": 1} WHERE a=1;`),                   // dot assignment
		[]byte(`SET a a=1 WHERE {"a":1};SET a a=1 WHERE id=1;`), // multiple stmts
		[]byte(`SET a a=1,b="b" WHERE a=1;`),                    // multiple assignments
		[]byte(`SET a a=[] WHERE a=1;`),                         // array assignment
		[]byte(`DELETE abc . WHERE a=1;`),                       // simple where (delete)
		[]byte(`DELETE abc a.b WHERE {"a": 1}`),                 // object where (delete)
	} {
		f.Add(valid)
	}
	f.Fuzz(func(t *testing.T, a []byte) {
		out, err := dml.Parse(a)
		if err != nil && len(out) != 0 {
			t.Errorf("%v, %v", out, err)
		}
	})
}
