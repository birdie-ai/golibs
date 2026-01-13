package obj_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/birdie-ai/golibs/obj"
	"github.com/birdie-ai/golibs/xjson"
	"github.com/google/go-cmp/cmp"
)

func TestGet(t *testing.T) {
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

	o, err := xjson.Unmarshal[obj.O](strings.NewReader(example))
	if err != nil {
		t.Fatal(err)
	}

	fmt.Println(o["number"])

	t.Run("basic", func(t *testing.T) {
		assertEqual(t, dynGet[float64](t, o, "number"), 666)
		assertEqual(t, dynGet[string](t, o, "string"), "test")
		assertEqual(t, dynGet[bool](t, o, "bool"), true)
		assertEqual(t, dynGet[[]any](t, o, "list"), []any{float64(6), float64(6), float64(6)})
	})

	t.Run("level 1 nesting", func(t *testing.T) {
		assertEqual(t, dynGet[float64](t, o, "nested.number"), 777)
		assertEqual(t, dynGet[string](t, o, "nested.string"), "test2")
		assertEqual(t, dynGet[bool](t, o, "nested.bool"), false)
		assertEqual(t, dynGet[[]any](t, o, "nested.list"), []any{float64(7), float64(7), float64(7)})
	})

	t.Run("level 2 nesting", func(t *testing.T) {
		assertEqual(t, dynGet[float64](t, o, "nested.nested2.number"), 888)
		assertEqual(t, dynGet[string](t, o, "nested.nested2.string"), "test3")
		assertEqual(t, dynGet[bool](t, o, "nested.nested2.bool"), true)
		assertEqual(t, dynGet[[]any](t, o, "nested.nested2.list"), []any{float64(8), float64(8), float64(8)})
		assertEqual(t, dynGet[[]any](t, o, "nested.nested2.list_objs"), []any{
			obj.O{"test": "a"}, obj.O{"test": "b"}, obj.O{"test": "c"},
		})
	})

	t.Run("key with dot", func(t *testing.T) {
		assertEqual(t, dynGet[float64](t, o, `nested."with.dot".number`), 112)
		assertEqual(t, dynGet[string](t, o, `nested."with.dot".string`), "testdot")
		assertEqual(t, dynGet[bool](t, o, `nested."with.dot".bool`), true)
	})

	t.Run("key is just dot", func(t *testing.T) {
		assertEqual(t, dynGet[float64](t, o, `nested.".".number`), 911)
		assertEqual(t, dynGet[string](t, o, `nested.".".string`), "just dot")
	})

	t.Run("key with dot and escaped double quotes", func(t *testing.T) {
		assertEqual(t, dynGet[float64](t, o, `nested."a\".b".number`), 6)
		assertEqual(t, dynGet[string](t, o, `nested."a\".b".string`), "escaping")
	})

	t.Run("invalid paths", func(t *testing.T) {
		assertInvalid := func(path string) {
			t.Helper()
			_, err := obj.Get[string](o, path)
			if !errors.Is(err, obj.ErrInvalidPath) {
				t.Fatalf("got %v; want %v", err, obj.ErrNotFound)
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
			_, err := obj.Get[string](o, path)
			if !errors.Is(err, obj.ErrNotFound) {
				t.Fatalf("got %v; want %v", err, obj.ErrNotFound)
			}
		}
		assertNotFound("notfound")
		assertNotFound("nested.notfound")
		assertNotFound(`nested."with.dot".notfound`)
	})

	t.Run("wrong type", func(t *testing.T) {
		v, err := obj.Get[string](o, "nested")
		if err == nil {
			t.Fatalf("want error but got %v", v)
		}
	})
}

func TestSet(t *testing.T) {
	o := obj.O{}
	dynSet(t, o, "text", "test")
	dynSet(t, o, "number", 666)
	dynSet(t, o, "list", []int{6, 6, 6})
	dynSet(t, o, "object", obj.O{
		"a": obj.O{
			"b": obj.O{
				"c": "c_value",
			},
		},
	})

	assertEqual(t, o, obj.O{
		"text":   "test",
		"number": 666,
		"list":   []int{6, 6, 6},
		"object": obj.O{
			"a": obj.O{
				"b": obj.O{
					"c": "c_value",
				},
			},
		},
	})

	dynSet(t, o, "object.a.b.c", obj.O{
		"overwrite": true,
	})

	assertEqual(t, o, obj.O{
		"text":   "test",
		"number": 666,
		"list":   []int{6, 6, 6},
		"object": obj.O{
			"a": obj.O{
				"b": obj.O{
					"c": obj.O{
						"overwrite": true,
					},
				},
			},
		},
	})

	dynSet(t, o, "object.merged", true)
	assertEqual(t, o, obj.O{
		"text":   "test",
		"number": 666,
		"list":   []int{6, 6, 6},
		"object": obj.O{
			"merged": true,
			"a": obj.O{
				"b": obj.O{
					"c": obj.O{
						"overwrite": true,
					},
				},
			},
		},
	})

	dynSet(t, o, `object."with.dot.again".x`, true)
	assertEqual(t, o, obj.O{
		"text":   "test",
		"number": 666,
		"list":   []int{6, 6, 6},
		"object": obj.O{
			"merged": true,
			"with.dot.again": obj.O{
				"x": true,
			},
			"a": obj.O{
				"b": obj.O{
					"c": obj.O{
						"overwrite": true,
					},
				},
			},
		},
	})

	dynSet(t, o, "object", "overwritten")
	assertEqual(t, o, obj.O{
		"text":   "test",
		"number": 666,
		"list":   []int{6, 6, 6},
		"object": "overwritten",
	})
}

func TestDel(t *testing.T) {
	dynDel(t, nil, "does.not.exist")

	o := obj.O{}
	dynDel(t, o, "does.not.exist")

	dynSet(t, o, "text", "test")
	dynSet(t, o, "number", 666)
	dynSet(t, o, "list", []int{6, 6, 6})
	dynSet(t, o, "object", obj.O{
		"a": obj.O{
			"b": obj.O{
				"c": "c_value",
			},
		},
	})

	dynDel(t, o, "number")
	assertEqual(t, o, obj.O{
		"text": "test",
		"list": []int{6, 6, 6},
		"object": obj.O{
			"a": obj.O{
				"b": obj.O{
					"c": "c_value",
				},
			},
		},
	})

	dynDel(t, o, "object.a.b")
	assertEqual(t, o, obj.O{
		"text": "test",
		"list": []int{6, 6, 6},
		"object": obj.O{
			"a": obj.O{},
		},
	})

	dynDel(t, o, "object")
	dynDel(t, o, "text")
	dynDel(t, o, "list")
	assertEqual(t, o, obj.O{})
}

func TestSetInvalidPath(t *testing.T) {
	invalidPaths := []string{
		"",
		".",
		".name",
		"name.",
		".name.",
	}
	o := obj.O{}

	for _, invalidPath := range invalidPaths {
		err := obj.Set(o, invalidPath, true)
		if !errors.Is(err, obj.ErrInvalidPath) {
			t.Errorf("path %q should be invalid; got %v", invalidPath, err)
		}
	}
}

func TestValidatePath(t *testing.T) {
	invalidPaths := []string{
		"",
		".",
		"..",
		".name",
		"name.",
		".name.",
	}
	for _, invalidPath := range invalidPaths {
		if obj.IsValidPath(invalidPath) {
			t.Errorf("path %q should be invalid", invalidPath)
		}
	}
}

func dynGet[T any](t *testing.T, o obj.O, path string) T {
	t.Helper()

	v, err := obj.Get[T](o, path)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func dynSet(t *testing.T, o obj.O, path string, value any) {
	t.Helper()

	err := obj.Set(o, path, value)
	if err != nil {
		t.Fatal(err)
	}
}

func dynDel(t *testing.T, o obj.O, path string) {
	t.Helper()

	err := obj.Del(o, path)
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
