package rouge

import (
	onigmo "github.com/go-ruby-regexp/regexp"
)

// maxNullScans bounds successive zero-width matches before a state is forced to
// fail, mirroring RegexLexer::MAX_NULL_SCANS.
const maxNullScans = 5

// action describes what a matched rule does to the token stream and the state
// stack. Exactly one of token-emission shapes applies: a single token for the
// whole match, or per-group tokens. After emitting, the listed state
// transitions run in order.
type action struct {
	// tok, when non-nil, is emitted for the whole match (group 0).
	tok *Token
	// groups, when non-nil, emits one token per capture group (group i -> token
	// groups[i]). A nil entry skips that group.
	groups []*Token
	// callback, when non-nil, fully overrides emission and is run with the
	// lexer; used for the handful of rules the gem implements with a block
	// (e.g. delegating sublexers, heredocs).
	callback func(l *lexState, m *onigmo.MatchData)
	// fall, when non-nil, is a callback that may decline the match: returning
	// false leaves the scan position unchanged and lets step try the next rule,
	// mirroring Rouge's fallthrough!. Returning true means handled (the callback
	// owns emission and advancing pos).
	fall func(l *lexState, m *onigmo.MatchData) bool
	// trans is the ordered list of state transitions to apply after emitting.
	trans []transition
}

// transition is one stack manipulation. kind selects which.
type transition struct {
	kind transKind
	// name is the target state for push, used only when kind == transPush.
	name string
}

type transKind int

const (
	transPush     transKind = iota // push state name onto the stack
	transPop                       // pop one state
	transPushSelf                  // push the current top state again (:push)
	transGoto                      // replace the top of the stack with state name
)

// rule is a compiled lexer rule: an anchored regex and the action to run on a
// match. mixin is non-empty for a mixin pseudo-rule, which splices in another
// state's rules in order.
type rule struct {
	re    *onigmo.Regexp
	act   action
	mixin string
}

// stateDef is a named, ordered list of rules, mirroring Rouge's State.
type stateDef struct {
	name  string
	rules []rule
}

// RegexLexer is a stateful, regex-rule lexer. A concrete lexer registers its
// states with newRegexLexer / state-builder helpers and exposes itself through
// the Lexer interface. It is immutable after construction and safe for
// concurrent use; per-lex mutable state lives in lexState.
type RegexLexer struct {
	tag      string
	title    string
	aliases  []string
	filename []string
	states   map[string]*stateDef
	// startPush lists extra states pushed onto the stack (after "root") when a
	// lex begins, mirroring Rouge's `start do push :x end` callback. Empty for
	// the common case where lexing simply starts in "root".
	startPush []string
	// detect, when non-nil, is a content sniffer used by Guess (Rouge's
	// self.detect?).
	detect func(text string) bool
}

// Tag returns the lexer's primary tag (e.g. "ruby").
func (rl *RegexLexer) Tag() string { return rl.tag }

// Title returns the lexer's human-readable title.
func (rl *RegexLexer) Title() string { return rl.title }

// Aliases returns the lexer's alternate names (e.g. "rb" for Ruby).
func (rl *RegexLexer) Aliases() []string { return rl.aliases }

// lexState holds the mutable per-lex state: the scan position, the state stack,
// and a null-scan counter. Each Lex call gets a fresh lexState.
type lexState struct {
	rl    *RegexLexer
	src   string
	pos   int
	stack []*stateDef
	null  int
	out   []TokenValue
	// heredocStr holds the active here-document terminator for the Shell lexer's
	// heredoc states; empty when none is active.
	heredocStr string
	// strReg is the f-string / quoted-string register stack used by the Python
	// lexer (each entry is a [type, delim] pair), mirroring its StringRegister.
	strReg [][2]string
	// indentStack, nextIndent, and blockScalarIndent hold the YAML lexer's
	// indentation state (the gem's @indent_stack / @next_indent /
	// @block_scalar_indent). blockScalarSet tracks whether blockScalarIndent has
	// been assigned (the gem's nil vs integer distinction).
	indentStack       []int
	nextIndent        int
	blockScalarIndent int
	blockScalarSet    bool
	// heredocQueue is the Ruby lexer's pending-heredoc queue (the gem's
	// @heredoc_queue): each entry is a (tolerant, terminator) pair awaiting its
	// body after the current line.
	heredocQueue []heredocEntry
}

// heredocEntry is one queued Ruby here-document: tolerant is true for <<- / <<~
// (indented terminator allowed), and name is the terminator word.
type heredocEntry struct {
	tolerant bool
	name     string
}

// TokenValue is a (token, value) pair in the lexed stream.
type TokenValue struct {
	Token *Token
	Value string
}

// Lex tokenizes text and returns the token stream, mirroring Rouge::Lexer#lex
// for a RegexLexer. The stream never contains empty values.
func (rl *RegexLexer) Lex(text string) []TokenValue {
	ls := &lexState{rl: rl, src: text, stack: []*stateDef{rl.getState("root")}}
	for _, name := range rl.startPush {
		ls.stack = append(ls.stack, rl.getState(name))
	}
	ls.run()
	return coalesce(ls.out)
}

// coalesce merges consecutive (token, value) pairs that carry the same token
// into one, mirroring Rouge::Lexer#continue_lex's "consolidate consecutive
// tokens of the same type" pass. Empty values are already dropped by emit, so
// this only joins runs. The input slice is not modified.
func coalesce(in []TokenValue) []TokenValue {
	if len(in) < 2 {
		return in
	}
	out := make([]TokenValue, 0, len(in))
	for _, tv := range in {
		if n := len(out); n > 0 && out[n-1].Token == tv.Token {
			out[n-1].Value += tv.Value
			continue
		}
		out = append(out, tv)
	}
	return out
}

// getState returns the named state, panicking on an unknown name (a lexer
// authoring bug, like Rouge raising "unknown state").
func (rl *RegexLexer) getState(name string) *stateDef {
	s, ok := rl.states[name]
	if !ok {
		panic("rouge: unknown state: " + name)
	}
	return s
}

// run drives the lex loop until the input is consumed: at each position it tries
// the current state's rules; on no match it emits one Error char and advances,
// exactly like RegexLexer#stream_tokens.
func (ls *lexState) run() {
	for ls.pos < len(ls.src) {
		if !ls.step(ls.stack[len(ls.stack)-1]) {
			// No rule matched: consume one byte as Error and continue.
			ls.emit(Error, ls.src[ls.pos:ls.pos+1])
			ls.pos++
		}
	}
	// At end of input, drain any pushed states whose top rule is a zero-width
	// cleanup (e.g. an empty-pattern pop): these terminate open constructs the way
	// Rouge's stream finalization does. Bounded by the stack depth and the
	// no-progress check so it always halts.
	for len(ls.stack) > 1 {
		before := len(ls.stack)
		if !ls.step(ls.stack[len(ls.stack)-1]) || len(ls.stack) >= before {
			break
		}
	}
}

// step tries each rule of state in order at the current position. A mixin rule
// recurses into the referenced state's rules. Returns true if a rule matched
// (and its action ran). This mirrors RegexLexer#step, including the null-scan
// guard.
func (ls *lexState) step(state *stateDef) bool {
	for i := range state.rules {
		r := &state.rules[i]
		if r.mixin != "" {
			if ls.step(ls.rl.getState(r.mixin)) {
				return true
			}
			continue
		}
		// MatchAt anchors \G at the cursor while keeping the whole buffer visible,
		// so ^/\A and lookbehind see the real prefix (StringScanner semantics). The
		// returned offsets are absolute into ls.src.
		m := r.re.MatchAt(ls.src, ls.pos)
		if m == nil {
			continue
		}
		// A fallthrough rule may decline the match; if it does, leave pos as-is
		// and keep trying subsequent rules (Rouge's fallthrough!). When it
		// accepts, the callback has already emitted and advanced pos, so the
		// null-scan guard is updated and the step is done.
		if r.act.fall != nil {
			before := ls.pos
			if !r.act.fall(ls, m) {
				continue
			}
			if ls.pos == before {
				ls.null++
				if ls.null > maxNullScans {
					return false
				}
			} else {
				ls.null = 0
			}
			return true
		}
		size := m.End(0) - ls.pos // absolute end minus cursor = match width
		if size == 0 {
			ls.null++
			if ls.null > maxNullScans {
				return false
			}
		} else {
			ls.null = 0
		}
		ls.apply(&r.act, m)
		return true
	}
	return false
}

// apply runs a matched rule's action: emit tokens, then perform state
// transitions, then advance the scan position past the whole match.
func (ls *lexState) apply(a *action, m *onigmo.MatchData) {
	if a.callback != nil {
		// The callback owns emission and position advancement.
		a.callback(ls, m)
		return
	}
	switch {
	case a.groups != nil:
		for i, t := range a.groups {
			if t == nil {
				continue
			}
			ls.emit(t, m.Str(i+1))
		}
	case a.tok != nil:
		ls.emit(a.tok, m.Str(0))
	}
	ls.pos = m.End(0)
	for _, tr := range a.trans {
		ls.doTrans(tr)
	}
}

// doTrans applies one stack transition.
func (ls *lexState) doTrans(tr transition) {
	switch tr.kind {
	case transPush:
		ls.stack = append(ls.stack, ls.rl.getState(tr.name))
	case transPushSelf:
		ls.stack = append(ls.stack, ls.stack[len(ls.stack)-1])
	case transPop:
		if len(ls.stack) > 1 {
			ls.stack = ls.stack[:len(ls.stack)-1]
		}
	case transGoto:
		ls.stack[len(ls.stack)-1] = ls.rl.getState(tr.name)
	}
}

// goto replaces the top of the stack with a named state (RegexLexer#goto).
func (ls *lexState) goTo(name string) {
	ls.stack[len(ls.stack)-1] = ls.rl.getState(name)
}

// emit appends a (token, value) pair, dropping empty values exactly like
// RegexLexer#yield_token.
func (ls *lexState) emit(t *Token, v string) {
	if v == "" {
		return
	}
	ls.out = append(ls.out, TokenValue{Token: t, Value: v})
}

// --- callback helpers used by lexers implemented with a block ---

// push pushes a named state (RegexLexer#push with a name).
func (ls *lexState) push(name string) {
	ls.stack = append(ls.stack, ls.rl.getState(name))
}

// pop pops n states (RegexLexer#pop!), never emptying the stack.
func (ls *lexState) pop(n int) {
	for ; n > 0 && len(ls.stack) > 1; n-- {
		ls.stack = ls.stack[:len(ls.stack)-1]
	}
}

// inState reports whether name is anywhere on the stack (RegexLexer#in_state?).
func (ls *lexState) inState(name string) bool {
	for _, s := range ls.stack {
		if s.name == name {
			return true
		}
	}
	return false
}

// delegate lexes text with another Lexer and splices its tokens into the
// stream, mirroring RegexLexer#delegate for the common (stateless re-entry)
// case used by the template lexers in this port.
func (ls *lexState) delegate(other Lexer, text string) {
	for _, tv := range other.Lex(text) {
		ls.emit(tv.Token, tv.Value)
	}
}

// recurse re-lexes text from this lexer's own root state and splices the tokens
// into the stream, mirroring RegexLexer#recurse (used for f-string
// interpolation in the Python lexer).
func (ls *lexState) recurse(text string) {
	sub := &lexState{rl: ls.rl, src: text, stack: []*stateDef{ls.rl.getState("root")}}
	sub.run()
	for _, tv := range sub.out {
		ls.emit(tv.Token, tv.Value)
	}
}

// --- state builder DSL ---
//
// These helpers compile Ruby-style state definitions into stateDef. Each rule's
// pattern is wrapped with \G so onigmo.Match anchors it to the current scan
// position, reproducing StringScanner#skip semantics. Authoring a lexer is a
// near-mechanical transcription of the gem's `state ... rule ...` blocks.

// lexerBuilder accumulates states while a lexer is defined.
type lexerBuilder struct {
	rl  *RegexLexer
	cur *stateDef
}

// newRegexLexer starts a new lexer definition with the given tag/title.
func newRegexLexer(tag, title string, aliases ...string) *lexerBuilder {
	rl := &RegexLexer{
		tag:     tag,
		title:   title,
		aliases: aliases,
		states:  map[string]*stateDef{},
	}
	return &lexerBuilder{rl: rl}
}

// filenames records glob patterns the lexer claims (used only for metadata).
func (b *lexerBuilder) filenames(globs ...string) *lexerBuilder {
	b.rl.filename = append(b.rl.filename, globs...)
	return b
}

// detect sets the content sniffer used by Guess.
func (b *lexerBuilder) detectWith(f func(string) bool) *lexerBuilder {
	b.rl.detect = f
	return b
}

// start records states to push onto the stack (after "root") when lexing
// begins, mirroring Rouge's `start do push :x end`.
func (b *lexerBuilder) start(names ...string) *lexerBuilder {
	b.rl.startPush = append(b.rl.startPush, names...)
	return b
}

// state begins a new named state. Subsequent rule/mixin calls add to it until
// the next state call. Mirrors RegexLexer.state.
func (b *lexerBuilder) state(name string) *lexerBuilder {
	s := &stateDef{name: name}
	b.rl.states[name] = s
	b.cur = s
	return b
}

// compileRE wraps pat with \G and compiles it, panicking on a malformed pattern
// (an authoring bug). The \G anchor pins the match to the scan position the way
// StringScanner with fixed_anchor does.
func compileRE(pat string) *onigmo.Regexp {
	re, err := onigmo.Compile(`\G(?:` + pat + `)`)
	if err != nil {
		panic("rouge: bad pattern " + pat + ": " + err.Error())
	}
	return re
}

// rule adds a single-token rule with optional state transitions. tok is the
// token for the whole match; trans are applied after emitting.
func (b *lexerBuilder) rule(pat string, tok *Token, trans ...transition) *lexerBuilder {
	b.cur.rules = append(b.cur.rules, rule{re: compileRE(pat), act: action{tok: tok, trans: trans}})
	return b
}

// rules adds a multi-group rule: group i+1 is emitted as toks[i]. A nil token
// skips that group. Transitions can follow via ruleGroupsT.
func (b *lexerBuilder) groupsRule(pat string, toks ...*Token) *lexerBuilder {
	b.cur.rules = append(b.cur.rules, rule{re: compileRE(pat), act: action{groups: toks}})
	return b
}

// groupsRuleT is groupsRule with trailing state transitions.
func (b *lexerBuilder) groupsRuleT(pat string, trans []transition, toks ...*Token) *lexerBuilder {
	b.cur.rules = append(b.cur.rules, rule{re: compileRE(pat), act: action{groups: toks, trans: trans}})
	return b
}

// cb adds a rule whose action is an arbitrary callback (for the block-form
// rules the gem uses). The callback must advance ls.pos itself.
func (b *lexerBuilder) cb(pat string, f func(l *lexState, m *onigmo.MatchData)) *lexerBuilder {
	b.cur.rules = append(b.cur.rules, rule{re: compileRE(pat), act: action{callback: f}})
	return b
}

// cbFall adds a rule whose callback may decline the match (Rouge's
// fallthrough!): returning false leaves pos unchanged and lets step try the next
// rule; returning true means handled and the callback must have emitted and
// advanced pos.
func (b *lexerBuilder) cbFall(pat string, f func(l *lexState, m *onigmo.MatchData) bool) *lexerBuilder {
	b.cur.rules = append(b.cur.rules, rule{re: compileRE(pat), act: action{fall: f}})
	return b
}

// mixin splices another state's rules in order at this point (RegexLexer mixin).
func (b *lexerBuilder) mixin(name string) *lexerBuilder {
	b.cur.rules = append(b.cur.rules, rule{mixin: name})
	return b
}

// done finalizes and returns the lexer.
func (b *lexerBuilder) done() *RegexLexer { return b.rl }

// transition constructors, named to read like the gem's :push / :pop! / a state
// symbol.
func push(name string) transition      { return transition{kind: transPush, name: name} }
func pop() transition                  { return transition{kind: transPop} }
func pushSelf() transition             { return transition{kind: transPushSelf} }
func gotoState(name string) transition { return transition{kind: transGoto, name: name} }
