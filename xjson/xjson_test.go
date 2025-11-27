package xjson_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/birdie-ai/golibs/xjson"
	"github.com/google/go-cmp/cmp"
)

func TestDynGet(t *testing.T) {
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
			},
			"with.dot" : {
				"number" : 112,
				"string" : "testdot",
				"bool"   : true
			},
			"." : {
				"number" : 911,
				"string" : "just dot"
			},
			"a\".b" : {
				"number" : 6,
				"string" : "escaping"
			}
		}
	}
	`

	obj, err := xjson.Unmarshal[xjson.Obj](strings.NewReader(example))
	if err != nil {
		t.Fatal(err)
	}

	fmt.Println(obj["number"])

	t.Run("basic", func(t *testing.T) {
		assertEqual(t, dynGet[float64](t, obj, "number"), 666)
		assertEqual(t, dynGet[string](t, obj, "string"), "test")
		assertEqual(t, dynGet[bool](t, obj, "bool"), true)
		assertEqual(t, dynGet[[]any](t, obj, "list"), []any{float64(6), float64(6), float64(6)})
	})

	t.Run("level 1 nesting", func(t *testing.T) {
		assertEqual(t, dynGet[float64](t, obj, "nested.number"), 777)
		assertEqual(t, dynGet[string](t, obj, "nested.string"), "test2")
		assertEqual(t, dynGet[bool](t, obj, "nested.bool"), false)
		assertEqual(t, dynGet[[]any](t, obj, "nested.list"), []any{float64(7), float64(7), float64(7)})
	})

	t.Run("level 2 nesting", func(t *testing.T) {
		assertEqual(t, dynGet[float64](t, obj, "nested.nested2.number"), 888)
		assertEqual(t, dynGet[string](t, obj, "nested.nested2.string"), "test3")
		assertEqual(t, dynGet[bool](t, obj, "nested.nested2.bool"), true)
		assertEqual(t, dynGet[[]any](t, obj, "nested.nested2.list"), []any{float64(8), float64(8), float64(8)})
		assertEqual(t, dynGet[[]any](t, obj, "nested.nested2.list_objs"), []any{
			xjson.Obj{"test": "a"}, xjson.Obj{"test": "b"}, xjson.Obj{"test": "c"},
		})
	})

	t.Run("key with dot", func(t *testing.T) {
		assertEqual(t, dynGet[float64](t, obj, `nested."with.dot".number`), 112)
		assertEqual(t, dynGet[string](t, obj, `nested."with.dot".string`), "testdot")
		assertEqual(t, dynGet[bool](t, obj, `nested."with.dot".bool`), true)
	})

	t.Run("key is just dot", func(t *testing.T) {
		assertEqual(t, dynGet[float64](t, obj, `nested.".".number`), 911)
		assertEqual(t, dynGet[string](t, obj, `nested.".".string`), "just dot")
	})

	t.Run("key with dot and escaped double quotes", func(t *testing.T) {
		assertEqual(t, dynGet[float64](t, obj, `nested."a\".b".number`), 6)
		assertEqual(t, dynGet[string](t, obj, `nested."a\".b".string`), "escaping")
	})

	t.Run("invalid paths", func(t *testing.T) {
		assertInvalid := func(path string) {
			t.Helper()
			_, err := xjson.DynGet[string](obj, path)
			if !errors.Is(err, xjson.ErrInvalidPath) {
				t.Fatalf("got %v; want %v", err, xjson.ErrNotFound)
			}
		}
		assertInvalid("")
		assertInvalid(".")
		assertInvalid("notfound.")
		assertInvalid(".notfound")
		assertInvalid(".notfound.")
	})

	t.Run("not found", func(t *testing.T) {
		assertNotFound := func(path string) {
			t.Helper()
			_, err := xjson.DynGet[string](obj, path)
			if !errors.Is(err, xjson.ErrNotFound) {
				t.Fatalf("got %v; want %v", err, xjson.ErrNotFound)
			}
		}
		assertNotFound("notfound")
		assertNotFound("nested.notfound")
		assertNotFound(`nested."with.dot".notfound`)
	})

	t.Run("wrong type", func(t *testing.T) {
		v, err := xjson.DynGet[string](obj, "nested")
		if err == nil {
			t.Fatalf("want error but got %v", v)
		}
	})
}

func TestDynSet(t *testing.T) {
	obj := xjson.Obj{}
	dynSet(t, obj, "text", "test")
	dynSet(t, obj, "number", 666)
	dynSet(t, obj, "list", []int{6, 6, 6})
	dynSet(t, obj, "object", xjson.Obj{
		"a": xjson.Obj{
			"b": xjson.Obj{
				"c": "c_value",
			},
		},
	})

	assertEqual(t, obj, xjson.Obj{
		"text":   "test",
		"number": 666,
		"list":   []int{6, 6, 6},
		"object": xjson.Obj{
			"a": xjson.Obj{
				"b": xjson.Obj{
					"c": "c_value",
				},
			},
		},
	})

	dynSet(t, obj, "object.a.b.c", xjson.Obj{
		"overwrite": true,
	})

	assertEqual(t, obj, xjson.Obj{
		"text":   "test",
		"number": 666,
		"list":   []int{6, 6, 6},
		"object": xjson.Obj{
			"a": xjson.Obj{
				"b": xjson.Obj{
					"c": xjson.Obj{
						"overwrite": true,
					},
				},
			},
		},
	})

	dynSet(t, obj, "object.merged", true)
	assertEqual(t, obj, xjson.Obj{
		"text":   "test",
		"number": 666,
		"list":   []int{6, 6, 6},
		"object": xjson.Obj{
			"merged": true,
			"a": xjson.Obj{
				"b": xjson.Obj{
					"c": xjson.Obj{
						"overwrite": true,
					},
				},
			},
		},
	})

	dynSet(t, obj, `object."with.dot.again".x`, true)
	assertEqual(t, obj, xjson.Obj{
		"text":   "test",
		"number": 666,
		"list":   []int{6, 6, 6},
		"object": xjson.Obj{
			"merged": true,
			"with.dot.again": xjson.Obj{
				"x": true,
			},
			"a": xjson.Obj{
				"b": xjson.Obj{
					"c": xjson.Obj{
						"overwrite": true,
					},
				},
			},
		},
	})

	dynSet(t, obj, "object", "overwritten")
	assertEqual(t, obj, xjson.Obj{
		"text":   "test",
		"number": 666,
		"list":   []int{6, 6, 6},
		"object": "overwritten",
	})
}

func TestDynSetInvalidPath(t *testing.T) {
	invalidPaths := []string{
		"",
		".",
		".name",
		"name.",
		".name.",
	}
	obj := xjson.Obj{}

	for _, invalidPath := range invalidPaths {
		err := xjson.DynSet(obj, invalidPath, true)
		if !errors.Is(err, xjson.ErrInvalidPath) {
			t.Errorf("path %q should be invalid; got %v", invalidPath, err)
		}
	}
}

func TestDynValidatePath(t *testing.T) {
	invalidPaths := []string{
		"",
		".",
		"..",
		".name",
		"name.",
		".name.",
	}
	for _, invalidPath := range invalidPaths {
		if xjson.IsValidDynPath(invalidPath) {
			t.Errorf("path %q should be invalid", invalidPath)
		}
	}
}

func TestUnmarshalFile(t *testing.T) {
	type obj struct {
		Name string
	}
	const example = `{"name": "test"}`

	path := filepath.Join(t.TempDir(), "file.json")
	if err := os.WriteFile(path, []byte(example), 0666); err != nil {
		t.Fatal(err)
	}

	v, err := xjson.UnmarshalFile[obj](path)
	if err != nil {
		t.Fatal(err)
	}
	if v.Name != "test" {
		t.Fatalf("got wrong name %q", v.Name)
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

	v, err := xjson.DynGet[T](o, path)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func dynSet(t *testing.T, o xjson.Obj, path string, value any) {
	t.Helper()

	err := xjson.DynSet(o, path, value)
	if err != nil {
		t.Fatal(err)
	}
}

func assertEqual[T any](t *testing.T, got, want T) {
	t.Helper()
	if d := cmp.Diff(got, want); d != "" {
		t.Fatalf("got(-) want(+):\n%s", d)
	}
}
