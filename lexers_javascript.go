// Copyright (c) the go-ruby-rouge/rouge authors
//
// SPDX-License-Identifier: BSD-3-Clause

package rouge

import (
	"strings"

	onigmo "github.com/go-ruby-regexp/regexp"
)

// --- JavaScript ---
// A faithful transcription of Rouge::Lexers::Javascript (rouge 5.0.0), including
// its regex / template-string / object-literal / ternary state machine. The
// keyword / declaration / reserved / constant / builtin word sets are the gem's.

var jsKeywords = stringSet(`async await break case catch continue debugger
default delete do else export finally from for if import in instanceof new of
return super switch this throw try typeof void while yield`)

var jsDeclarations = stringSet(`var let const with function class extends
constructor get set static`)

var jsReserved = stringSet(`enum implements interface package private protected
public`)

var jsConstants = stringSet(`true false null NaN Infinity undefined`)

var jsBuiltins = stringSet(`Array Boolean Date Error Function Math netscape
Number Object Packages RegExp String sun decodeURI decodeURIComponent encodeURI
encodeURIComponent Error eval isFinite isNaN parseFloat parseInt document window
navigator self global Promise Set Map WeakSet WeakMap Symbol Proxy Reflect
Int8Array Uint8Array Uint8ClampedArray Int16Array Uint16Array Uint16ClampedArray
Int32Array Uint32Array Uint32ClampedArray Float32Array Float64Array DataView
ArrayBuffer`)

// jsID is the gem's id_regex: /[\p{L}\p{Nl}$_][\p{Word}]*/.
const jsID = `[\p{L}\p{Nl}$_][\p{Word}]*`

var javascriptLexer = func() *RegexLexer {
	b := newRegexLexer("javascript", "JavaScript", "js")
	b.filenames("*.cjs", "*.js", "*.mjs")
	b.detectWith(func(text string) bool {
		if strings.HasPrefix(text, "#!") {
			line := text
			if nl := strings.IndexByte(text, '\n'); nl >= 0 {
				line = text[:nl]
			}
			return strings.Contains(line, "node") || strings.Contains(line, "jsc")
		}
		return false
	})

	b.state("multiline_comment").
		rule(`[*]/`, CommentMultiline, pop()).
		rule(`[^*/]+`, CommentMultiline).
		rule(`[*/]`, CommentMultiline)

	b.state("comments_and_whitespace").
		rule(`\s+`, Text).
		rule(`<!--`, Comment).
		rule(`//.*?$`, CommentSingle).
		rule(`/[*]`, CommentMultiline, push("multiline_comment"))

	b.state("expr_start").
		mixin("comments_and_whitespace").
		cb(`/`, func(l *lexState, m *onigmo.MatchData) {
			l.emit(LiteralStringRegex, m.Str(0))
			l.pos = m.End(0)
			l.goTo("regex")
		}).
		cb(`[{]`, func(l *lexState, m *onigmo.MatchData) {
			l.emit(Punctuation, m.Str(0))
			l.pos = m.End(0)
			l.goTo("object")
		}).
		rule(``, Text, pop())

	b.state("regex").
		cb(`/`, func(l *lexState, m *onigmo.MatchData) {
			l.emit(LiteralStringRegex, m.Str(0))
			l.pos = m.End(0)
			l.goTo("regex_end")
		}).
		rule(`[^/]\n`, Error, pop()).
		rule(`\n`, Error, pop()).
		rule(`\[\^`, LiteralStringEscape, push("regex_group")).
		rule(`\[`, LiteralStringEscape, push("regex_group")).
		rule(`\\.`, LiteralStringEscape).
		rule(`[(][?][:=<!]`, LiteralStringEscape).
		rule(`[{][\d,]+[}]`, LiteralStringEscape).
		rule(`[()?]`, LiteralStringEscape).
		rule(`.`, LiteralStringRegex)

	b.state("regex_end").
		rule(`[gimuy]+`, LiteralStringRegex, pop()).
		cb(``, func(l *lexState, m *onigmo.MatchData) { l.pop(1) })

	b.state("regex_group").
		rule(`/`, LiteralStringEscape).
		cb(`[^/]\n`, func(l *lexState, m *onigmo.MatchData) {
			l.emit(Error, m.Str(0))
			l.pos = m.End(0)
			l.pop(2)
		}).
		rule(`\]`, LiteralStringEscape, pop()).
		rule(`\\.`, LiteralStringEscape).
		rule(`.`, LiteralStringRegex)

	b.state("bad_regex").
		rule(`[^\n]+`, Error, pop())

	b.state("root").
		rule(`(?m)\A\s*#!.*?\n`, CommentPreproc, push("statement")).
		rule(`(?<=\n)(?=\s|/|<!--)`, Text, push("expr_start")).
		mixin("comments_and_whitespace").
		rule(`(?x)\+\+ | -- | ~ | \?\?=? | && | \|\| | \\(?=\n) | << | >>>? | === | !== `, Operator, push("expr_start")).
		rule(`[-<>+*%&|^/!=]=?`, Operator, push("expr_start")).
		rule(`[(\[,]`, Punctuation, push("expr_start")).
		rule(`;`, Punctuation, push("statement")).
		rule(`[)\].]`, Punctuation).
		rule("`", LiteralStringDouble, push("template_string")).
		rule(`[?][.]`, Punctuation).
		cb(`[?]`, func(l *lexState, m *onigmo.MatchData) {
			l.emit(Punctuation, m.Str(0))
			l.pos = m.End(0)
			l.push("ternary")
			l.push("expr_start")
		}).
		groupsRuleT(`(@)(\w+)?`, []transition{push("expr_start")}, Punctuation, NameDecorator).
		groupsRuleT(`(class)((?:\s|\\\s)+)`, []transition{push("classname")}, KeywordDeclaration, Text).
		rule(`(?m)[\p{Nl}$_]*\p{Lu}[\p{Word}]*[ \t]*(?=(?:\(.*\)))`, NameClass).
		groupsRule(`(function)((?:\s|\\\s)+)(`+jsID+`)`, KeywordDeclaration, Text, NameFunction).
		rule(`function(?=(?:\(.*\)))`, KeywordDeclaration).
		cb(`(?m)(#?`+jsID+`)[ \t]*(?=(?:\(.*\)))`, func(l *lexState, m *onigmo.MatchData) {
			if jsKeywords[m.Str(1)] {
				l.emit(Keyword, m.Str(0))
			} else {
				l.emit(NameFunction, m.Str(0))
			}
			l.pos = m.End(0)
		}).
		rule(`[{}]`, Punctuation, push("statement")).
		cb(`#?`+jsID, func(l *lexState, m *onigmo.MatchData) {
			w := m.Str(0)
			switch {
			case jsKeywords[w]:
				l.emit(Keyword, w)
				l.pos = m.End(0)
				l.push("expr_start")
				return
			case jsDeclarations[w]:
				l.emit(KeywordDeclaration, w)
				l.pos = m.End(0)
				l.push("expr_start")
				return
			case jsReserved[w]:
				l.emit(KeywordReserved, w)
			case jsConstants[w]:
				l.emit(KeywordConstant, w)
			case jsBuiltins[w]:
				l.emit(NameBuiltin, w)
			default:
				l.emit(NameOther, w)
			}
			l.pos = m.End(0)
		}).
		rule(`[0-9][0-9]*\.[0-9]+(?:[eE][0-9]+)?[fd]?`, LiteralNumberFloat).
		rule(`(?i)0x[0-9a-fA-F]+`, LiteralNumberHex).
		rule(`(?i)0o[0-7][0-7_]*`, LiteralNumberOct).
		rule(`(?i)0b[01][01_]*`, LiteralNumberBin).
		rule(`[0-9]+`, LiteralNumberInteger).
		rule(`"`, LiteralStringDelimiter, push("dq")).
		rule(`'`, LiteralStringDelimiter, push("sq")).
		rule(`:`, Punctuation)

	b.state("dq").
		rule(`\\[\\nrt"]?`, LiteralStringEscape).
		rule(`[^\\"]+`, LiteralStringDouble).
		rule(`"`, LiteralStringDelimiter, pop())

	b.state("sq").
		rule(`\\[\\nrt']?`, LiteralStringEscape).
		rule(`[^\\']+`, LiteralStringSingle).
		rule(`'`, LiteralStringDelimiter, pop())

	b.state("classname").
		groupsRule(`(`+jsID+`)((?:\s|\\\s)+)(extends)((?:\s|\\\s)+)`, NameClass, Text, KeywordDeclaration, Text).
		rule(jsID, NameClass, pop())

	b.state("statement").
		cb(`case\b`, func(l *lexState, m *onigmo.MatchData) {
			l.emit(Keyword, m.Str(0))
			l.pos = m.End(0)
			l.goTo("expr_start")
		}).
		groupsRule(`(`+jsID+`)(\s*)(:)`, NameLabel, Text, Punctuation).
		mixin("expr_start")

	b.state("object").
		mixin("comments_and_whitespace").
		cb(`[{]`, func(l *lexState, m *onigmo.MatchData) {
			l.emit(Punctuation, m.Str(0))
			l.pos = m.End(0)
			l.push(l.stack[len(l.stack)-1].name)
		}).
		cb(`[}]`, func(l *lexState, m *onigmo.MatchData) {
			l.emit(Punctuation, m.Str(0))
			l.pos = m.End(0)
			l.goTo("statement")
		}).
		groupsRuleT(`(`+jsID+`)(\s*)(:)`, []transition{push("expr_start")}, NameAttribute, Text, Punctuation).
		rule(`:`, Punctuation).
		mixin("root")

	b.state("ternary").
		cb(`:`, func(l *lexState, m *onigmo.MatchData) {
			l.emit(Punctuation, m.Str(0))
			l.pos = m.End(0)
			l.goTo("expr_start")
		}).
		mixin("root")

	b.state("template_string").
		rule(`[$]{`, Punctuation, push("template_string_expr")).
		rule("`", LiteralStringDouble, pop()).
		rule(`\\[$`+"`"+`\\]`, LiteralStringEscape).
		rule("[^$`\\\\]+", LiteralStringDouble).
		rule("[\\\\$]", LiteralStringDouble)

	b.state("template_string_expr").
		rule(`}`, Punctuation, pop()).
		mixin("root")

	return b.done()
}()
