// Command gen downloads the pinned Unicode Character Database (UCD) files
// that define the extended grapheme cluster boundary properties, verifies
// their sha256 checksums, and writes ../../tables_gen.go.
//
// Run it with `go generate ./...` from the package root, or `go run
// ./internal/gen` from here. It requires network access to unicode.org and
// is not run at build or test time — only when regenerating the tables for a
// new Unicode version.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

// unicodeVersion is the pinned UCD version these tables are generated from.
// Bump it, update the URLs and sha256 sums below (from
// https://www.unicode.org/Public/<version>/ucd/...), and re-run this
// generator to move to a new version.
const unicodeVersion = "18.0.0"

const ucdBase = "https://www.unicode.org/Public/" + unicodeVersion + "/ucd/"

type source struct {
	name   string // logical name, used in error messages
	url    string
	sha256 string // hex-encoded, lowercase
}

var sources = []source{
	{
		name:   "GraphemeBreakProperty.txt",
		url:    ucdBase + "auxiliary/GraphemeBreakProperty.txt",
		sha256: "0839dcb79e4ac639ecd538b1abf7c9d22e3f9dd265b7e182d33627aa4d75b45a",
	},
	{
		name:   "DerivedCoreProperties.txt",
		url:    ucdBase + "DerivedCoreProperties.txt",
		sha256: "09c928886a178fcafd93c29e4bd59073a058e5a100b716d425cb563ab50f68c9",
	},
	{
		name:   "emoji-data.txt",
		url:    ucdBase + "emoji/emoji-data.txt",
		sha256: "80d00f8e616a0ef27fd6b8de3b758c06383b5d917e2977709578e68baf733bf1",
	},
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "gen:", err)
		os.Exit(1)
	}
}

func run() error {
	content := make(map[string][]byte, len(sources))
	client := &http.Client{Timeout: 30 * time.Second}

	for _, src := range sources {
		fmt.Fprintf(os.Stderr, "fetching %s ...\n", src.url)
		b, err := fetch(client, src.url)
		if err != nil {
			return fmt.Errorf("fetching %s: %w", src.name, err)
		}
		sum := sha256.Sum256(b)
		got := hex.EncodeToString(sum[:])
		if got != src.sha256 {
			return fmt.Errorf("%s: sha256 mismatch: got %s, want %s (Unicode data changed, or this is not %s — update the pin deliberately, don't silence this check)", src.name, got, src.sha256, unicodeVersion)
		}
		content[src.name] = b
	}

	gcbRanges, err := parseSimpleProperty(content["GraphemeBreakProperty.txt"], gcbValues)
	if err != nil {
		return fmt.Errorf("parsing GraphemeBreakProperty.txt: %w", err)
	}

	incbRanges, err := parseInCB(content["DerivedCoreProperties.txt"])
	if err != nil {
		return fmt.Errorf("parsing DerivedCoreProperties.txt: %w", err)
	}

	extPictRanges, err := parseBooleanProperty(content["emoji-data.txt"], "Extended_Pictographic")
	if err != nil {
		return fmt.Errorf("parsing emoji-data.txt: %w", err)
	}

	out, err := render(gcbRanges, incbRanges, extPictRanges)
	if err != nil {
		return err
	}

	outPath := "tables_gen.go"
	if err := os.WriteFile(outPath, out, 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", outPath, err)
	}
	fmt.Fprintf(os.Stderr, "wrote %s (%d GCB ranges, %d InCB ranges, %d Extended_Pictographic ranges)\n",
		outPath, len(gcbRanges), len(incbRanges), len(extPictRanges))
	return nil
}

func fetch(client *http.Client, url string) ([]byte, error) {
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %s", resp.Status)
	}
	return io.ReadAll(resp.Body)
}

// entry is one parsed data-file line: a code point range and the property
// value name attached to it (before mapping to our numeric encoding).
type entry struct {
	lo, hi rune
	value  string
}

// gcbValues maps Grapheme_Cluster_Break property value names to our
// numeric encoding (see tables.go).
var gcbValues = map[string]uint8{
	"CR":                 1,
	"LF":                 2,
	"Control":            3,
	"Extend":             4,
	"ZWJ":                5,
	"Regional_Indicator": 6,
	"Prepend":            7,
	"SpacingMark":        8,
	"L":                  9,
	"V":                  10,
	"T":                  11,
	"LV":                 12,
	"LVT":                13,
}

var incbValues = map[string]uint8{
	"Extend":    1,
	"Linker":    2,
	"Consonant": 3,
}

// parseDataLines splits a UCD data file into (range, ;-separated fields)
// pairs, stripping comments and blank lines. Each returned []string has the
// range's fields already trimmed.
func parseDataLines(data []byte) ([]struct {
	lo, hi rune
	fields []string
}, error) {
	var out []struct {
		lo, hi rune
		fields []string
	}
	for _, line := range strings.Split(string(data), "\n") {
		if i := strings.IndexByte(line, '#'); i >= 0 {
			line = line[:i]
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, ";")
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}
		rangeField := parts[0]
		lo, hi, err := parseCodepointRange(rangeField)
		if err != nil {
			return nil, fmt.Errorf("bad range %q: %w", rangeField, err)
		}
		out = append(out, struct {
			lo, hi rune
			fields []string
		}{lo, hi, parts[1:]})
	}
	return out, nil
}

func parseCodepointRange(s string) (lo, hi rune, err error) {
	if dots := strings.Index(s, ".."); dots >= 0 {
		loV, err := strconv.ParseInt(s[:dots], 16, 32)
		if err != nil {
			return 0, 0, err
		}
		hiV, err := strconv.ParseInt(s[dots+2:], 16, 32)
		if err != nil {
			return 0, 0, err
		}
		return rune(loV), rune(hiV), nil
	}
	v, err := strconv.ParseInt(s, 16, 32)
	if err != nil {
		return 0, 0, err
	}
	return rune(v), rune(v), nil
}

// parseSimpleProperty parses a file in the "<range> ; <PropertyValue>"
// format (GraphemeBreakProperty.txt, WordBreakProperty.txt,
// SentenceBreakProperty.txt) and maps each value through values.
func parseSimpleProperty(data []byte, values map[string]uint8) ([]entry, error) {
	lines, err := parseDataLines(data)
	if err != nil {
		return nil, err
	}
	entries := make([]entry, 0, len(lines))
	for _, l := range lines {
		if len(l.fields) == 0 {
			continue
		}
		name := l.fields[0]
		if _, ok := values[name]; !ok {
			continue // property value we don't encode (there are none for GCB, but be tolerant)
		}
		entries = append(entries, entry{l.lo, l.hi, name})
	}
	return coalesce(entries, values), nil
}

// parseInCB parses DerivedCoreProperties.txt, keeping only lines of the form
// "<range> ; InCB ; <value>".
func parseInCB(data []byte) ([]entry, error) {
	lines, err := parseDataLines(data)
	if err != nil {
		return nil, err
	}
	var entries []entry
	for _, l := range lines {
		if len(l.fields) < 2 || l.fields[0] != "InCB" {
			continue
		}
		name := l.fields[1]
		if _, ok := incbValues[name]; !ok {
			continue
		}
		entries = append(entries, entry{l.lo, l.hi, name})
	}
	return coalesce(entries, incbValues), nil
}

// parseBooleanProperty parses a file in the "<range> ; <PropertyName>"
// format and keeps only ranges tagged with the given property name,
// collapsing the value to a single boolean (encoded as 1).
func parseBooleanProperty(data []byte, propName string) ([]entry, error) {
	lines, err := parseDataLines(data)
	if err != nil {
		return nil, err
	}
	var entries []entry
	for _, l := range lines {
		if len(l.fields) == 0 || l.fields[0] != propName {
			continue
		}
		entries = append(entries, entry{l.lo, l.hi, propName})
	}
	return coalesce(entries, map[string]uint8{propName: 1}), nil
}

// coalesce sorts entries by code point and merges adjacent/overlapping
// ranges that carry the same value, producing the smallest sorted,
// non-overlapping range table equivalent to the input.
func coalesce(entries []entry, values map[string]uint8) []entry {
	type re struct {
		lo, hi rune
		val    uint8
	}
	res := make([]re, 0, len(entries))
	for _, e := range entries {
		res = append(res, re{e.lo, e.hi, values[e.value]})
	}
	sort.Slice(res, func(i, j int) bool {
		if res[i].lo != res[j].lo {
			return res[i].lo < res[j].lo
		}
		return res[i].hi < res[j].hi
	})
	merged := make([]re, 0, len(res))
	for _, r := range res {
		if n := len(merged); n > 0 && merged[n-1].val == r.val && r.lo <= merged[n-1].hi+1 {
			if r.hi > merged[n-1].hi {
				merged[n-1].hi = r.hi
			}
			continue
		}
		merged = append(merged, r)
	}
	final := make([]entry, len(merged))
	for i, m := range merged {
		final[i] = entry{lo: m.lo, hi: m.hi, value: strconv.Itoa(int(m.val))}
	}
	return final
}

func render(gcb, incb, extPict []entry) ([]byte, error) {
	var b strings.Builder
	fmt.Fprintf(&b, "// Code generated by internal/gen from the Unicode Character Database; DO NOT EDIT.\n")
	fmt.Fprintf(&b, "//\n")
	fmt.Fprintf(&b, "// Source: Unicode Character Database version %s (https://www.unicode.org/Public/%s/ucd/).\n", unicodeVersion, unicodeVersion)
	fmt.Fprintf(&b, "// Generated tables are derived data under the Unicode License v3; see LICENSE-unicode.\n")
	fmt.Fprintf(&b, "//\n")
	fmt.Fprintf(&b, "// Regenerate with: go generate ./...\n")
	fmt.Fprintf(&b, "\npackage graphemebreak\n\n")

	writeTable(&b, "gcbTable", gcb)
	writeTable(&b, "incbTable", incb)
	writeTable(&b, "extPictTable", extPict)

	return []byte(b.String()), nil
}

func writeTable(b *strings.Builder, name string, entries []entry) {
	fmt.Fprintf(b, "var %s = []propRange{\n", name)
	for _, e := range entries {
		fmt.Fprintf(b, "\t{0x%04X, 0x%04X, %s},\n", e.lo, e.hi, e.value)
	}
	fmt.Fprintf(b, "}\n\n")
}
