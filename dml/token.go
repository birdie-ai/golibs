package dml

import (
	"unicode"
	"unicode/utf8"
)

// identifier: \p{L}+[\p{L}0-9_-]*[\p{L}0-9]+$ with support for matched quotes
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
