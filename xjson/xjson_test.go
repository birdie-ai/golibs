package xjson_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/birdie-ai/golibs/xjson"
	"github.com/google/go-cmp/cmp"
)

func TestObj(t *testing.T) {
	const example = `
	{
		"number" : 666,
		"string" : "test",
		"bool"   : true,
		"list"   : [6,6,6],
		"nested" : {
			"number" : 777,
			"string" : "test2",
			"bool"   : false,
			"list"   : [7,7,7],
			"nested2" : {
				"number" : 888,
				"string" : "test3",
				"bool"   : true,
				"list"   : [8,8,8],
				"list_objs" : [
					{ "test": "a"},
					{ "test": "b"},
					{ "test": "c"}
				]
			}
		}
	}
	`

	obj, err := xjson.Unmarshal[xjson.Obj](strings.NewReader(example))
	if err != nil {
		t.Fatal(err)
	}

	fmt.Println(obj["number"])

	assertEqual(t, dynGet[float64](t, obj, "number"), 666)
	assertEqual(t, dynGet[string](t, obj, "string"), "test")
	assertEqual(t, dynGet[bool](t, obj, "bool"), true)
	assertEqual(t, dynGet[[]any](t, obj, "list"), []any{float64(6), float64(6), float64(6)})

	// level 1 nesting
	assertEqual(t, dynGet[float64](t, obj, "nested.number"), 777)
	assertEqual(t, dynGet[string](t, obj, "nested.string"), "test2")
	assertEqual(t, dynGet[bool](t, obj, "nested.bool"), false)
	assertEqual(t, dynGet[[]any](t, obj, "nested.list"), []any{float64(7), float64(7), float64(7)})

	// level 2 nesting
	assertEqual(t, dynGet[float64](t, obj, "nested.nested2.number"), 888)
	assertEqual(t, dynGet[string](t, obj, "nested.nested2.string"), "test3")
	assertEqual(t, dynGet[bool](t, obj, "nested.nested2.bool"), true)
	assertEqual(t, dynGet[[]any](t, obj, "nested.nested2.list"), []any{float64(8), float64(8), float64(8)})

	assertEqual(t, dynGet[[]any](t, obj, "nested.nested2.list_objs"), []any{
		xjson.Obj{"test": "a"}, xjson.Obj{"test": "b"}, xjson.Obj{"test": "c"},
	})

	assertNotFound := func(path string) {
		t.Helper()
		v, ok, err := xjson.DynGet[string](obj, path)
		if err != nil {
			t.Fatal(err)
		}
		if ok {
			t.Fatalf("expected not found; got value: %v", v)
		}
	}

	assertNotFound("")
	assertNotFound("notfound")
	assertNotFound("nested.notfound")
	assertNotFound("nested.nested2.notfound")

	v, ok, err := xjson.DynGet[string](obj, "nested")
	if err == nil {
		t.Fatalf("want error but got %v,%v", v, ok)
	}
}

func TestUnmarshal(t *testing.T) {
	type obj struct {
		Name string
	}
	const example = `{"name": "test"}`

	v, err := xjson.Unmarshal[obj](strings.NewReader(example))
	if err != nil {
		t.Fatal(err)
	}
	if v.Name != "test" {
		t.Fatalf("got wrong name %q", v.Name)
	}
}

func TestUnmarshalError(t *testing.T) {
	type obj struct {
		Name string
	}
	const example = `{"name": "test}`

	v, err := xjson.Unmarshal[obj](strings.NewReader(example))
	if err == nil {
		t.Fatalf("got value %v; want error", v)
	}
	var errDetails xjson.UnmarshalError
	if errors.As(err, &errDetails) {
		if errDetails.Data != example {
			t.Fatalf("got %q; want %q", errDetails.Data, example)
		}
	}
}

func TestDecoder(t *testing.T) {
	type obj struct {
		Name  string
		Count int
	}
	const jsonlStream = `
{"name": "test0", "count": 0}
{"name": "test1", "count": 1}
{"name": "test2", "count": 2}
	`

	i := 0
	dec := xjson.NewDecoder[obj](strings.NewReader(jsonlStream))

	for v := range dec.All() {
		wantName := fmt.Sprintf("test%d", i)
		if v.Name != wantName {
			t.Fatalf("got %q; want %q", v.Name, wantName)
		}
		if v.Count != i {
			t.Fatalf("got %q; want %q", v.Count, i)
		}
		i++
	}

	if i != 3 {
		t.Fatalf("got %d iterations; want 3", i)
	}

	if dec.Error() != nil {
		t.Fatalf("unexpected iteration error: %v", dec.Error())
	}

	for v := range dec.All() {
		t.Fatalf("unexpected re-iteration with val: %v", v)
	}
}

func TestDecoderFailureInterruptStream(t *testing.T) {
	type obj struct {
		Name  string
		Count int
	}
	const jsonlStream = `
{"name": "test0", "count": 0}
{"name": "test1", "definitely not JSON 1212
{"name": "test1", "count": 1}
	`

	i := 0
	dec := xjson.NewDecoder[obj](strings.NewReader(jsonlStream))

	for v := range dec.All() {
		wantName := fmt.Sprintf("test%d", i)
		if v.Name != wantName {
			t.Fatalf("got %q; want %q", v.Name, wantName)
		}
		if v.Count != i {
			t.Fatalf("got %q; want %q", v.Count, i)
		}
		i++
	}

	if i != 1 {
		t.Fatalf("got %d iterations; want 1", i)
	}

	firstErr := dec.Error()
	if firstErr == nil {
		t.Fatal("want iteration error but got none")
	}

	for v := range dec.All() {
		t.Fatalf("unexpected re-iteration with val: %v", v)
	}

	secondErr := dec.Error()
	if firstErr != secondErr {
		t.Fatalf("second iteration should not change error: got %v != want %v", secondErr, firstErr)
	}
}

func dynGet[T any](t *testing.T, o xjson.Obj, path string) T {
	t.Helper()

	v, ok, err := xjson.DynGet[T](o, path)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatalf("expected path to exist in object: %v", o)
	}
	return v
}

func assertEqual[T any](t *testing.T, got, want T) {
	t.Helper()
	if d := cmp.Diff(got, want); d != "" {
		t.Fatalf("got(-) want(+):\n%s", d)
	}
}
