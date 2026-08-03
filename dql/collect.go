package dql

import (
	"maps"
	"slices"
)

// CollectFields returns all of the field names present within a statement.
//
// The output of this function is unordered, and may change with each call.
func CollectFields(stmt Stmt) []StaticPath {
	// Use a map to collect fields to avoid needing to dedup the end result
	fields := map[string]StaticPath{}

	if len(stmt.Fields) > 0 {
		for _, f := range stmt.Fields {
			for _, collectedField := range collectFields(f) {
				fields[collectedField.String()] = collectedField
			}
		}
	}

	// TODO(Gu): Not yet implemented.
	// if stmt.Where != nil {
	// }

	if len(stmt.OrderBy) > 0 {
		for _, o := range stmt.OrderBy {
			fields[o.Field.String()] = o.Field
		}
	}

	// TODO(Gu): Not yet implemented.
	// if len(stmt.Aggs) > 0 {
	// }

	if len(fields) > 0 {
		return slices.Collect(maps.Values(fields))
	}

	return []StaticPath{}
}

// collectFields returns the field names for any [Expr].
func collectFields(e Expr) []StaticPath {
	fields := []StaticPath{}
	switch v := e.(type) {
	case VarExpr:
		fields = append(fields, []string{v.Value})
	case PathExpr:
		base := collectFields(v.Base)
		// PathExpr has no base, returning the collected fields.
		if len(base) < 1 {
			return base
		}
		// When evaluating the base path, only include named fields in the path
		if !isNamedField(v.Base) {
			return base
		}
		path := []string{base[0][0]}
		for _, s := range v.Steps {
			if s.Type == FieldStep {
				path = append(path, s.Field)
			}
		}
		fields = append(fields, path)
	case FncallExpr:
		for _, arg := range v.Args {
			fields = append(fields, collectFields(arg)...)
		}
	}

	return fields
}

// isNamedField evaluates whether a expression is a named field. Useful for
// avoiding composite field names which are a mix of base fields and transformed
// fields, e.g fn(a).c.
func isNamedField(e Expr) bool {
	switch e.(type) {
	case VarExpr, PathExpr:
		return true
	}

	return false
}
