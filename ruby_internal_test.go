// Copyright (c) the go-ruby-rouge/rouge authors
//
// SPDX-License-Identifier: BSD-3-Clause

package rouge

import "testing"

// TestMatchBalancedBrace covers both the balanced and the unbalanced (runs to
// end of input) arms of matchBalancedBrace.
func TestMatchBalancedBrace(t *testing.T) {
	// Balanced, with nesting: the '{' at index 0 closes at the matching '}'.
	if got := matchBalancedBrace("{a{b}c}d", 0); got != 7 {
		t.Errorf("balanced = %d, want 7", got)
	}
	// Unbalanced: no closing brace -> len(s).
	s := "{a{b}"
	if got := matchBalancedBrace(s, 0); got != len(s) {
		t.Errorf("unbalanced = %d, want %d", got, len(s))
	}
}

// TestSigilStringEdges drives the rubyLexer through %-sigil forms that exercise
// sigilString's branches: a non-interpolating %w word list (the # default arm),
// an interpolating %() with a nested-brace #{...}, an escape inside the body, an
// asymmetric nested delimiter, and an unterminated body that runs to EOF. These
// assert the lexer produces a token stream (no panic / full consumption) rather
// than gem parity, since the unterminated case is malformed Ruby.
func TestSigilStringEdges(t *testing.T) {
	for _, src := range []string{
		`%w(a b c)`,             // non-interp: '#'/default arm, symmetric ()
		`%(x #{y} z)`,           // interp with #{...}
		`%(esc \) done)`,        // escape inside body
		`%[outer [inner] tail]`, // asymmetric nested delimiter
		`%(unterminated body`,   // EOF before close
		`%r{a#{b}c}i`,           // regex sigil with interp + flags
	} {
		out := rubyLexer.Lex(src)
		if len(out) == 0 {
			t.Errorf("%q produced no tokens", src)
		}
		// The whole input must be consumed (values concatenate back to src).
		var b string
		for _, tv := range out {
			b += tv.Value
		}
		if b != src {
			t.Errorf("%q: reconstructed %q", src, b)
		}
	}
}
