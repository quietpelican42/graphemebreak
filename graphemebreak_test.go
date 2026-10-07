package graphemebreak_test

import (
	"bytes"
	"strings"
	"testing"
	"unicode/utf8"

	gb "github.com/quietpelican42/graphemebreak"
)

func collect(s string) []string {
	var out []string
	for c := range gb.Graphemes(s) {
		out = append(out, c)
	}
	return out
}

func TestGraphemesHandPicked(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []string
	}{
		{"empty", "", nil},
		{"ascii", "abc", []string{"a", "b", "c"}},
		{"crlf", "a\r\nb", []string{"a", "\r\n", "b"}},
		{"combining mark", "éclair", []string{"é", "c", "l", "a", "i", "r"}},
		{
			"family emoji (ZWJ sequence)",
			"\U0001F468‍\U0001F469‍\U0001F467‍\U0001F466",
			[]string{"\U0001F468‍\U0001F469‍\U0001F467‍\U0001F466"},
		},
		{
			"flag (regional indicator pair)",
			"\U0001F1FA\U0001F1F8",
			[]string{"\U0001F1FA\U0001F1F8"},
		},
		{
			"two flags stay separate (even RI run breaks in the middle)",
			"\U0001F1FA\U0001F1F8\U0001F1EE\U0001F1F9",
			[]string{"\U0001F1FA\U0001F1F8", "\U0001F1EE\U0001F1F9"},
		},
		{
			"devanagari conjunct (GB9c)",
			"क्त", // KA + VIRAMA + TA, one cluster
			[]string{"क्त"},
		},
		{
			"devanagari conjunct with no leading consonant still binds (matches GraphemeBreakTest.txt)",
			"a्त", // 'a' + VIRAMA + TA
			[]string{"a्त"},
		},
		{
			"hangul syllable block",
			"각", // L + V + T jamo
			[]string{"각"},
		},
		{"emoji with skin tone modifier", "\U0001F44D\U0001F3FB", []string{"\U0001F44D\U0001F3FB"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := collect(tt.in)
			if !equalStrings(got, tt.want) {
				t.Errorf("Graphemes(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestGraphemesConcatenationReproducesInput(t *testing.T) {
	inputs := []string{
		"",
		"hello, world",
		"éclair with \U0001F468‍\U0001F469‍\U0001F467 family and \U0001F1FA\U0001F1F8\U0001F1EE\U0001F1F9 flags",
		"क्तक्त",
		strings.Repeat("\U0001F1E6\U0001F1E7", 50),
		"line1\r\nline2\nline3\rline4",
		string([]byte{0xff, 0xfe, 'a', 0x80}), // invalid UTF-8 mixed with valid
	}
	for _, in := range inputs {
		var buf strings.Builder
		n := 0
		for c := range gb.Graphemes(in) {
			if c == "" {
				t.Errorf("Graphemes(%q) yielded an empty cluster", in)
			}
			buf.WriteString(c)
			n++
		}
		if buf.String() != in {
			t.Errorf("Graphemes(%q): concatenation = %q, want %q", in, buf.String(), in)
		}
		if in == "" && n != 0 {
			t.Errorf("Graphemes(\"\") yielded %d clusters, want 0", n)
		}
	}
}

func TestGraphemesBytesMatchesString(t *testing.T) {
	inputs := []string{
		"hello, world",
		"éclair \U0001F468‍\U0001F469‍\U0001F467",
		"क्त",
	}
	for _, in := range inputs {
		var wantByBytes [][]byte
		for c := range gb.GraphemesBytes([]byte(in)) {
			cp := append([]byte(nil), c...)
			wantByBytes = append(wantByBytes, cp)
		}
		var wantByString []string
		for c := range gb.Graphemes(in) {
			wantByString = append(wantByString, c)
		}
		if len(wantByBytes) != len(wantByString) {
			t.Fatalf("%q: byte iterator yielded %d clusters, string iterator yielded %d", in, len(wantByBytes), len(wantByString))
		}
		for i := range wantByBytes {
			if string(wantByBytes[i]) != wantByString[i] {
				t.Errorf("%q: cluster %d: bytes=%q string=%q", in, i, wantByBytes[i], wantByString[i])
			}
		}
	}
}

func TestFirstGrapheme(t *testing.T) {
	tests := []struct {
		in         string
		cluster    string
		restPrefix string
	}{
		{"", "", ""},
		{"abc", "a", "bc"},
		{"éclair", "é", "clair"},
		{"\U0001F1FA\U0001F1F8rest", "\U0001F1FA\U0001F1F8", "rest"},
	}
	for _, tt := range tests {
		cluster, rest := gb.FirstGrapheme(tt.in)
		if cluster != tt.cluster || rest != tt.restPrefix {
			t.Errorf("FirstGrapheme(%q) = (%q, %q), want (%q, %q)", tt.in, cluster, rest, tt.cluster, tt.restPrefix)
		}
		if cluster+rest != tt.in {
			t.Errorf("FirstGrapheme(%q): cluster+rest = %q, want %q", tt.in, cluster+rest, tt.in)
		}
	}
}

func TestFirstGraphemeBytes(t *testing.T) {
	in := []byte("éclair")
	cluster, rest := gb.FirstGraphemeBytes(in)
	if string(cluster) != "é" || string(rest) != "clair" {
		t.Errorf("FirstGraphemeBytes(%q) = (%q, %q)", in, cluster, rest)
	}
}

func TestFirstGraphemeBytesEmpty(t *testing.T) {
	cluster, rest := gb.FirstGraphemeBytes(nil)
	if cluster != nil || rest != nil {
		t.Errorf("FirstGraphemeBytes(nil) = (%q, %q), want (nil, nil)", cluster, rest)
	}
}

func TestGraphemeCount(t *testing.T) {
	tests := []struct {
		in   string
		want int
	}{
		{"", 0},
		{"abc", 3},
		{"éclair", 6},
		{"\U0001F468‍\U0001F469‍\U0001F467‍\U0001F466", 1},
		{"\U0001F1FA\U0001F1F8\U0001F1EE\U0001F1F9", 2},
	}
	for _, tt := range tests {
		if got := gb.GraphemeCount(tt.in); got != tt.want {
			t.Errorf("GraphemeCount(%q) = %d, want %d", tt.in, got, tt.want)
		}
		if got := len(collect(tt.in)); got != tt.want {
			t.Errorf("len(collect(%q)) = %d, want %d (GraphemeCount and Graphemes disagree)", tt.in, got, tt.want)
		}
	}
}

func TestReaderMatchesGraphemes(t *testing.T) {
	inputs := []string{
		"",
		"hello, world",
		"éclair with \U0001F468‍\U0001F469‍\U0001F467 family and \U0001F1FA\U0001F1F8\U0001F1EE\U0001F1F9 flags",
		"क्तक्त",
		strings.Repeat("x", 5000) + "\U0001F468‍\U0001F469", // crosses typical bufio buffer boundaries
	}
	for _, in := range inputs {
		want := collect(in)
		r := gb.NewReader(strings.NewReader(in))
		var got []string
		for r.Next() {
			got = append(got, r.GraphemeString())
		}
		if err := r.Err(); err != nil {
			t.Fatalf("Reader on %q: %v", in, err)
		}
		if !equalStrings(got, want) {
			t.Errorf("Reader(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestReaderReproducesInput(t *testing.T) {
	in := "The quick brown fox éclair \U0001F468‍\U0001F469‍\U0001F467 \U0001F1FA\U0001F1F8\U0001F1EE\U0001F1F9"
	r := gb.NewReader(strings.NewReader(in))
	var buf bytes.Buffer
	for r.Next() {
		if len(r.Grapheme()) == 0 {
			t.Fatal("Reader yielded an empty cluster")
		}
		buf.Write(r.Grapheme())
	}
	if err := r.Err(); err != nil {
		t.Fatalf("Reader: %v", err)
	}
	if buf.String() != in {
		t.Errorf("Reader concatenation = %q, want %q", buf.String(), in)
	}
}

// TestNoEmptyOrInvalidClusters is a broad randomized sweep (not a Fuzz test,
// so it always runs) over strings built from a mix of ASCII, combining
// marks, ZWJ, regional indicators, and Devanagari conjunct characters,
// checking the two invariants every segmenter must hold.
func TestNoEmptyOrInvalidClusters(t *testing.T) {
	alphabet := []rune{
		'a', 'b', '\r', '\n',
		'́',                        // combining acute accent (Extend)
		'‍',                        // ZWJ
		'\U0001F600',               // Extended_Pictographic
		'\U0001F1FA', '\U0001F1F8', // regional indicators
		'क', '्', 'त', // Devanagari K, virama, T
	}
	rng := newXorshift(12345)
	for trial := 0; trial < 2000; trial++ {
		n := int(rng.next() % 12)
		var b strings.Builder
		for i := 0; i < n; i++ {
			b.WriteRune(alphabet[rng.next()%uint64(len(alphabet))])
		}
		s := b.String()

		var reassembled strings.Builder
		for c := range gb.Graphemes(s) {
			if c == "" {
				t.Fatalf("empty cluster for input %q", s)
			}
			if !utf8.ValidString(c) {
				t.Fatalf("invalid UTF-8 cluster %q for input %q", c, s)
			}
			reassembled.WriteString(c)
		}
		if reassembled.String() != s {
			t.Fatalf("concatenation mismatch: input %q, reassembled %q", s, reassembled.String())
		}
	}
}

// xorshift64 is a tiny deterministic PRNG so this test is reproducible
// without pulling in math/rand/v2 for what is a handful of values.
type xorshift64 struct{ x uint64 }

func newXorshift(seed uint64) *xorshift64 {
	if seed == 0 {
		seed = 1
	}
	return &xorshift64{x: seed}
}

func (x *xorshift64) next() uint64 {
	x.x ^= x.x << 13
	x.x ^= x.x >> 7
	x.x ^= x.x << 17
	return x.x
}
