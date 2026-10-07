package graphemebreak_test

// This file runs the official Unicode GraphemeBreakTest.txt against the
// package. It is pinned to the same UCD version as tables_gen.go
// (unicodeVersion below) and verified by sha256 before use, exactly like the
// generator. The file is fetched into a gitignored cache directory the first
// time the test runs and reused after that; with no network and no cache, it
// skips with an explicit, actionable message rather than silently passing.

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	gb "github.com/quietpelican42/graphemebreak"
)

const (
	testUnicodeVersion = "18.0.0"
	testFileURL        = "https://www.unicode.org/Public/" + testUnicodeVersion + "/ucd/auxiliary/GraphemeBreakTest.txt"
	testFileSHA256     = "b0cf047ee94485bbdc846de2b902f5f8a815f6b674f9d04223cddadd91c9df31"
)

// loadConformanceFile returns the contents of GraphemeBreakTest.txt, from a
// gitignored local cache if present and correct, or freshly downloaded and
// verified otherwise. It calls t.Skip with an actionable message if neither
// is available (typically: no network in this environment).
func loadConformanceFile(t *testing.T) []byte {
	t.Helper()

	cacheDir := filepath.Join("testdata", ".cache")
	cachePath := filepath.Join(cacheDir, "GraphemeBreakTest-"+testUnicodeVersion+".txt")

	if b, err := os.ReadFile(cachePath); err == nil {
		if verifySHA256(b) {
			return b
		}
		t.Logf("cached %s failed sha256 verification, re-downloading", cachePath)
	}

	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Get(testFileURL)
	if err != nil {
		t.Skipf("no network access to fetch %s (%v) — this test needs the official Unicode conformance file; re-run with network access, or place a verified copy at %s", testFileURL, err, cachePath)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Skipf("fetching %s: unexpected status %s", testFileURL, resp.Status)
	}
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Skipf("fetching %s: %v", testFileURL, err)
	}
	if !verifySHA256(b) {
		t.Fatalf("downloaded %s does not match the pinned sha256 %s — Unicode data changed or the download was corrupted", testFileURL, testFileSHA256)
	}

	if err := os.MkdirAll(cacheDir, 0o755); err == nil {
		_ = os.WriteFile(cachePath, b, 0o644)
	}
	return b
}

func verifySHA256(b []byte) bool {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]) == testFileSHA256
}

// testCase is one line of GraphemeBreakTest.txt: the full string with breaks
// removed, and the expected grapheme clusters.
type testCase struct {
	input    string
	expected []string
	raw      string // original line, for failure messages
}

func parseConformanceFile(t *testing.T, data []byte) []testCase {
	t.Helper()
	var cases []testCase
	sc := bufio.NewScanner(strings.NewReader(string(data)))
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		raw := sc.Text()
		line := raw
		if i := strings.IndexByte(line, '#'); i >= 0 {
			line = line[:i]
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var clusters []string
		var cur strings.Builder
		for _, tok := range strings.Fields(line) {
			switch tok {
			case "÷":
				if cur.Len() > 0 {
					clusters = append(clusters, cur.String())
					cur.Reset()
				}
			case "×":
				// no boundary; keep building the current cluster
			default:
				cp, err := strconv.ParseInt(tok, 16, 32)
				if err != nil {
					t.Fatalf("parsing conformance file: bad token %q in line %q: %v", tok, raw, err)
				}
				cur.WriteRune(rune(cp))
			}
		}
		if cur.Len() > 0 {
			clusters = append(clusters, cur.String())
		}
		cases = append(cases, testCase{
			input:    strings.Join(clusters, ""),
			expected: clusters,
			raw:      raw,
		})
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("scanning conformance file: %v", err)
	}
	return cases
}

func TestConformanceGraphemeBreakTest(t *testing.T) {
	data := loadConformanceFile(t)
	cases := parseConformanceFile(t, data)
	if len(cases) == 0 {
		t.Fatal("parsed zero test cases from GraphemeBreakTest.txt — parser is broken")
	}

	var failures int
	for _, tc := range cases {
		var got []string
		for cluster := range gb.Graphemes(tc.input) {
			got = append(got, cluster)
		}
		if !equalStrings(got, tc.expected) {
			failures++
			if failures <= 20 {
				t.Errorf("%s\n  input:    %q\n  expected: %q\n  got:      %q", tc.raw, tc.input, tc.expected, got)
			}
		}
	}
	if failures > 0 {
		t.Fatalf("%d/%d conformance cases failed (Unicode %s)", failures, len(cases), testUnicodeVersion)
	}
	t.Logf("passed all %d cases from GraphemeBreakTest.txt (Unicode %s)", len(cases), testUnicodeVersion)
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
