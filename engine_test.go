// Copyright (c) the go-ruby-rouge/rouge authors
//
// SPDX-License-Identifier: BSD-3-Clause

package rouge

import (
	"strings"
	"testing"

	onigmo "github.com/go-ruby-regexp/regexp"
)

// TestCompileREPanic checks that an unparseable pattern panics with a clear
// message (an authoring bug).
func TestCompileREPanic(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic on bad pattern")
		}
		if !strings.Contains(r.(string), "bad pattern") {
			t.Errorf("panic = %v", r)
		}
	}()
	compileRE("(") // unbalanced group
}

// TestGetStatePanic checks the unknown-state panic via a lexer that transitions
// to a state that was never defined.
func TestGetStatePanic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil || !strings.Contains(r.(string), "unknown state") {
			t.Fatalf("expected unknown-state panic, got %v", r)
		}
	}()
	lx := newRegexLexer("bad", "Bad")
	lx.state("root").rule(`a`, Text, push("missing"))
	lx.done().Lex("a")
}

// TestRegisterLexerDuplicatePanic checks that registering a duplicate tag panics.
func TestRegisterLexerDuplicatePanic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil || !strings.Contains(r.(string), "duplicate lexer tag") {
			t.Fatalf("expected duplicate-tag panic, got %v", r)
		}
	}()
	registerLexer(newRegexLexer("ruby", "Dup").done()) // "ruby" already registered
}

// TestEngineTransitions exercises every transition kind, the mixin path, the
// nil-group skip, pop-at-bottom (never empties the stack), pushSelf, goto, and
// inState, on a small synthetic lexer.
func TestEngineTransitions(t *testing.T) {
	lx := newRegexLexer("syn", "Syn")
	lx.start("root") // start that pushes root again (covers startPush)
	lx.state("root").
		mixin("nums").
		// nil-group skip: group 2 is dropped.
		groupsRule(`(a)(b)`, Keyword, nil).
		rule(`P`, Punctuation, pushSelf()). // pushSelf
		rule(`G`, Operator, gotoState("other")).
		rule(`x`, Text, pop()). // pop at/near bottom never empties
		rule(`\s+`, Text)
	lx.state("nums").
		rule(`\d+`, LiteralNumberInteger)
	lx.state("other").
		rule(`o`, NameOther, pop()).
		rule(`.`, Text)
	rl := lx.done()

	out := rl.Lex("12 ab P G o")
	// Sanity: the integer, the (a)(b) -> Keyword "a" (b skipped), etc. We mostly
	// care that it runs through every transition without panicking.
	if len(out) == 0 {
		t.Fatal("expected tokens")
	}
	// The "ab" rule emits Keyword "a" and skips group 2.
	var sawKeywordA bool
	for _, tv := range out {
		if tv.Token == Keyword && tv.Value == "a" {
			sawKeywordA = true
		}
		if tv.Token == Keyword && tv.Value == "b" {
			t.Error("group 2 (b) should have been skipped")
		}
	}
	if !sawKeywordA {
		t.Error("expected Keyword a from the nil-group rule")
	}
}

// TestEngineNullScan covers the maxNullScans guard: a state whose first rule is a
// zero-width match must eventually fail over to consuming an Error byte rather
// than looping forever.
func TestEngineNullScan(t *testing.T) {
	lx := newRegexLexer("nz", "NZ")
	// A zero-width-only state: the lookahead matches without consuming, so the
	// null-scan guard must trip and the byte is emitted as Error.
	lx.state("root").rule(`(?=z)`, Text)
	out := lx.done().Lex("z")
	if len(out) != 1 || out[0].Token != Error || out[0].Value != "z" {
		t.Errorf("null-scan guard: got %v, want one Error z", out)
	}
}

// TestEngineCbFallbackNull covers a fallthrough callback that declines, leaving
// the position unchanged, so the next rule handles the input.
func TestEngineCbFallback(t *testing.T) {
	lx := newRegexLexer("fb", "FB")
	lx.state("root").
		cbFall(`\w+`, func(l *lexState, m *onigmo.MatchData) bool {
			// Decline anything that is not "keep".
			if m.Str(0) != "keep" {
				return false
			}
			l.emit(Keyword, m.Str(0))
			l.pos = m.End(0)
			return true
		}).
		rule(`\w+`, Name) // the fallthrough target
	out := lx.done().Lex("keep other")
	// "keep" -> Keyword via cbFall; " " -> Error (no ws rule); "other" -> Name.
	var k, n bool
	for _, tv := range out {
		if tv.Token == Keyword && tv.Value == "keep" {
			k = true
		}
		if tv.Token == Name && tv.Value == "other" {
			n = true
		}
	}
	if !k || !n {
		t.Errorf("fallthrough: got %v", out)
	}
}

// TestCbFallZeroWidthAccept covers the branch where a fallthrough callback
// accepts the match but leaves pos unchanged (a zero-width accept), so the
// null-scan counter is bumped. After maxNullScans such accepts the state fails
// over to the Error byte.
func TestCbFallZeroWidthAccept(t *testing.T) {
	calls := 0
	lx := newRegexLexer("zw", "ZW")
	lx.state("root").
		cbFall(`(?=a)`, func(l *lexState, m *onigmo.MatchData) bool {
			calls++
			// Accept but do not advance: a zero-width accept.
			return true
		})
	out := lx.done().Lex("a")
	// The guard trips after maxNullScans, then the Error byte is consumed.
	if len(out) != 1 || out[0].Token != Error {
		t.Errorf("expected one Error after null-scan guard, got %v", out)
	}
	if calls == 0 {
		t.Error("fallthrough accept callback should have run")
	}
}

// TestInStateAndGoTo covers inState (true and false) and goTo via a tiny lexer.
func TestInStateAndGoTo(t *testing.T) {
	var inRootSeen, inDeepSeen bool
	lx := newRegexLexer("is", "IS")
	lx.state("root").
		cb(`a`, func(l *lexState, m *onigmo.MatchData) {
			inRootSeen = l.inState("root")   // true
			inDeepSeen = l.inState("absent") // false
			l.emit(Text, m.Str(0))
			l.pos = m.End(0)
			l.goTo("deep")
		}).
		rule(`.`, Text)
	lx.state("deep").rule(`.`, NameOther)
	lx.done().Lex("ab")
	if !inRootSeen || inDeepSeen {
		t.Errorf("inState root=%v absent=%v", inRootSeen, inDeepSeen)
	}
}
