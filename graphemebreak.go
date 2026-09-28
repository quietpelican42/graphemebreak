// Package graphemebreak splits text into extended grapheme clusters — the
// "user-perceived characters" defined by Unicode Standard Annex #29 (UAX #29)
// — using the default grapheme cluster boundary rules, including the
// Indic_Conjunct_Break rule (GB9c) added in Unicode 15.1 for conjunct
// consonant sequences in Devanagari, Bengali, and related scripts.
//
// It depends only on the standard library. Grapheme cluster property data is
// generated from the Unicode Character Database; see the package README for
// the version and how to regenerate it.
package graphemebreak

import (
	"iter"
	"unicode/utf8"
)

// Graphemes returns an iterator over the extended grapheme clusters of s, in
// order. Each yielded string is a non-empty substring of s; concatenating
// every yielded value reproduces s exactly. Invalid UTF-8 is not modified: an
// invalid byte is treated as if it decoded to U+FFFD for the purpose of
// property lookup, but the original byte is what gets returned, exactly as
// utf8.DecodeRuneInString behaves.
func Graphemes(s string) iter.Seq[string] {
	return func(yield func(string) bool) {
		if s == "" {
			return
		}
		var st state
		start := 0
		i := 0
		for i < len(s) {
			r, size := utf8.DecodeRuneInString(s[i:])
			if st.step(r) && i > start {
				if !yield(s[start:i]) {
					return
				}
				start = i
			}
			i += size
		}
		if start < len(s) {
			yield(s[start:])
		}
	}
}

// GraphemesBytes returns an iterator over the extended grapheme clusters of
// b, in order. Each yielded slice aliases b; callers that retain a yielded
// slice past the next mutation of b must copy it first. Concatenating every
// yielded value reproduces b exactly.
func GraphemesBytes(b []byte) iter.Seq[[]byte] {
	return func(yield func([]byte) bool) {
		if len(b) == 0 {
			return
		}
		var st state
		start := 0
		i := 0
		for i < len(b) {
			r, size := utf8.DecodeRune(b[i:])
			if st.step(r) && i > start {
				if !yield(b[start:i]) {
					return
				}
				start = i
			}
			i += size
		}
		if start < len(b) {
			yield(b[start:])
		}
	}
}

// FirstGrapheme returns the first extended grapheme cluster in s and the
// remainder of s following it. If s is empty, both return values are empty.
func FirstGrapheme(s string) (cluster, rest string) {
	if s == "" {
		return "", ""
	}
	var st state
	i := 0
	for i < len(s) {
		r, size := utf8.DecodeRuneInString(s[i:])
		if st.step(r) && i > 0 {
			return s[:i], s[i:]
		}
		i += size
	}
	return s, ""
}

// FirstGraphemeBytes returns the first extended grapheme cluster in b and the
// remainder of b following it. The returned slices alias b. If b is empty,
// both return values are empty.
func FirstGraphemeBytes(b []byte) (cluster, rest []byte) {
	if len(b) == 0 {
		return nil, nil
	}
	var st state
	i := 0
	for i < len(b) {
		r, size := utf8.DecodeRune(b[i:])
		if st.step(r) && i > 0 {
			return b[:i], b[i:]
		}
		i += size
	}
	return b, nil
}

// GraphemeCount returns the number of extended grapheme clusters in s. It
// runs in a single pass without allocating the intermediate substrings.
func GraphemeCount(s string) int {
	if s == "" {
		return 0
	}
	var st state
	n := 0
	i := 0
	for i < len(s) {
		r, size := utf8.DecodeRuneInString(s[i:])
		if st.step(r) && i > 0 {
			n++
		}
		i += size
	}
	return n + 1
}
