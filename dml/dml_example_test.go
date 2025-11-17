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
		},
		Where: dml.Where{
			"id": "test",
		},
	}
	err := dml.Encode(os.Stdout, dml.Stmts{stmt})
	if err != nil {
		panic(err)
	}

	// Output: SET mydata kind.rating=10 WHERE id="test";
}

func ExampleParse() {
	stmts, err := dml.Parse([]byte(`
	  SET feedbacks
		custom_fields.connector_id = "my connector",
		custom_fields.name = "test"
	  WHERE id = "abc";

	  SET feedbacks
		custom_fields.connector_id = "my other connector",
		custom_fields.name = "test 2"
	  WHERE id = "xyz";
	`))
	if err != nil {
		panic(err)
	}

	fmt.Println(len(stmts))
	for _, stmt := range stmts {
		for _, k := range slices.Sorted(maps.Keys(stmt.Assign)) {
			fmt.Printf("%s: %s\n", k, stmt.Assign[k].(string))
		}
	}
	// Output: 2
	// custom_fields.connector_id: my connector
	// custom_fields.name: test
	// custom_fields.connector_id: my other connector
	// custom_fields.name: test 2
}
