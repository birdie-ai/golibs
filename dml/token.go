package dml

import (
	"unicode"
	"unicode/utf8"
)

// identifier: \p{L}+[_\p{L}0-9]* with support for matched quotes
func isIdent(s string) bool {
	// Check for quotes
	if len(s) >= 3 && s[0] == '"' && s[len(s)-1] == '"' {
		// when between quotes, any combination of characters is allow
		return true
	}

	data := []byte(s)
	r, size := utf8.DecodeRune(data)
	if r == utf8.RuneError || !unicode.IsLetter(r) {
		return false
	}

	data = data[size:]
	for len(data) > 0 {
		r, size := utf8.DecodeRune(data)
		if r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r) {
			data = data[size:]
			continue
		}
		return false
	}
	return true
}
