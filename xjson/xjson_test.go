package xjson_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/birdie-ai/golibs/xjson"
)

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

	if dec.Error() == nil {
		t.Fatal("want iteration error but got none")
	}

	for v := range dec.All() {
		t.Fatalf("unexpected re-iteration with val: %v", v)
	}
}
