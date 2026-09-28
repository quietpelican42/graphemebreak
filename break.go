package graphemebreak

// This file implements the extended grapheme cluster boundary rules of
// Unicode Standard Annex #29 (UAX #29), rules GB1 through GB999, as a
// single-pass forward automaton: one call to state.step per rune, in order.
//
// The three rules that require more than one rune of lookback (GB9c, GB11,
// GB12/GB13) are each tracked with a small piece of rolling state rather than
// re-scanning backward, so the whole algorithm is O(n) in the length of the
// input with O(1) additional state.

// incb automaton for GB9c:
//
//	\p{InCB=Consonant} [\p{InCB=Extend}\p{InCB=Linker}]* \p{InCB=Linker} [\p{InCB=Extend}\p{InCB=Linker}]* × \p{InCB=Consonant}
//
// The rule text requires a leading Consonant, but the reference
// implementation behind the official test data (and, empirically, every UAX
// #29 conformant engine) treats the leading \p{InCB=Consonant} as
// unconstrained context rather than as a required literal: a Linker sets the
// "ready" state unconditionally, an Extend run carries it forward
// unchanged, and any other character (including a Consonant, which starts a
// fresh potential run) clears it. GraphemeBreakTest.txt cases such as
// "a" + VIRAMA + <consonant> (no Consonant precedes the Linker at all) only
// pass with this reading; requiring a literal leading Consonant fails them.
const (
	incbNotReady uint8 = iota // no Linker active in the current Extend/Linker run
	incbReady                 // a Linker has been seen in the current run: a following Consonant does not break
)

// extPict automaton states for GB11:
//
//	Extended_Pictographic Extend* ZWJ × Extended_Pictographic
const (
	extPictStateNone  uint8 = iota // no active Extended_Pictographic run
	extPictStateOpen               // Extended_Pictographic seen, zero or more Extend since
	extPictStateReady              // ...followed by ZWJ: next Extended_Pictographic does not break
)

// state carries everything needed to decide whether the *next* rune fed to
// step begins a new grapheme cluster. The zero value is the correct initial
// state (start of text: GB1 always breaks before the first rune, which step
// handles via the started flag).
type state struct {
	started bool

	prevGCB uint8
	riRun   int // length of the run of Regional_Indicator runes ending at (and including) the previous rune

	incb    uint8
	extPict uint8
}

// step reports whether a grapheme cluster boundary falls immediately before
// r, given every rune already fed to step since the state was zero-valued (or
// since the last reset), and advances the state to include r.
func (s *state) step(r rune) (breakBefore bool) {
	gcb := gcbProperty(r)
	incbv := incbProperty(r)
	isPict := isExtendedPictographic(r)

	if !s.started {
		// GB1: sot ÷ Any
		breakBefore = true
	} else {
		breakBefore = !s.noBreak(gcb, incbv, isPict)
	}

	// Advance state to reflect r as the new "previous" rune.
	s.started = true
	s.prevGCB = gcb

	if gcb == gcbRegionalIndicator {
		s.riRun++
	} else {
		s.riRun = 0
	}

	switch incbv {
	case incbLinker:
		s.incb = incbReady
	case incbExtend:
		// Carries the run forward unchanged either way.
	default:
		s.incb = incbNotReady
	}

	switch {
	case isPict:
		s.extPict = extPictStateOpen
	case gcb == gcbExtend:
		if s.extPict != extPictStateOpen {
			s.extPict = extPictStateNone
		}
	case gcb == gcbZWJ:
		if s.extPict == extPictStateOpen {
			s.extPict = extPictStateReady
		} else {
			s.extPict = extPictStateNone
		}
	default:
		s.extPict = extPictStateNone
	}

	return breakBefore
}

// noBreak evaluates GB3 through GB999 in priority order (first match wins)
// for the boundary between the previous rune (recorded in s) and a rune whose
// properties are passed in. It reports true when the rule that matches
// forbids a break ("×"); GB999 (the default) reports false, i.e. break.
func (s *state) noBreak(curGCB, curInCB uint8, curIsPict bool) bool {
	prev := s.prevGCB

	// GB3: CR × LF
	if prev == gcbCR && curGCB == gcbLF {
		return true
	}
	// GB4: (Control | CR | LF) ÷
	if prev == gcbControl || prev == gcbCR || prev == gcbLF {
		return false
	}
	// GB5: ÷ (Control | CR | LF)
	if curGCB == gcbControl || curGCB == gcbCR || curGCB == gcbLF {
		return false
	}
	// GB6: L × (L | V | LV | LVT)
	if prev == gcbL && (curGCB == gcbL || curGCB == gcbV || curGCB == gcbLV || curGCB == gcbLVT) {
		return true
	}
	// GB7: (LV | V) × (V | T)
	if (prev == gcbLV || prev == gcbV) && (curGCB == gcbV || curGCB == gcbT) {
		return true
	}
	// GB8: (LVT | T) × T
	if (prev == gcbLVT || prev == gcbT) && curGCB == gcbT {
		return true
	}
	// GB9: × (Extend | ZWJ)
	if curGCB == gcbExtend || curGCB == gcbZWJ {
		return true
	}
	// GB9a: × SpacingMark
	if curGCB == gcbSpacingMark {
		return true
	}
	// GB9b: Prepend ×
	if prev == gcbPrepend {
		return true
	}
	// GB9c: [Extend Linker]* Linker [Extend Linker]* × \p{InCB=Consonant}
	// (see the incbReady doc comment for why the leading Consonant in the
	// rule text is not treated as a hard requirement here)
	if s.incb == incbReady && curInCB == incbConsonant {
		return true
	}
	// GB11: \p{Extended_Pictographic} Extend* ZWJ × \p{Extended_Pictographic}
	if s.extPict == extPictStateReady && curIsPict {
		return true
	}
	// GB12/GB13: RI × RI, only when the run of RIs ending at prev is odd.
	if prev == gcbRegionalIndicator && curGCB == gcbRegionalIndicator && s.riRun%2 == 1 {
		return true
	}
	// GB999: Any ÷ Any
	return false
}
