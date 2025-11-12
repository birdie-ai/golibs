package dml_test

import (
	"fmt"
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
		Where: dml.Clauses{
			{
				Field: "id",
				Op:    dml.Eq,
				Value: "test",
			},
		},
	}

	fmt.Println(stmt)
}
