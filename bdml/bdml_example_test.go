package bdml_test

import (
	"fmt"
	"unique"

	"github.com/birdie-ai/golibs/bdml"
)

func ExampleStmt() {
	stmt := bdml.Stmt{
		Op:     bdml.SET,
		Entity: unique.Make("mydata"),
		Assign: bdml.Assign{
			"kind.rating": 10,
		},
		Where: bdml.Clauses{
			{
				Field: "id",
				Op:    bdml.Eq,
				Value: "test",
			},
		},
	}

	fmt.Println(stmt)
}
