package dml_test

import (
	"fmt"
	"maps"
	"os"
	"slices"
	"unique"

	dml "github.com/birdie-ai/golibs/dml"
)

func ExampleStmt() {
	stmt := dml.Stmt{
		Op:     dml.SET,
		Entity: unique.Make("mydata"),
		Assign: dml.Assign{
			"kind.rating": 10,
			"labels":      dml.Append[string]{Values: []string{"some-label"}},
		},
		Where: dml.Where{
			"id": "test",
		},
	}
	err := dml.Encode(os.Stdout, dml.Stmts{stmt})
	if err != nil {
		panic(err)
	}

	// Output: SET mydata kind.rating=10,labels=...["some-label"] WHERE id="test";
}

func ExampleParse() {
	stmts, err := dml.Parse([]byte(`
	  SET feedbacks
		custom_fields.connector_id = "my connector",
		custom_fields.name = "test",
		labels = ... ["some-label"]
	  WHERE id = "abc";

	  SET feedbacks
		custom_fields.connector_id = "my other connector",
		custom_fields.name = "test 2",
		labels = ["some-label", "some-other-label"] ...
	  WHERE id = "xyz";
	`))
	if err != nil {
		panic(err)
	}

	fmt.Println(len(stmts))
	for _, stmt := range stmts {
		for _, k := range slices.Sorted(maps.Keys(stmt.Assign)) {
			fmt.Printf("%s: %s\n", k, stmt.Assign[k])
		}
	}
	// Output: 2
	// custom_fields.connector_id: my connector
	// custom_fields.name: test
	// labels: {[some-label]}
	// custom_fields.connector_id: my other connector
	// custom_fields.name: test 2
	// labels: {[some-label some-other-label]}
}
