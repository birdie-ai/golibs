package dql

import (
	"fmt"
	"maps"
	"slices"
)

// Collect can be used to fetch all of the fields name present within a statement.
func Collect(stmt Stmt) []string {
	// Use a map to collect fields to avoid needing to dedup the end result
	fields := map[string]struct{}{}

	if len(stmt.Fields) > 0 {
		for _, f := range stmt.Fields {
			for _, collectedField := range collectFields(f) {
				fields[collectedField] = struct{}{}
			}
		}
	}

	if stmt.Where != nil {
		// TODO(Gu): Not yet implemented.
	}

	if len(stmt.OrderBy) > 0 {
		// TODO(Gu): Not yet implemented.
	}

	if len(stmt.Aggs) > 0 {
		// TODO(Gu): Not yet implemented.
	}

	if len(fields) > 0 {
		return slices.Collect(maps.Keys(fields))
	}

	return []string{}
}

func collectFields(e Expr) []string {
	fields := []string{}
	switch v := e.(type) {
	case VarExpr:
		fields = append(fields, v.Value)
	case PathExpr:
		base := collectFields(v.Base)
		if len(base) != 1 {
			return base
		}
		path := base[0]
		for _, s := range v.Steps {
			if s.Type == FieldStep {
				path = fmt.Sprintf("%s.%s", path, s.Field)
			}
		}
		fields = append(fields, path)
	}

	return fields
}
