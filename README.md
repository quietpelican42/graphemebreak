# graphemebreak

Splits text into Unicode extended grapheme clusters (UAX #29 revision 49,
Unicode 18.0.0), including the current form of the Indic_Conjunct_Break rule
(GB9c).

```go
for cluster := range graphemebreak.Graphemes("Namaste नमस्ते \U0001F64F") {
    fmt.Println(cluster)
}
// N a m a s t e (space) न म स्ते (space) 🙏 — 13 clusters; स्ते (4 code points:
// स + ् + त + े) is one grapheme cluster, not four.
```

## Why

Two established Go grapheme segmenters, `rivo/uniseg` and
`clipperhouse/uax29`, both fail the same class of case: conjunct consonant
sequences in Devanagari, Bengali, and related scripts, governed by rule GB9c
(Indic_Conjunct_Break), added in Unicode 15.1. A third, less widely used
package, `scalecode-solutions/runeseg`, gets closer but still misses cases
where the conjunct's leading consonant is implicit rather than literal.

Measured: a Go program running each library's grapheme iterator over every
line of the official `GraphemeBreakTest.txt` (Unicode 18.0.0, 853 cases) and
diffing the resulting clusters against the file's expected output.

```
rivo/uniseg (v0.4.7):                    827/853 (96.95%)
clipperhouse/uax29 (v1.16.0):            827/853 (96.95%)
scalecode-solutions/runeseg (v1.0.4):    843/853 (98.83%)
graphemebreak (this package):            853/853 (100.00%)
```

Every failure in the two established libraries is a GB9c case, e.g. the pair
U+0915 U+094D U+0915 (Devanagari "क्क") splits into two clusters instead of
one. `graphemebreak` was built by re-implementing UAX #29 directly rather
than adapting either library, against the **current** GB9c rule: UAX #29
revision 49 (reissued for Unicode 18.0.0) changed GB9c to

```
\p{InCB=Linker} \p{InCB=Extend}* × \p{InCB=Consonant}
```

— no leading consonant required. The rule published for Unicode 15.1
through 17.0 (UAX #29 revisions up to 47) read

```
\p{InCB=Consonant} [\p{InCB=Extend}\p{InCB=Linker}]* \p{InCB=Linker} [\p{InCB=Extend}\p{InCB=Linker}]* × \p{InCB=Consonant}
```

instead, which does require a leading consonant. `rivo/uniseg` and
`clipperhouse/uax29` implement that older form, which is why they fail cases
where no consonant precedes the linker (e.g. a plain letter, or the start of
the string, directly followed by a virama and a consonant) — the official
test data is built against the current rule. See `break.go` for the
automaton and the exact citations.

Correctness costs some speed against `clipperhouse/uax29`, which precomputes
a lookup trie rather than binary-searching range tables; it's a fair trade
here, since `uax29`'s speed comes with the GB9c cases above unfixed.

Measured: `cd bench && go test -bench=. -benchtime=1s .`, two runs, Apple M5
Pro, `ns/op` (lower is better):

| input | graphemebreak | rivo/uniseg | clipperhouse/uax29 |
| --- | --- | --- | --- |
| ascii | 12.8–12.9 µs | 44.5–46.8 µs | 4.3–4.4 µs |
| mixed (ascii + combining + emoji ZWJ + Devanagari) | 25.6–25.9 µs | 87.2–88.9 µs | 7.7–8.0 µs |

All three allocate 0 B/op on these inputs.

## Installation

```sh
go get github.com/quietpelican42/graphemebreak
```

## Usage

### Iterate over clusters

```go
for cluster := range graphemebreak.Graphemes(s) {
    // cluster is a substring of s; concatenating every cluster reproduces s.
}
```

### Work with `[]byte` without copying into a string

```go
for cluster := range graphemebreak.GraphemesBytes(b) {
    // cluster aliases b; copy it if you need to keep it past the next mutation of b.
}
```

### Take just the first cluster

```go
first, rest := graphemebreak.FirstGrapheme(s)
```

### Count clusters without allocating

```go
n := graphemebreak.GraphemeCount(s) // e.g. len("👨‍👩‍👧‍👦") == 25 bytes, GraphemeCount == 1
```

### Stream clusters from an `io.Reader`

```go
r := graphemebreak.NewReader(file)
for r.Next() {
    process(r.Grapheme())
}
if err := r.Err(); err != nil {
    // handle read error
}
```

## Reference

| Function | Signature |
| --- | --- |
| `Graphemes` | `func Graphemes(s string) iter.Seq[string]` |
| `GraphemesBytes` | `func GraphemesBytes(b []byte) iter.Seq[[]byte]` |
| `FirstGrapheme` | `func FirstGrapheme(s string) (cluster, rest string)` |
| `FirstGraphemeBytes` | `func FirstGraphemeBytes(b []byte) (cluster, rest []byte)` |
| `GraphemeCount` | `func GraphemeCount(s string) int` |
| `NewReader` | `func NewReader(r io.Reader) *Reader` |

`Reader` methods: `Next() bool`, `Grapheme() []byte`, `GraphemeString() string`,
`Err() error` — the same calling convention as `bufio.Scanner`.

## How it works

Grapheme cluster boundaries (UAX #29 rules GB1–GB999) are evaluated as a
single-pass forward automaton over the input's runes: one property lookup
and one state transition per rune, in `break.go`. Three rules — GB9c
(Indic_Conjunct_Break), GB11 (emoji ZWJ sequences), and GB12/GB13 (regional
indicator pairs) — need more than the immediately preceding rune's property,
so each is tracked with a couple of bits of rolling state rather than
re-scanning backward. The whole algorithm is O(n) with O(1) additional
state, and the public iterators allocate nothing beyond the substrings they
yield.

Property data (`Grapheme_Cluster_Break`, `Indic_Conjunct_Break`, and
`Extended_Pictographic`) comes from the Unicode Character Database, pinned to
version 18.0.0. `internal/gen` downloads the three source files, checks each
against a pinned sha256, and writes `tables_gen.go` as sorted, coalesced
range tables looked up by binary search. Regenerate with `go generate ./...`
after updating the version and hashes in `internal/gen/main.go`.

## Limitations

- Ships grapheme cluster segmentation only. Word and sentence boundaries
  (UAX #29 also defines these) are not implemented; adding them without
  verifying each against its own official test file was out of scope here.
- `Reader` decodes with `bufio.Reader.ReadRune`, which replaces invalid UTF-8
  bytes with U+FFFD (one byte consumed per replacement) — this is a
  limitation of `bufio.Reader` itself, not something this package adds. On
  invalid UTF-8 input, `Reader`'s output is not guaranteed to reproduce the
  original bytes; `Graphemes` and `GraphemesBytes`, which decode directly
  from the string or slice, always do.
- A single grapheme cluster's memory use is bounded only by the input:
  pathological input (e.g. a long run of combining marks) makes one cluster
  arbitrarily large, the same limitation every UAX #29 implementation
  surveyed here has.
- The conformance test (`conformance_test.go`) needs network access the
  first time it runs, to fetch and verify the official test file into a
  gitignored cache; with no network and no cache it skips loudly rather than
  passing silently.

## Compatibility

Built and tested with Go 1.27.1 on darwin/arm64. CI runs the same checks on
`ubuntu-latest` with `go-version: stable`; that combination is run in CI, not
independently verified here.

## Development

```sh
go generate ./...                 # regenerate tables_gen.go from the UCD
go build ./...
go vet ./...
go test -race -count=1 ./...
gofmt -l .                        # must print nothing
```

Benchmarks against `rivo/uniseg` and `clipperhouse/uax29` live in `bench/`,
a separate Go module so those dependencies never appear in this package's
`go.mod`:

```sh
cd bench && go test -bench=. -benchtime=1s .
```

## License

MIT — see [LICENSE](LICENSE). Generated grapheme cluster property tables
(`tables_gen.go`) are derived from the Unicode Character Database and carry
the Unicode License v3 notice; see [LICENSE-unicode](LICENSE-unicode).
