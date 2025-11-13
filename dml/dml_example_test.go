package dml_test

import (
	"os"
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
