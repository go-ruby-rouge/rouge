// Copyright (c) the go-ruby-rouge/rouge authors
//
// SPDX-License-Identifier: BSD-3-Clause

package rouge

import (
	onigmo "github.com/go-ruby-regexp/regexp"
)

// --- YAML ---
// A faithful transcription of Rouge::Lexers::YAML (rouge 5.0.0), including its
// indentation state machine. The gem tracks @indent_stack / @next_indent /
// @block_scalar_indent on the lexer instance; here those live on lexState and
// are manipulated by the helper methods below, which mirror reset_indent,
// save_indent, set_indent, and continue_indent exactly.

// yamlReset resets the indentation state (reset_indent).
func (l *lexState) yamlReset() {
	l.indentStack = []int{0}
	l.nextIndent = 0
	l.blockScalarSet = false
}

// yamlIndent is the current indentation level (the gem's #indent).
func (l *lexState) yamlIndent() int {
	if len(l.indentStack) == 0 {
		return 0
	}
	return l.indentStack[len(l.indentStack)-1]
}

// yamlSaveIndent mirrors save_indent: records the match size as the next indent,
// pops the stack while dedenting, and returns the (text, err) split of match
// when dedenting to a level not previously indented to.
func (l *lexState) yamlSaveIndent(match string) (string, string) {
	l.nextIndent = len(match)
	if l.nextIndent < l.yamlIndent() {
		for l.nextIndent < l.yamlIndent() && len(l.indentStack) > 1 {
			l.indentStack = l.indentStack[:len(l.indentStack)-1]
		}
		ind := l.yamlIndent()
		if ind > len(match) {
			ind = len(match)
		}
		return match[:ind], match[ind:]
	}
	return match, ""
}

// yamlContinueIndent mirrors continue_indent.
func (l *lexState) yamlContinueIndent(match string) { l.nextIndent += len(match) }

// yamlSetIndent mirrors set_indent: push nextIndent when it exceeds the current
// indent, and (unless implicit) advance nextIndent by the match size.
func (l *lexState) yamlSetIndent(match string, implicit bool) {
	if l.yamlIndent() < l.nextIndent {
		l.indentStack = append(l.indentStack, l.nextIndent)
	}
	if !implicit {
		l.nextIndent += len(match)
	}
}

// plainScalarStart is the gem's plain_scalar_start character class.
const yamlPlainScalarStart = "[^ \\t\\n\\r\\f\\v?:,\\[\\]{}#&*!|>'\"%@`]"

var yamlLexer = func() *RegexLexer {
	b := newRegexLexer("yaml", "YAML", "yml")
	b.filenames("*.yaml", "*.yml")
	b.detectWith(func(text string) bool {
		if re, err := onigmo.Compile(`(?m)\A\s*%YAML`); err == nil && re.MatchString(text) {
			return true
		}
		return false
	})

	b.state("basic").
		rule(`#.*$`, CommentSingle)

	b.state("root").
		mixin("basic").
		rule(`\n+`, Text).
		rule(`[ ]+(?=#|$)`, Text).
		cb(`^%YAML\b`, func(l *lexState, m *onigmo.MatchData) {
			l.emit(NameTag, m.Str(0))
			l.pos = m.End(0)
			l.yamlReset()
			l.push("yaml_directive")
		}).
		cb(`^%TAG\b`, func(l *lexState, m *onigmo.MatchData) {
			l.emit(NameTag, m.Str(0))
			l.pos = m.End(0)
			l.yamlReset()
			l.push("tag_directive")
		}).
		cb(`^(?:---|\.\.\.)(?= |$)`, func(l *lexState, m *onigmo.MatchData) {
			l.emit(NameNamespace, m.Str(0))
			l.pos = m.End(0)
			l.yamlReset()
			l.push("block_line")
		}).
		cb(`[ ]*(?!\s|$)`, func(l *lexState, m *onigmo.MatchData) {
			text, err := l.yamlSaveIndent(m.Str(0))
			l.emit(Text, text)
			l.emit(Error, err)
			l.pos = m.End(0)
			l.push("block_line")
			l.push("indentation")
		})

	b.state("indentation").
		cb(`\s*?\n`, func(l *lexState, m *onigmo.MatchData) {
			l.emit(Text, m.Str(0))
			l.pos = m.End(0)
			l.pop(2)
		}).
		cb(`[ ]+(?=[-:?](?:[ ]|$))`, func(l *lexState, m *onigmo.MatchData) {
			l.emit(Text, m.Str(0))
			l.pos = m.End(0)
			l.yamlContinueIndent(m.Str(0))
		}).
		cb(`[?:-](?=[ ]|$)`, func(l *lexState, m *onigmo.MatchData) {
			l.yamlSetIndent(m.Str(0), false)
			l.emit(PunctuationIndicator, m.Str(0))
			l.pos = m.End(0)
		}).
		cb(`[ ]*`, func(l *lexState, m *onigmo.MatchData) {
			l.emit(Text, m.Str(0))
			l.pos = m.End(0)
			l.yamlContinueIndent(m.Str(0))
			l.pop(1)
		})

	b.state("block_line").
		rule(`[ ]*(?=#|$)`, Text, pop()).
		rule(`[ ]+`, Text).
		mixin("descriptors").
		mixin("block_nodes").
		mixin("flow_nodes").
		cb(`(?=`+yamlPlainScalarStart+`|[?:-][^ \t\n\r\f\v])`, func(l *lexState, m *onigmo.MatchData) {
			l.emit(NameVariable, m.Str(0))
			l.push("plain_scalar_in_block_context")
		})

	b.state("descriptors").
		rule(`!<[0-9A-Za-z;/?:@&=+$,_.!~*'()\[\]%-]+>`, KeywordType).
		rule(`(?:![\w-]+)?!(?:[\w;/?:@&=+$,.!~*'()\[\]%-]*)`, KeywordType).
		rule(`&[\p{L}\p{Nl}\p{Nd}_-]+`, NameLabel).
		rule(`\*[\p{L}\p{Nl}\p{Nd}_-]+`, NameVariable)

	b.state("block_nodes").
		cb(`([^#,?\[\]{}"'\n]+)(:)(?=\s|$)`, func(l *lexState, m *onigmo.MatchData) {
			l.emit(NameAttribute, m.Str(1))
			l.emit(PunctuationIndicator, m.Str(2))
			l.yamlSetIndent(m.Str(0), true)
			l.pos = m.End(0)
		}).
		cb(`("(?:[^\n"]|\\")*")(\s*)(:)(?=\s|$)`, func(l *lexState, m *onigmo.MatchData) {
			l.emit(NameAttribute, m.Str(1))
			l.emit(Text, m.Str(2))
			l.emit(PunctuationIndicator, m.Str(3))
			l.yamlSetIndent(m.Str(0), true)
			l.pos = m.End(0)
		}).
		cb(`('(?:[^\n']|\\')*')(\s*)(:)(?=\s|$)`, func(l *lexState, m *onigmo.MatchData) {
			l.emit(NameAttribute, m.Str(1))
			l.emit(Text, m.Str(2))
			l.emit(PunctuationIndicator, m.Str(3))
			l.yamlSetIndent(m.Str(0), true)
			l.pos = m.End(0)
		}).
		rule(`[|>][+-]?`, PunctuationIndicator, push("block_scalar_content"), push("block_scalar_header"))

	b.state("flow_nodes").
		rule(`\[`, PunctuationIndicator, push("flow_sequence")).
		rule(`\{`, PunctuationIndicator, push("flow_mapping")).
		rule(`'`, LiteralStringSingle, push("single_quoted_scalar")).
		rule(`"`, LiteralStringDouble, push("double_quoted_scalar"))

	b.state("flow_collection").
		rule(`(?m)\s+`, Text).
		mixin("basic").
		rule(`[?:,]`, PunctuationIndicator).
		mixin("descriptors").
		mixin("flow_nodes").
		cb(`(?=`+yamlPlainScalarStart+`)`, func(l *lexState, m *onigmo.MatchData) {
			l.push("plain_scalar_in_flow_context")
		})

	b.state("flow_sequence").
		rule(`\]`, PunctuationIndicator, pop()).
		mixin("flow_collection")

	b.state("flow_mapping").
		rule(`\}`, PunctuationIndicator, pop()).
		mixin("flow_collection")

	b.state("block_scalar_content").
		rule(`\n+`, Text).
		cb(`^[ ]+$`, func(l *lexState, m *onigmo.MatchData) {
			text := m.Str(0)
			indentMark := len(text)
			if l.blockScalarSet {
				indentMark = l.blockScalarIndent
			}
			if indentMark > len(text) {
				indentMark = len(text)
			}
			l.emit(Text, text[:indentMark])
			l.emit(NameConstant, text[indentMark:])
			l.pos = m.End(0)
		}).
		cb(`^[ ]*`, func(l *lexState, m *onigmo.MatchData) {
			l.emit(Text, m.Str(0))
			l.pos = m.End(0)
			indentSize := len(m.Str(0))
			dedentLevel := l.yamlIndent()
			if l.blockScalarSet {
				dedentLevel = l.blockScalarIndent
			}
			if !l.blockScalarSet {
				l.blockScalarIndent = indentSize
				l.blockScalarSet = true
			}
			if indentSize < dedentLevel {
				l.yamlSaveIndent(m.Str(0))
				l.pop(1)
				l.push("indentation")
			}
		}).
		rule(`[^\n\r\f\v]+`, LiteralString)

	b.state("block_scalar_header").
		cb(`(?:([1-9])[+-]?|[+-]?([1-9])?)(?=[ ]|$)`, func(l *lexState, m *onigmo.MatchData) {
			l.blockScalarSet = false
			l.goTo("ignored_line")
			if m.Str(0) == "" {
				l.pos = m.End(0)
				return
			}
			inc := m.Str(1)
			if inc == "" {
				inc = m.Str(2)
			}
			if inc != "" {
				l.blockScalarIndent = l.yamlIndent() + int(inc[0]-'0')
				l.blockScalarSet = true
			}
			l.emit(PunctuationIndicator, m.Str(0))
			l.pos = m.End(0)
		})

	b.state("ignored_line").
		mixin("basic").
		rule(`[ ]+`, Text).
		rule(`\n`, Text, pop())

	b.state("quoted_scalar_whitespaces").
		rule(`^[ ]+`, Text).
		rule(`[ ]+$`, Text).
		rule(`(?m)\n+`, Text).
		rule(`[ ]+`, NameVariable)

	b.state("single_quoted_scalar").
		mixin("quoted_scalar_whitespaces").
		rule(`\\'`, LiteralStringEscape).
		rule(`'`, LiteralString, pop()).
		rule(`[^\s']+`, LiteralString)

	b.state("double_quoted_scalar").
		rule(`"`, LiteralString, pop()).
		mixin("quoted_scalar_whitespaces").
		rule(`\\[0abt\tn\nvfre "\\N_LP]`, LiteralStringEscape).
		rule(`\\(?:x[0-9A-Fa-f]{2}|u[0-9A-Fa-f]{4}|U[0-9A-Fa-f]{8})`, LiteralStringEscape).
		rule(`[^ \t\n\r\f\v"\\]+`, LiteralString)

	b.state("plain_scalar_in_block_context_new_line").
		rule(`^[ ]+\n`, Text).
		rule(`(?m)\n+`, Text).
		cb(`^(?=---|\.\.\.)`, func(l *lexState, m *onigmo.MatchData) { l.pop(3) }).
		cb(`^[ ]*`, func(l *lexState, m *onigmo.MatchData) {
			l.emit(Text, m.Str(0))
			l.pos = m.End(0)
			l.pop(1)
			indentSize := len(m.Str(0))
			if indentSize <= l.yamlIndent() {
				l.pop(1)
				l.yamlSaveIndent(m.Str(0))
				l.push("indentation")
			}
		})

	b.state("plain_scalar_in_block_context").
		rule(`[ ]*(?=:[ \n]|:$)`, Text, pop()).
		rule(`[ ]*:\S+`, LiteralString).
		rule(`[ ]+(?=#)`, Text, pop()).
		rule(`[ ]+$`, Text).
		cb(`\n+`, func(l *lexState, m *onigmo.MatchData) {
			l.emit(Text, m.Str(0))
			l.pos = m.End(0)
			l.push("plain_scalar_in_block_context_new_line")
		}).
		rule(`[ ]+`, LiteralString).
		rule(`(?:true|false|null)\b`, KeywordConstant).
		rule(`\d+(?:\.\d+)?(?=(?:\r?\n)| +#)`, LiteralNumber, pop()).
		rule(`[^\s:]+`, LiteralString)

	b.state("plain_scalar_in_flow_context").
		rule(`[ ]*(?=[,:?\[\]{}])`, Text, pop()).
		rule(`[ ]+(?=#)`, Text, pop()).
		rule(`^[ ]+`, Text).
		rule(`[ ]+$`, Text).
		rule(`\n+`, Text).
		rule(`[ ]+`, NameVariable).
		rule(`[^\s,:?\[\]{}]+`, NameVariable)

	b.state("yaml_directive").
		cb(`([ ]+)(\d+\.\d+)`, func(l *lexState, m *onigmo.MatchData) {
			l.emit(Text, m.Str(1))
			l.emit(LiteralNumber, m.Str(2))
			l.pos = m.End(0)
			l.goTo("ignored_line")
		})

	b.state("tag_directive").
		cb(`([ ]+)(!|![\w-]*!)([ ]+)(!|!?[\w;/?:@&=+$,.!~*'()\[\]%-]+)`, func(l *lexState, m *onigmo.MatchData) {
			l.emit(Text, m.Str(1))
			l.emit(KeywordType, m.Str(2))
			l.emit(Text, m.Str(3))
			l.emit(KeywordType, m.Str(4))
			l.pos = m.End(0)
			l.goTo("ignored_line")
		})

	return b.done()
}()
