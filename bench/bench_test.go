// Package bench measures graphemebreak against rivo/uniseg and
// clipperhouse/uax29 on the same inputs. It is a separate module so the
// comparison dependencies never appear in the main package's go.mod — see
// ../README.md "Why".
package bench

import (
	"strings"
	"testing"

	uax29 "github.com/clipperhouse/uax29/graphemes"
	uniseg "github.com/rivo/uniseg"

	gb "github.com/quietpelican42/graphemebreak"
)

var (
	plainASCII = strings.Repeat("The quick brown fox jumps over the lazy dog. ", 20)
	combining  = strings.Repeat("éclair à î õ ü ", 20)
	emojiZWJ   = strings.Repeat("\U0001F468‍\U0001F469‍\U0001F467‍\U0001F466 ", 20)
	devanagari = strings.Repeat("क्षत्र ", 20) // conjunct-heavy
	mixed      = plainASCII + combining + emojiZWJ + devanagari
)

func benchInputs() map[string]string {
	return map[string]string{
		"ascii":      plainASCII,
		"combining":  combining,
		"emoji_zwj":  emojiZWJ,
		"devanagari": devanagari,
		"mixed":      mixed,
	}
}

func BenchmarkGraphemebreak(b *testing.B) {
	for name, s := range benchInputs() {
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(s)))
			for i := 0; i < b.N; i++ {
				n := 0
				for range gb.Graphemes(s) {
					n++
				}
				if n == 0 {
					b.Fatal("zero clusters")
				}
			}
		})
	}
}

func BenchmarkUniseg(b *testing.B) {
	for name, s := range benchInputs() {
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(s)))
			for i := 0; i < b.N; i++ {
				n := 0
				g := uniseg.NewGraphemes(s)
				for g.Next() {
					n++
				}
				if n == 0 {
					b.Fatal("zero clusters")
				}
			}
		})
	}
}

func BenchmarkUax29(b *testing.B) {
	for name, s := range benchInputs() {
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(s)))
			for i := 0; i < b.N; i++ {
				n := 0
				seg := uax29.NewStringSegmenter(s)
				for seg.Next() {
					n++
				}
				if n == 0 {
					b.Fatal("zero clusters")
				}
			}
		})
	}
}
