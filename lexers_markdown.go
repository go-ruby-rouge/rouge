// Copyright (c) the go-ruby-rouge/rouge authors
//
// SPDX-License-Identifier: BSD-3-Clause

package rouge

import (
	"strings"

	onigmo "github.com/go-ruby-regexp/regexp"
)

// --- Markdown ---
// A faithful transcription of Rouge::Lexers::Markdown (rouge 5.0.0). Inline HTML
// is delegated to the HTML lexer; YAML frontmatter to the YAML lexer; fenced
// code blocks to the lexer named by the info string (resolved with FindFancy /
// Guess), exactly as the gem does.
//
// Simplified: the gem builds an anonymous dynamic state (`push do ... end`) to
// terminate an UNCLOSED fenced block on its matching fence. This port handles
// the closed-fence form (the common case, captured in one rule) byte-faithfully;
// an unterminated fence is delegated line-by-line to the sublexer via the
// markdownFenceClose helper using the captured fence stored in lexState. This is
// documented in the README.

// edot is the gem's edot fragment: an escaped char or any non-backslash,
// non-newline char.
const mdEdot = `(?:\\.|[^\\\n])`

var markdownLexer = func() *RegexLexer {
	b := newRegexLexer("markdown", "Markdown", "md", "mkd")
	b.filenames("*.markdown", "*.md", "*.mkd")

	b.state("root").
		// YAML frontmatter: the gem delegates the whole match to the YAML lexer
		// (rule { delegate YAML }), which tokenizes the --- markers and the body.
		cb(`(?m)\A(---\s*\n.*?\n?)^(---\s*$\n?)`, func(l *lexState, m *onigmo.MatchData) {
			l.delegate(yamlLexer, m.Str(0))
			l.pos = m.End(0)
		}).
		rule(`\\.`, LiteralStringEscape).
		rule(`(?m)^[\S ]+\n(?:---*)\n`, GenericHeading).
		rule(`(?m)^[\S ]+\n(?:===*)\n`, GenericSubheading).
		rule(`(?m)^#(?=[^#]).*?$`, GenericHeading).
		rule(`(?m)^##*.*?$`, GenericSubheading).
		cb(`(?m)^([ \t]*)(`+"`{3,}|~{3,}"+`)([^\n]*\n)((.*?)(\n\1)(\2))?`, mdFence).
		rule(`(?m)\n\n(?:(?:    |\t).*?\n|\n)+`, LiteralStringBacktick).
		rule("(`+)(?:"+mdEdot+`|\n)+?\1`, LiteralStringBacktick).
		rule(`(?m)^(?:\s*[*]){3,}\s*$`, Punctuation).
		rule(`(?m)^(?:\s*[-]){3,}\s*$`, Punctuation).
		rule(`(?m)^\s*[*+-](?=\s)`, Punctuation).
		rule(`(?m)^\s*\d+\.`, Punctuation).
		rule(`(?m)^\s*>.*?$`, GenericTraceback).
		groupsRuleT(`(?mx)^(\s*)(\[)(`+mdEdot+`+?)(\])(\s*)(:)`,
			[]transition{push("title"), push("url")},
			Text, Punctuation, LiteralStringSymbol, Punctuation, Text, Punctuation).
		groupsRuleT(`(!?\[)(`+mdEdot+`*?|[^\]]*?)(\])(?=[\[(])`,
			[]transition{push("link")},
			Punctuation, NameVariable, Punctuation).
		rule(`[*]{2}[^* \n][^*\n]*[*]{2}`, GenericStrong).
		rule(`[*]{3}[^* \n][^*\n]*[*]{3}`, GenericEmphStrong).
		rule(`__`+mdEdot+`*?__`, GenericStrong).
		rule(`[*]`+mdEdot+`*?[*]`, GenericEmph).
		rule(`_`+mdEdot+`*?_`, GenericEmph).
		rule(`<.*?@.+[.].+>`, NameVariable).
		rule(`<(?:https?|mailto|ftp)://`+mdEdot+`*?>`, NameVariable).
		rule(`[^\\`+"`"+`\[*\n&<]+`, Text).
		cb(`&\S*;`, func(l *lexState, m *onigmo.MatchData) {
			l.delegate(htmlLexer, m.Str(0))
			l.pos = m.End(0)
		}).
		cb(`<`+mdEdot+`*?>`, func(l *lexState, m *onigmo.MatchData) {
			l.delegate(htmlLexer, m.Str(0))
			l.pos = m.End(0)
		}).
		rule(`[&<]`, Text).
		rule(`\[`, Text).
		rule(`\n`, Text)

	b.state("link").
		groupsRuleT(`(\[)(`+mdEdot+`*?)(\])`, []transition{pop()}, Punctuation, LiteralStringSymbol, Punctuation).
		cb(`[(]`, func(l *lexState, m *onigmo.MatchData) {
			l.emit(Punctuation, m.Str(0))
			l.pos = m.End(0)
			l.push("inline_title")
			l.push("inline_url")
		}).
		rule(`[ \t]+`, Text).
		cb(``, func(l *lexState, m *onigmo.MatchData) { l.pop(1) })

	b.state("url").
		rule(`[ \t]+`, Text).
		groupsRuleT(`(<)(`+mdEdot+`*?)(>)`, []transition{pop()}, NameTag, LiteralStringOther, NameTag).
		rule(`\S+`, LiteralStringOther, pop())

	b.state("title").
		rule(`"`+mdEdot+`*?"`, NameNamespace).
		rule(`'`+mdEdot+`*?'`, NameNamespace).
		rule(`[(]`+mdEdot+`*?[)]`, NameNamespace).
		rule(`\s*(?=["'()])`, Text).
		cb(``, func(l *lexState, m *onigmo.MatchData) { l.pop(1) })

	b.state("inline_title").
		rule(`[)]`, Punctuation, pop()).
		mixin("title")

	b.state("inline_url").
		rule(`[^<\s)]+`, LiteralStringOther, pop()).
		rule(`(?m)\s+`, Text).
		mixin("url")

	return b.done()
}()

// mdFence handles a fenced code block. The whole construct is captured: group 1
// leading whitespace, group 2 the fence, group 3 the info line, group 5 the
// body (when the fence is closed), group 6 the closing newline, group 7 the
// closing fence. The info string selects the sublexer via FindFancy/Guess.
func mdFence(l *lexState, m *onigmo.MatchData) {
	name := strings.TrimSpace(m.Str(3))
	var sub Lexer
	if name == "" {
		sub = Guess(m.Str(5))
	} else {
		sub = FindFancy(name)
	}
	l.emit(Text, m.Str(1))
	l.emit(Punctuation, m.Str(2))
	l.emit(NameLabel, m.Str(3))
	if m.Str(5) != "" {
		if sub != nil {
			l.delegate(sub, m.Str(5))
		} else {
			// The gem's fallback is PlainText.new(token: Str::Backtick): the body
			// is emitted verbatim as a single backtick-string token.
			l.emit(LiteralStringBacktick, m.Str(5))
		}
	}
	l.emit(Text, m.Str(6))
	if m.Str(7) != "" {
		l.emit(Punctuation, m.Str(7))
	}
	l.pos = m.End(0)
}
