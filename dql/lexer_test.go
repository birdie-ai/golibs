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
				token(keywordToken, "SEARCH", 0, 0, 1),
				token(identToken, "feedbacks", 7, 0, 8),
				token(identToken, "id", 17, 0, 18),
				token(keywordToken, "WHERE", 20, 0, 21),
				token(identToken, "id", 26, 0, 27),
				token(equalToken, "=", 28, 0, 29),
				token(numberToken, "1", 29, 0, 30),
				token(semicolonToken, ";", 30, 0, 31),
				token(eofToken, "", 31, 0, 32),
			},
		},
		{
			name: "advanced statement",
			in: `
				AS s1
					SEARCH orders
						   id, labels, custom_fields.abc.xyz
					WHERE {
						"$and": [
							{"text": "something"},
							{"$or": [{"posted_at": {"$gte": "2026-01-01"}}, {"custom_fields.abc": "abc"}]}
						]
					}
					ORDER BY posted_at ASC
					LIMIT 10000
					AGGS {
						filtered: filter({"custom_fields.xyz": "test"}) {
							value: count(labels)
						}
					}
					WITH CURSOR;
			`,
			out: tokvals{
				token(keywordToken, "AS", 5, 1, 4),
				token(identToken, "s1", 8, 1, 7),
				token(keywordToken, "SEARCH", 16, 2, 5),
				token(identToken, "orders", 23, 2, 12),
				token(identToken, "id", 39, 3, 9),
				token(commaToken, ",", 41, 3, 11),
				token(identToken, "labels", 43, 3, 13),
				token(commaToken, ",", 49, 3, 19),
				token(identToken, "custom_fields", 51, 3, 21),
				token(dotToken, ".", 64, 3, 34),
				token(identToken, "abc", 65, 3, 35),
				token(dotToken, ".", 68, 3, 38),
				token(identToken, "xyz", 69, 3, 39),
				token(keywordToken, "WHERE", 78, 4, 5),
				token(lbraceToken, "{", 84, 4, 11),
				token(stringToken, "$and", 92, 5, 6),
				token(colonToken, ":", 98, 5, 12),
				token(lbrackToken, "[", 100, 5, 14),
				token(lbraceToken, "{", 109, 6, 7),
				token(stringToken, "text", 110, 6, 8),
				token(colonToken, ":", 116, 6, 14),
				token(stringToken, "something", 118, 6, 16),
				token(rbraceToken, "}", 129, 6, 27),
				token(commaToken, ",", 130, 6, 28),
				token(lbraceToken, "{", 139, 7, 7),
				token(stringToken, "$or", 140, 7, 8),
				token(colonToken, ":", 145, 7, 13),
				token(lbrackToken, "[", 147, 7, 15),
				token(lbraceToken, "{", 148, 7, 16),
				token(stringToken, "posted_at", 149, 7, 17),
				token(colonToken, ":", 160, 7, 28),
				token(lbraceToken, "{", 162, 7, 30),
				token(stringToken, "$gte", 163, 7, 31),
				token(colonToken, ":", 169, 7, 37),
				token(stringToken, "2026-01-01", 171, 7, 39),
				token(rbraceToken, "}", 183, 7, 51),
				token(rbraceToken, "}", 184, 7, 52),
				token(commaToken, ",", 185, 7, 53),
				token(lbraceToken, "{", 187, 7, 55),
				token(stringToken, "custom_fields.abc", 188, 7, 56),
				token(colonToken, ":", 207, 7, 75),
				token(stringToken, "abc", 209, 7, 77),
				token(rbraceToken, "}", 214, 7, 82),
				token(rbrackToken, "]", 215, 7, 83),
				token(rbraceToken, "}", 216, 7, 84),
				token(rbrackToken, "]", 224, 8, 6),
				token(rbraceToken, "}", 231, 9, 5),
				token(keywordToken, "ORDER", 238, 10, 5),
				token(keywordToken, "BY", 244, 10, 11),
				token(identToken, "posted_at", 247, 10, 14),
				token(keywordToken, "ASC", 257, 10, 24),
				token(keywordToken, "LIMIT", 266, 11, 5),
				token(numberToken, "10000", 272, 11, 11),
				token(keywordToken, "AGGS", 283, 12, 5),
				token(lbraceToken, "{", 288, 12, 10),
				token(identToken, "filtered", 296, 13, 6),
				token(colonToken, ":", 304, 13, 14),
				token(identToken, "filter", 306, 13, 16),
				token(lparenToken, "(", 312, 13, 22),
				token(lbraceToken, "{", 313, 13, 23),
				token(stringToken, "custom_fields.xyz", 314, 13, 24),
				token(colonToken, ":", 333, 13, 43),
				token(stringToken, "test", 335, 13, 45),
				token(rbraceToken, "}", 341, 13, 51),
				token(rparenToken, ")", 342, 13, 52),
				token(lbraceToken, "{", 344, 13, 54),
				token(identToken, "value", 353, 14, 7),
				token(colonToken, ":", 358, 14, 12),
				token(identToken, "count", 360, 14, 14),
				token(lparenToken, "(", 365, 14, 19),
				token(identToken, "labels", 366, 14, 20),
				token(rparenToken, ")", 372, 14, 26),
				token(rbraceToken, "}", 380, 15, 6),
				token(rbraceToken, "}", 387, 16, 5),
				token(keywordToken, "WITH", 394, 17, 5),
				token(keywordToken, "CURSOR", 399, 17, 10),
				token(semicolonToken, ";", 405, 17, 16),
				token(eofToken, "", 410, 18, 3),
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
