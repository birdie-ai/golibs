package dml_test

import (
	"bytes"
	"errors"
	"testing"
	"unique"

	dml "github.com/birdie-ai/golibs/dml"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestEncode(t *testing.T) {
	t.Parallel()
	type testcase struct {
		name string
		ast  dml.Stmts
		want string
		err  error
	}

	for _, tc := range []testcase{
		{
			name: "list of errors must contain dml.ErrInvalidOperation",
			ast: dml.Stmts{
				{},
			},
			err: dml.ErrInvalidOperation,
		},
		{
			name: "list of errors must contain dml.ErrMissingAssign",
			ast: dml.Stmts{
				{},
			},
			err: dml.ErrMissingAssign,
		},
		{
			name: "list of errors must contain dml.ErrMissingEntity",
			ast: dml.Stmts{
				{},
			},
			err: dml.ErrMissingEntity,
		},
		{
			name: "list of errors must contain dml.ErrMissingWhere",
			ast: dml.Stmts{
				{},
			},
			err: dml.ErrMissingWhereClause,
		},
		{
			name: "list of errors must contain dml.ErrMissingEntity",
			ast: dml.Stmts{
				{Op: dml.SET},
			},
			err: dml.ErrMissingEntity,
		},
		{
			name: "missing WHERE clause",
			ast: dml.Stmts{
				{Op: dml.SET, Entity: u("test"), Assign: dml.Assign{".": map[string]any{}}},
			},
			err: dml.ErrMissingWhereClause,
		},
		// inner validations
		{
			name: "inner stmts missing lots of fields - must report dml.ErrInvalidOperation",
			ast: dml.Stmts{
				{
					Entity: u("something"),
					Inner: dml.Stmts{
						{},
					},
					Where: dml.Where{"id": "test"},
				},
			},
			err: dml.ErrInvalidOperation,
		},
		{
			name: "inner stmts missing lots of fields - must report dml.ErrMissingAssign",
			ast: dml.Stmts{
				{
					Entity: u("something"),
					Inner: dml.Stmts{
						{},
					},
					Where: dml.Where{"id": "test"},
				},
			},
			err: dml.ErrMissingAssign,
		},
		{
			name: "inner stmts missing lots of fields - must report dml.ErrMissingEntity",
			ast: dml.Stmts{
				{
					Entity: u("something"),
					Inner: dml.Stmts{
						{},
					},
					Where: dml.Where{"id": "test"},
				},
			},
			err: dml.ErrMissingEntity,
		},
		{
			name: "inner stmts missing lots of fields - must report dml.ErrMissingWhereClause",
			ast: dml.Stmts{
				{
					Entity: u("something"),
					Inner: dml.Stmts{
						{},
					},
					Where: dml.Where{"id": "test"},
				},
			},
			err: dml.ErrMissingWhereClause,
		},
		{
			name: "invalid assign key",
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
			name: "inner stmt with invalid assign key",
			ast: dml.Stmts{
				{
					Op:     dml.SET,
					Entity: u("test"),
					Inner: dml.Stmts{
						{
							Op:     dml.SET,
							Entity: u("other"),
							Assign: dml.Assign{"$a": map[string]any{}},
							Where: dml.Where{
								"id": "other",
							},
						},
					},
					Where: dml.Where{
						"id": "abc",
					},
				},
			},
			err: dml.ErrInvalidAssignKey,
		},
		{
			name: "invalid entity name",
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
			name: "invalid ident in where",
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
			name: "invalid path traversal",
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
			name: " invalid assign quoted path traversal",
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
			name: "single-quote path traversal is not valid",
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
			name: "invalid quoted base key",
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
			name: "invalid assign key - required ident",
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
			name: "invalid assign key - required ident",
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
			name: "invalid assign key - required ident",
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
			name: "valid stmt using assign of numbers",
			ast: dml.Stmts{
				{
					Op:     dml.SET,
					Entity: u("feedbacks"),
					Assign: dml.Assign{"a": 1.0},
					Where: dml.Where{
						"id": 1.0,
					},
				},
			},
			want: "SET feedbacks a=1 WHERE id=1;",
		},
		{
			name: "valid stmt using assign of booleans",
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
			name: "valid stmt using assign of objects",
			ast: dml.Stmts{
				{
					Op:     dml.SET,
					Entity: u("feedbacks"),
					Assign: dml.Assign{"a": map[string]any{"k1": "v1", "k2": "v2"}},
					Where: dml.Where{
						"id": "abc",
					},
				},
			},
			want: `SET feedbacks a={"k1":"v1","k2":"v2"} WHERE id="abc";`,
		},
		{
			name: "valid stmt using assign of arrays",
			ast: dml.Stmts{
				{
					Op:     dml.SET,
					Entity: u("feedbacks"),
					Assign: dml.Assign{
						"a.b.c": []any{"a", "b", "c"},
					},
					Where: dml.Where{
						"id": "abc",
					},
				},
			},
			want: `SET feedbacks a.b.c=["a","b","c"] WHERE id="abc";`,
		},
		{
			name: "valid stmt using multiple assigns",
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
			name: "valid dot-assign",
			ast: dml.Stmts{
				{
					Op:     dml.SET,
					Entity: u("organizations"),
					Assign: dml.Assign{
						".": map[string]any{
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
			name: "it's invalid to merge dot-assign with normal assign",
			ast: dml.Stmts{
				{
					Op:     dml.SET,
					Entity: u("organizations"),
					Assign: dml.Assign{
						".": map[string]string{
							"name": "some org",
							"test": "abc",
						},
						"test": 1.0,
					},
					Where: dml.Where{
						"id": "abc",
					},
				},
			},
			err: dml.ErrInvalidDotAssign,
		},
		{
			name: "multiple stmts",
			ast: dml.Stmts{
				{
					Op:     dml.SET,
					Entity: u("feedbacks"),
					Assign: dml.Assign{"a": 1.0},
					Where: dml.Where{
						"id": 1.0,
					},
				},
				{
					Op:     dml.SET,
					Entity: u("organizations"),
					Assign: dml.Assign{"abc": 1.0},
					Where: dml.Where{
						"id": "abc",
					},
				},
			},
			// For now there's no "pretty" format support.
			want: `SET feedbacks a=1 WHERE id=1;SET organizations abc=1 WHERE id="abc";`,
		},
		{
			name: "assign of dashed keys",
			ast: dml.Stmts{
				{
					Op:     dml.SET,
					Entity: u("feedbacks"),
					Assign: dml.Assign{"some-field": 1.0},
					Where: dml.Where{
						"id": 1.0,
					},
				},
			},
			want: "SET feedbacks some-field=1 WHERE id=1;",
		},
		{
			name: "assign of dashed keys in path traversal",
			ast: dml.Stmts{
				{
					Op:     dml.SET,
					Entity: u("feedbacks"),
					Assign: dml.Assign{"abc.some-field": 1.0},
					Where: dml.Where{
						"id": 1.0,
					},
				},
			},
			want: "SET feedbacks abc.some-field=1 WHERE id=1;",
		},
		{
			name: "assign of base dashed path traversal",
			ast: dml.Stmts{
				{
					Op:     dml.SET,
					Entity: u("feedbacks"),
					Assign: dml.Assign{"some-field.some-other-field": 1.0},
					Where: dml.Where{
						"id": 1.0,
					},
				},
			},
			want: "SET feedbacks some-field.some-other-field=1 WHERE id=1;",
		},
		{
			name: "assign of path traversal containing quoted string components",
			ast: dml.Stmts{
				{
					Op:     dml.SET,
					Entity: u("feedbacks"),
					Assign: dml.Assign{`a."some_field"`: 1.0},
					Where: dml.Where{
						"id": 1.0,
					},
				},
			},
			want: `SET feedbacks a."some_field"=1 WHERE id=1;`,
		},
		{
			name: "assign of quoted string containing advanced symbols",
			ast: dml.Stmts{
				{
					Op:     dml.SET,
					Entity: u("feedbacks"),
					Assign: dml.Assign{"a.\"some-other-field\"": 1.0},
					Where: dml.Where{
						"id": 1.0,
					},
				},
			},
			want: `SET feedbacks a."some-other-field"=1 WHERE id=1;`,
		},
		{
			name: "combining multiple quoted strings in path traversal",
			ast: dml.Stmts{
				{
					Op:     dml.SET,
					Entity: u("feedbacks"),
					Assign: dml.Assign{`a."some field".test."other field"`: 1.0},
					Where: dml.Where{
						"id": 1.0,
					},
				},
			},
			want: `SET feedbacks a."some field".test."other field"=1 WHERE id=1;`,
		},
		{
			name: "multiple predicates in AND clause",
			ast: dml.Stmts{
				{
					Op:     dml.SET,
					Entity: u("feedbacks"),
					Assign: dml.Assign{
						"a.b.c": []any{"a", "b", "c"},
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
			name: "append of values list",
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
			name: "prepend of values list",
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
			name: "multiple assigns containing append and prepend",
			ast: dml.Stmts{
				{
					Op:     dml.SET,
					Entity: u("feedbacks"),
					Assign: dml.Assign{
						"d":       1.0,
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
			name: "stmt with single inner stmt",
			ast: dml.Stmts{
				{
					Op:     dml.SET,
					Entity: u("orders"),
					Inner: dml.Stmts{
						{
							Op:     dml.SET,
							Entity: u("feedbacks"),
							Assign: dml.Assign{
								"text": "test",
							},
							Where: dml.Where{
								"id": "abc",
							},
						},
					},
					Where: dml.Where{
						"id": "order_id",
					},
				},
			},
			want: `SET orders (SET feedbacks text="test" WHERE id="abc") WHERE id="order_id";`,
		},
		{
			name: "stmt with mixed assign and inner stmts",
			ast: dml.Stmts{
				{
					Op:     dml.SET,
					Entity: u("orders"),
					Assign: dml.Assign{
						"name": "test",
					},
					Inner: dml.Stmts{
						{
							Op:     dml.SET,
							Entity: u("feedbacks"),
							Assign: dml.Assign{
								"text": "test",
							},
							Where: dml.Where{
								"id": "abc",
							},
						},
						{
							Op:     dml.SET,
							Entity: u("feedbacks"),
							Assign: dml.Assign{
								"text2": "test",
							},
							Where: dml.Where{
								"id": "abc2",
							},
						},
					},
					Where: dml.Where{
						"id": "order_id",
					},
				},
			},
			want: `SET orders name="test",(SET feedbacks text="test" WHERE id="abc"),(SET feedbacks text2="test" WHERE id="abc2") WHERE id="order_id";`,
		},
		{
			name: "append missing values",
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
			name: "prepend missing values",
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
			name: " delete requires only 1 assign",
			ast: dml.Stmts{
				{
					Op:     dml.DELETE,
					Entity: u("feedbacks"),
					Assign: dml.Assign{
						".":      dml.DeleteKey{},
						"labels": dml.DeleteKey{},
					},
					Where: dml.Where{
						"id":     "abc",
						"org_id": "xyz",
					},
				},
			},
			err: dml.ErrInvalidDotAssign,
		},
		{
			name: "delete by key filter",
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
			want: `DELETE feedbacks obj[k] : k="a" WHERE {"id":"abc","org_id":"xyz"};`,
		},
		{
			name: "delete by multiple keys",
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
			want: `DELETE feedbacks obj[k] : k IN ["a","b"] WHERE {"id":"abc","org_id":"xyz"};`,
		},
		{
			name: "delete by value",
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
			want: `DELETE feedbacks labels[_] => v : v="label-1" WHERE {"id":"abc","org_id":"xyz"};`,
		},
		{
			name: "delete by multiple values",
			ast: dml.Stmts{
				{
					Op:     dml.DELETE,
					Entity: u("feedbacks"),
					Assign: dml.Assign{
						"labels": dml.ValueFilter[string]{Values: []string{"label-1", "label-2"}},
					},
					Where: dml.Where{
						"id":     "abc",
						"org_id": "xyz",
					},
				},
			},
			want: `DELETE feedbacks labels[_] => v : v IN ["label-1","label-2"] WHERE {"id":"abc","org_id":"xyz"};`,
		},
		{
			name: "delete by key and value",
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
			want: `DELETE feedbacks custom_fields[k] => v : k="country" AND v="us" WHERE {"id":"abc","org_id":"xyz"};`,
		},
		{
			name: "multiple delete assignments",
			ast: dml.Stmts{
				{
					Op:     dml.DELETE,
					Entity: u("feedbacks"),
					Assign: dml.Assign{
						"a.b":   dml.KeyFilter{Keys: []string{"test"}},
						"a.b.c": dml.KeyValueFilter[string]{Key: "country", Values: []string{"us"}},
					},
					Where: dml.Where{
						"id":     "abc",
						"org_id": "xyz",
					},
				},
			},
			want: `DELETE feedbacks a.b[k] : k="test",a.b.c[k] => v : k="country" AND v="us" WHERE {"id":"abc","org_id":"xyz"};`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			err := dml.Encode(&buf, tc.ast)
			if !errors.Is(err, tc.err) {
				t.Fatalf("error mismatch: expected [%v] but got [%v]", tc.err, err)
			}
			if err != nil {
				return
			}
			got := buf.String()
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Log(tc.want)
				t.Fatalf("got [+], want [-]: %s", diff)
			}
			decoded, err := dml.Parse([]byte(got))
			if err != nil {
				t.Fatalf("failed to decoded the encoded buffer [%s]: %v", tc.want, err)
			}
			if diff := cmp.Diff(decoded, tc.ast, cmpopts.EquateComparable(unique.Handle[string]{})); diff != "" {
				t.Fatalf("encode/decode of [%s], got diff: %s", tc.want, diff)
			}
		})
	}
}

func u(s string) unique.Handle[string] {
	return unique.Make(s)
}
