package dql

import (
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestLexer(t *testing.T) {
	t.Parallel()

	type testcase struct {
		name string
		in   string
		out  tokvals
		err  error
	}

	for _, tc := range []testcase{
		{
			name: "empty",
			out:  tokvals{token(eofToken, "", 0, 0, 1)},
		},
		{
			name: "basic SEARCH stmt",
			in:   `SEARCH feedbacks id WHERE id=1;`,
			out: tokvals{
				token(identToken, "SEARCH", 0, 0, 1),
				token(identToken, "feedbacks", 7, 0, 8),
				token(identToken, "id", 17, 0, 18),
				token(identToken, "WHERE", 20, 0, 21),
				token(identToken, "id", 26, 0, 27),
				token(equalToken, "=", 28, 0, 29),
				token(numberToken, "1", 29, 0, 30),
				token(semicolonToken, ";", 30, 0, 31),
				token(eofToken, "", 31, 0, 32),
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			l := newlexer(tc.in)
			var got tokvals
			var errs []error
			for {
				tok, err := l.Next()
				errs = append(errs, err)
				got = append(got, tok)
				if tok.Type == eofToken {
					break
				}
			}
			if goterr := errors.Join(errs...); !errors.Is(goterr, tc.err) {
				t.Fatalf("unexpected err[%v], want[%v]", goterr, tc.err)
			}
			if tc.err != nil {
				return
			}
			if diff := cmp.Diff(got, tc.out); diff != "" {
				t.Fatal(diff)
			}
		})
	}
}

func token(t toktype, val string, pos, line, col int) tokval {
	return tokval{
		Type:  t,
		Value: val,
		Pos: Pos{
			Byte:   pos,
			Line:   line,
			Column: col,
		},
	}
}
