package graphemebreak

import (
	"bufio"
	"io"
	"unicode/utf8"
)

// Reader scans an io.Reader for extended grapheme clusters, so that only the
// current cluster needs to be held in memory rather than the whole input.
// It follows the bufio.Scanner calling convention: call Next repeatedly
// until it returns false, then check Err to distinguish end of input from a
// read error.
//
// Reader decodes with bufio.Reader.ReadRune, which replaces invalid UTF-8
// bytes with U+FFFD (one byte consumed per replacement); on input containing
// invalid UTF-8, the text returned by Grapheme is therefore not guaranteed to
// reproduce the original bytes. On valid UTF-8 input, concatenating every
// cluster Reader yields reproduces the input exactly.
type Reader struct {
	br  *bufio.Reader
	st  state
	buf []byte

	pending  []byte
	havePend bool
	err      error
	done     bool
}

// NewReader returns a Reader that scans r for grapheme clusters.
func NewReader(r io.Reader) *Reader {
	return &Reader{br: bufio.NewReader(r)}
}

// Next advances to the next grapheme cluster and reports whether one was
// found. It returns false at end of input or on a read error; call Err
// afterward to distinguish the two.
func (r *Reader) Next() bool {
	if r.done {
		return false
	}
	r.buf = r.buf[:0]

	if r.havePend {
		r.buf = append(r.buf, r.pending...)
		r.havePend = false
	} else {
		ru, size, err := r.br.ReadRune()
		if err != nil {
			r.done = true
			if err != io.EOF {
				r.err = err
			}
			return false
		}
		r.st.step(ru)
		r.buf = utf8.AppendRune(r.buf[:0], ru)
		_ = size
	}

	for {
		ru, _, err := r.br.ReadRune()
		if err != nil {
			r.done = true
			if err != io.EOF {
				r.err = err
			}
			return true
		}
		if r.st.step(ru) {
			r.pending = utf8.AppendRune(r.pending[:0], ru)
			r.havePend = true
			return true
		}
		r.buf = utf8.AppendRune(r.buf, ru)
	}
}

// Grapheme returns the most recent grapheme cluster found by Next, as a
// byte slice valid until the next call to Next.
func (r *Reader) Grapheme() []byte {
	return r.buf
}

// GraphemeString returns the most recent grapheme cluster found by Next.
func (r *Reader) GraphemeString() string {
	return string(r.buf)
}

// Err returns the first non-EOF error encountered while reading, if any.
func (r *Reader) Err() error {
	return r.err
}
