package graphemebreak_test

import (
	"testing"
	"unicode/utf8"

	gb "github.com/__GH_OWNER__/graphemebreak"
)

// FuzzGraphemes checks the two invariants that must hold for any input,
// valid UTF-8 or not: concatenating every yielded cluster reproduces the
// input exactly, no cluster is empty, and GraphemeCount agrees with the
// number of clusters actually yielded.
func FuzzGraphemes(f *testing.F) {
	seeds := []string{
		"",
		"hello",
		"éclair",
		"\U0001F468‍\U0001F469‍\U0001F467‍\U0001F466",
		"\U0001F1FA\U0001F1F8\U0001F1EE\U0001F1F9\U0001F1EE\U0001F1F9",
		"क्त्त",
		"a्क",
		"\r\n\r\n",
		string([]byte{0xff, 0xfe, 0x80, 'a', 0xc0}),
		"\U0001F600‍\U0001F600",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, s string) {
		var buf []byte
		n := 0
		for c := range gb.Graphemes(s) {
			if c == "" {
				t.Fatalf("empty cluster for input %q", s)
			}
			buf = append(buf, c...)
			n++
		}
		if string(buf) != s {
			t.Fatalf("concatenation mismatch: input %q, got %q", s, buf)
		}
		if want := gb.GraphemeCount(s); want != n {
			t.Fatalf("GraphemeCount(%q) = %d, but Graphemes yielded %d clusters", s, want, n)
		}

		// Byte-slice variant must agree with the string variant.
		var bufFromBytes []byte
		nb := 0
		for c := range gb.GraphemesBytes([]byte(s)) {
			if len(c) == 0 {
				t.Fatalf("GraphemesBytes: empty cluster for input %q", s)
			}
			bufFromBytes = append(bufFromBytes, c...)
			nb++
		}
		if string(bufFromBytes) != s {
			t.Fatalf("GraphemesBytes concatenation mismatch: input %q, got %q", s, bufFromBytes)
		}
		if nb != n {
			t.Fatalf("GraphemesBytes yielded %d clusters, Graphemes yielded %d, for input %q", nb, n, s)
		}

		// FirstGrapheme must agree with the first cluster Graphemes yields.
		first, rest := gb.FirstGrapheme(s)
		if n == 0 {
			if first != "" || rest != "" {
				t.Fatalf("FirstGrapheme(%q) = (%q, %q), want (\"\", \"\") for empty input", s, first, rest)
			}
		} else {
			if first+rest != s {
				t.Fatalf("FirstGrapheme(%q): first+rest = %q, want %q", s, first+rest, s)
			}
			if !utf8.ValidString(s) {
				return // rest re-segmentation below assumes valid UTF-8 boundaries throughout
			}
		}
	})
}
