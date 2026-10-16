package dml

import (
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

const dotdotdot = "..."

// identifier: ^\p{L}(?:[\p{L}0-9_-]*[\p{L}0-9])?$
func isIdent(s string) bool {
	data := []byte(s)
	r, size := utf8.DecodeRune(data)
	if r == utf8.RuneError || !unicode.IsLetter(r) {
		return false
	}

	var dashtrailer bool
	data = data[size:]
	for len(data) > 0 {
		r, size := utf8.DecodeRune(data)
		if r == '_' || r == '-' || unicode.IsLetter(r) || unicode.IsDigit(r) {
			dashtrailer = r == '-'
			data = data[size:]
			continue
		}
		return false
	}
	return !dashtrailer
}

// isPath checks if s is a valid path traversal (for example: `a`, `a.b`, `a."b.c".d`).
// The path can have a multiple number of dot separated components. The first must be
// an identifier and the next components can be identifiers or double-quoted strings.
func isPath(s string) bool {
	for i := 0; ; i++ {
		comp, rest, ok := nextPathComponent(s)
		if !ok || !isPathComponent(i, comp) {
			return false
		}
		if rest == "" {
			return true
		}
		s = rest[1:] // skip the '.'
	}
}

// nextPathComponent returns the first component of the path s and the remaining input, which
// is either empty or starts with a '.'.
func nextPathComponent(s string) (comp, rest string, ok bool) {
	if len(s) > 0 && s[0] == '"' {
		for j := 1; j < len(s); j++ {
			switch s[j] {
			case '\\':
				j++
			case '"':
				rest = s[j+1:]
				if rest != "" && rest[0] != '.' {
					return "", "", false
				}
				return s[:j+1], rest, true
			}
		}
		return "", "", false
	}
	j := strings.IndexByte(s, '.')
	if j < 0 {
		return s, "", true
	}
	return s[:j], s[j:], true
}

// isPathComponent reports whether s is a valid component at position i of a path traversal.
func isPathComponent(i int, s string) bool {
	if i > 0 && len(s) > 2 && s[0] == '"' {
		_, err := strconv.Unquote(s)
		return err == nil
	}
	return isIdent(s)
}
