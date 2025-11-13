package dml

import (
	"unicode"
	"unicode/utf8"
)

func isIdent(s string) bool {
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
