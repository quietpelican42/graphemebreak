package graphemebreak

// Grapheme_Cluster_Break property values (UAX #29 table 1a).
const (
	gcbOther uint8 = iota
	gcbCR
	gcbLF
	gcbControl
	gcbExtend
	gcbZWJ
	gcbRegionalIndicator
	gcbPrepend
	gcbSpacingMark
	gcbL
	gcbV
	gcbT
	gcbLV
	gcbLVT
)

// Indic_Conjunct_Break property values (derived core property InCB).
const (
	incbNone uint8 = iota
	incbExtend
	incbLinker
	incbConsonant
)

// propRange is one entry in a sorted, non-overlapping range table mapping a
// contiguous span of code points to a single property value.
type propRange struct {
	lo, hi rune
	val    uint8
}

// lookupRange returns the value associated with r in table, or 0 (the
// default/"none" value for every property used here) if r falls in no range.
func lookupRange(table []propRange, r rune) uint8 {
	lo, hi := 0, len(table)
	for lo < hi {
		mid := int(uint(lo+hi) >> 1)
		e := table[mid]
		switch {
		case r < e.lo:
			hi = mid
		case r > e.hi:
			lo = mid + 1
		default:
			return e.val
		}
	}
	return 0
}

// inRange reports whether r falls in any range of table (used for the
// boolean Extended_Pictographic property).
func inRange(table []propRange, r rune) bool {
	lo, hi := 0, len(table)
	for lo < hi {
		mid := int(uint(lo+hi) >> 1)
		e := table[mid]
		switch {
		case r < e.lo:
			hi = mid
		case r > e.hi:
			lo = mid + 1
		default:
			return true
		}
	}
	return false
}

// gcbProperty returns the Grapheme_Cluster_Break property value of r. CR and
// LF are handled directly, matching the Unicode data exactly (there is
// exactly one code point in each category); every other value, including
// Control, comes from the generated table rather than a hand-picked
// shortcut, so a future Unicode version cannot silently invalidate an
// assumption baked in here.
func gcbProperty(r rune) uint8 {
	switch r {
	case '\r':
		return gcbCR
	case '\n':
		return gcbLF
	}
	return lookupRange(gcbTable, r)
}

func incbProperty(r rune) uint8 {
	return lookupRange(incbTable, r)
}

func isExtendedPictographic(r rune) bool {
	return inRange(extPictTable, r)
}
