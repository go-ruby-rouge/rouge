// Copyright (c) the go-ruby-rouge/rouge authors
//
// SPDX-License-Identifier: BSD-3-Clause

package rouge

import (
	"regexp"
	"strings"

	onigmo "github.com/go-ruby-regexp/regexp"
)

// --- PHP ---
// A faithful transcription of Rouge::Lexers::PHP (rouge 5.0.0), including its
// template start/root delegation, the php code state machine, string
// interpolation, heredocs, function/enum/property-hook sub-states, and the name
// classification of the gem's `names` state.
//
// One documented divergence from the gem's defaults: funcnamehighlighting (the
// gem's `@funcnamehighlighting`, which promotes calls to a large built-in
// function table to Name::Builtin) is NOT modelled. Built-in function calls
// render as Name::Function, exactly as the gem does when constructed with
// funcnamehighlighting:false. The PHP golden corpus is captured from the gem in
// that configuration, so this lexer still matches the gem byte-for-byte for the
// configuration it models. Nothing in kramdown's rouge fixtures depends on
// builtin-function highlighting.

// phpID is the gem's `id`: /[\p{L}_][\p{L}\p{N}_]*/.
const phpID = `[\p{L}_][\p{L}\p{N}_]*`

// phpNS is the gem's `ns`: /(?:#{id}\\)+/ — one or more namespace segments each
// ending in a backslash.
const phpNS = `(?:` + phpID + `\\)+`

// phpIDNS is the gem's `id_with_ns`: /\\?(?:#{ns})?#{id}/ — an optionally
// namespace-qualified identifier.
const phpIDNS = `\\?(?:` + phpNS + `)?` + phpID

// phpKeywords is Rouge::Lexers::PHP.keywords: the reserved words the `names`
// state (and the values state, for the ones it handles first) tags as Keyword.
// Matching is case-insensitive on the lower-cased identifier.
var phpKeywords = stringSet(`old_function cfunction __class__ __dir__ __file__
__function__ __halt_compiler __line__ __method__ __namespace__ __trait__ abstract
and array as break case catch clone continue declare default die do echo else
elseif empty enddeclare endfor endforeach endif endswitch endwhile eval exit
extends final finally fn for foreach global goto if implements include
include_once instanceof insteadof isset list match new or parent print private
protected public readonly require require_once return self static switch throw try
unset var while xor yield`)

// Name-classification regexes used by the gem's `names` block on the raw matched
// identifier. They mirror the gem's POSIX character classes ([[:upper:]] etc.).
var (
	rePhpDunder    = regexp.MustCompile(`^__.*?__$`)
	rePhpEConst    = regexp.MustCompile(`^(E|PHP)(_[[:upper:]]+)+$`)
	rePhpConstName = regexp.MustCompile(`(\\|^)[[:upper:]][[:upper:][:digit:]_]+$`)
	rePhpClassName = regexp.MustCompile(`(\\|^)[[:upper:]][[:alnum:]]*?$`)
)

// PHP-tag detectors for Guess (Rouge::Lexers::PHP.detect?). Ruby's ^ is
// line-anchored, so these use (?m).
var (
	rePhpHHTag  = regexp.MustCompile(`(?m)^<\?hh`)
	rePhpPHPTag = regexp.MustCompile(`(?m)^<\?php`)
)

// resetStack pops back to the sole root state, mirroring Rouge's reset_stack
// (used when a `?>` closing tag returns control to the template/HTML lexer). The
// bottom of the stack is always the root state (Lex seeds it there), so this is a
// truncation.
func (ls *lexState) resetStack() { ls.stack = ls.stack[:1] }

// phpClassifyName tags a bare identifier the way the gem's `names` block does
// (minus the funcnamehighlighting builtin promotion, which this port does not
// model). It emits the token, advances the position, and applies the sub-state
// push the classification calls for.
func (ls *lexState) phpClassifyName(m *onigmo.MatchData) {
	raw := m.Str(0)
	name := strings.ToLower(raw)
	ls.pos = m.End(0)
	switch {
	case name == "use":
		ls.emit(KeywordNamespace, raw)
		ls.push("in_use")
	case name == "const":
		ls.emit(Keyword, raw)
		ls.push("in_const")
	case name == "catch":
		ls.emit(Keyword, raw)
		ls.push("in_catch")
	case name == "public" || name == "protected" || name == "private":
		ls.emit(Keyword, raw)
		ls.push("in_visibility")
	case phpKeywords[name]:
		ls.emit(Keyword, raw)
	case rePhpDunder.MatchString(raw):
		ls.emit(NameBuiltin, raw)
	case rePhpEConst.MatchString(raw):
		ls.emit(KeywordConstant, raw)
	case rePhpConstName.MatchString(raw):
		ls.emit(NameConstant, raw)
	case rePhpClassName.MatchString(raw):
		ls.emit(NameClass, raw)
	default:
		ls.emit(Name, raw)
	}
}

// detectPHP is the content sniffer for Guess (Rouge::Lexers::PHP.detect?).
func detectPHP(text string) bool {
	if strings.HasPrefix(text, "#!") {
		line := text
		if nl := strings.IndexByte(text, '\n'); nl >= 0 {
			line = text[:nl]
		}
		if strings.Contains(line, "php") {
			return true
		}
	}
	if rePhpHHTag.MatchString(text) {
		return false
	}
	return rePhpPHPTag.MatchString(text)
}

// phpTruthy interprets a fancy-spec option value the way Rouge's bool_option
// does: everything is truthy except the recognised falsey spellings.
func phpTruthy(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "", "0", "false", "no", "off", "nil":
		return false
	default:
		return true
	}
}

// phpConfigure builds a per-spec PHP lexer variant from find-fancy options,
// honouring start_inline: a truthy value starts lexing in the php state (inline
// code, no <?php required); a falsey value starts in root (require <?php); no
// option keeps the default guess behaviour (the base lexer's start state). The
// shared, immutable state map is aliased into the shallow copy.
func phpConfigure(base *RegexLexer, opts map[string]string) Lexer {
	v, ok := opts["start_inline"]
	if !ok {
		return base
	}
	cp := *base
	if phpTruthy(v) {
		cp.startPush = []string{"php"}
	} else {
		cp.startPush = nil
	}
	return &cp
}

var phpLexer = func() *RegexLexer {
	b := newRegexLexer("php", "PHP", "php3", "php4", "php5")
	b.filenames("*.php", "*.php3", "*.php4", "*.php5", "*.phtml",
		"*.module", "*.inc", "*.profile", "*.install", "*.test")
	b.detectWith(detectPHP)
	// Default (no start_inline option) is the gem's :guess start behaviour.
	b.start("start")
	b.rl.options = phpConfigure

	b.state("escape").
		cb(`\?>`, func(l *lexState, m *onigmo.MatchData) {
			l.emit(CommentPreproc, m.Str(0))
			l.pos = m.End(0)
			l.resetStack()
		})

	b.state("return").
		rule(``, nil, pop())

	b.state("start").
		cb(`(?m)\s*(?=<)`, func(l *lexState, m *onigmo.MatchData) {
			l.delegate(htmlLexer, m.Str(0))
			l.pos = m.End(0)
			l.pop(1)
		}).
		cb(`(?i)[^$]+(?=<\?(php|=))`, func(l *lexState, m *onigmo.MatchData) {
			l.delegate(htmlLexer, m.Str(0))
			l.pos = m.End(0)
			l.pop(1)
		}).
		cb(``, func(l *lexState, m *onigmo.MatchData) { l.goTo("php") })

	b.state("names").
		rule(`(?i)(?:public|protected|private)\(set\)`, Keyword, push("in_visibility")).
		cb(phpIDNS+`(?=\s*\()`, func(l *lexState, m *onigmo.MatchData) {
			if phpKeywords[strings.ToLower(m.Str(0))] {
				l.emit(Keyword, m.Str(0))
			} else {
				l.emit(NameFunction, m.Str(0))
			}
			l.pos = m.End(0)
		}).
		cb(phpIDNS, func(l *lexState, m *onigmo.MatchData) { l.phpClassifyName(m) })

	b.state("operators").
		rule(`[~!%^&*+|:.<>/@-]+`, Operator)

	b.state("string").
		rule(`"`, LiteralStringDouble, pop()).
		rule(`[^\\{$"]+`, LiteralStringDouble).
		rule(`\\u\{[0-9a-fA-F]+\}`, LiteralStringEscape).
		rule(`\\([efrntv"$\\]|[0-7]{1,3}|[xX][0-9a-fA-F]{1,2})`, LiteralStringEscape).
		rule(`\$`+phpID+`(\[\S+\]|->`+phpID+`)?`, NameVariable).
		rule(`\{\$\{`, LiteralStringInterpol, push("string_interp_double")).
		rule(`\{(?=\$)`, LiteralStringInterpol, push("string_interp_single")).
		groupsRule(`(\{)(\S+)(\})`, LiteralStringInterpol, NameVariable, LiteralStringInterpol).
		rule(`[${\\]+`, LiteralStringDouble)

	b.state("string_interp_double").
		rule(`\}\}`, LiteralStringInterpol, pop()).
		mixin("php")

	b.state("string_interp_single").
		rule(`\}`, LiteralStringInterpol, pop()).
		mixin("php")

	b.state("values").
		rule(`(?im)<<<(["']?)(`+phpID+`)\1\n.*?\n\s*\2;?`, LiteralStringHeredoc).
		rule(`(?i)(\d[_\d]*)?\.(\d[_\d]*)?(e[+-]?\d[_\d]*)?`, LiteralNumberFloat).
		rule(`(?i)0o?[0-7][0-7_]*`, LiteralNumberOct).
		rule(`(?i)0b[01][01_]*`, LiteralNumberBin).
		rule(`(?i)0x[a-f0-9][a-f0-9_]*`, LiteralNumberHex).
		rule(`\d[_\d]*`, LiteralNumberInteger).
		rule(`'([^'\\]*(?:\\.[^'\\]*)*)'`, LiteralStringSingle).
		rule("`([^`\\\\]*(?:\\\\.[^`\\\\]*)*)`", LiteralStringBacktick).
		rule(`"`, LiteralStringDouble, push("string")).
		cb(`(?i)(function|fn)\b`, func(l *lexState, m *onigmo.MatchData) {
			l.emit(Keyword, m.Str(0))
			l.pos = m.End(0)
			l.push("in_function_return")
			l.push("in_function_params")
			l.push("in_function_name")
		}).
		rule(`(?i)(true|false|null)\b`, KeywordConstant).
		rule(`(?i)new\b`, Keyword, push("in_new"))

	b.state("variables").
		rule(`\$\{\$+`+phpID+`\}`, NameVariable).
		rule(`\$+`+phpID, NameVariable)

	b.state("whitespace").
		rule(`\s+`, Text).
		rule(`#[^\[].*?$`, CommentSingle).
		rule(`//.*?$`, CommentSingle).
		rule(`(?m)/\*\*(?!/).*?\*/`, CommentDoc).
		rule(`(?m)/\*.*?\*/`, CommentMultiline)

	b.state("root").
		rule(`(?i)<\?(php|=)?`, CommentPreproc, push("php")).
		cb(`(?m).*?(?=<\?)|.*`, func(l *lexState, m *onigmo.MatchData) {
			l.delegate(htmlLexer, m.Str(0))
			l.pos = m.End(0)
		})

	b.state("php").
		mixin("escape").
		mixin("whitespace").
		mixin("variables").
		mixin("values").
		groupsRule(`(?i)(namespace)(\s+)(`+phpIDNS+`)`, KeywordNamespace, Text, NameNamespace).
		rule(`#\[.*\]$`, NameAttribute).
		groupsRule(`(?i)(class|interface|trait|extends|implements)(\s+)(`+phpIDNS+`)`, KeywordDeclaration, Text, NameClass).
		groupsRuleT(`(?i)(enum)(\s+)(`+phpIDNS+`)`, []transition{push("in_enum")}, KeywordDeclaration, Text, NameClass).
		mixin("names").
		rule(`[;,()\{\}\[\]]`, Punctuation).
		mixin("operators").
		rule(`[=?]`, Operator)

	b.state("in_assign").
		rule(`,`, Punctuation, pop()).
		rule(`[\[\]]`, Punctuation).
		rule(`\(`, Punctuation, push("in_assign_function")).
		mixin("escape").
		mixin("whitespace").
		mixin("values").
		mixin("variables").
		mixin("names").
		mixin("operators").
		mixin("return")

	b.state("in_assign_function").
		rule(`\)`, Punctuation, pop()).
		rule(`,`, Punctuation).
		mixin("in_assign")

	b.state("in_catch").
		rule(`\(`, Punctuation).
		rule(`\|`, Operator).
		rule(phpIDNS, NameClass).
		mixin("escape").
		mixin("whitespace").
		mixin("return")

	b.state("in_const").
		groupsRule(`(?i)(\??`+phpID+`)(\s+)(`+phpID+`)`, KeywordType, Text, NameConstant).
		rule(phpID, NameConstant).
		rule(`=`, Operator, push("in_assign")).
		mixin("escape").
		mixin("whitespace").
		mixin("return")

	b.state("in_function_body").
		rule(`\{`, Punctuation, pushSelf()).
		rule(`\}`, Punctuation, pop()).
		mixin("php")

	b.state("in_function_name").
		rule(`&`, Operator).
		rule(phpID, Name).
		rule(`\(`, Punctuation, pop()).
		mixin("escape").
		mixin("whitespace").
		mixin("return")

	b.state("in_function_params").
		rule(`\)`, Punctuation, pop()).
		rule(`,`, Punctuation).
		rule(`[.]{3}`, Punctuation).
		rule(`=`, Operator, push("in_assign")).
		rule(`(?i)\b(?:public|protected|private|readonly)(?:\(set\)|\b)`, Keyword).
		rule(`(?i)\breadonly\b`, Keyword).
		rule(`\??`+phpID, KeywordType, push("in_property")).
		mixin("escape").
		mixin("whitespace").
		mixin("variables").
		mixin("return")

	b.state("in_function_return").
		rule(`:`, Punctuation).
		rule(`(?i)use\b`, Keyword, push("in_function_use")).
		rule(`\??`+phpID, KeywordType, push("in_assign")).
		cb(`\{`, func(l *lexState, m *onigmo.MatchData) {
			l.emit(Punctuation, m.Str(0))
			l.pos = m.End(0)
			l.goTo("in_function_body")
		}).
		mixin("escape").
		mixin("whitespace").
		mixin("return")

	b.state("in_function_use").
		rule(`[,(]`, Punctuation).
		rule(`&`, Operator).
		rule(`\)`, Punctuation, pop()).
		mixin("escape").
		mixin("whitespace").
		mixin("variables").
		mixin("return")

	b.state("in_new").
		cb(`(?i)class\b`, func(l *lexState, m *onigmo.MatchData) {
			l.emit(KeywordDeclaration, m.Str(0))
			l.pos = m.End(0)
			l.goTo("in_new_class")
		}).
		rule(phpIDNS, NameClass, pop()).
		mixin("escape").
		mixin("whitespace").
		mixin("return")

	b.state("in_new_class").
		rule(`\}`, Punctuation, pop()).
		rule(`\{`, Punctuation).
		mixin("php")

	b.state("in_use").
		rule(`[,\}]`, Punctuation).
		rule(`(?i)(function|const)\b`, Keyword).
		groupsRule(`(`+phpNS+`)(\{)`, NameNamespace, Punctuation).
		rule(phpIDNS+`(_`+phpID+`)+`, NameFunction).
		mixin("escape").
		mixin("whitespace").
		mixin("names").
		mixin("return")

	b.state("in_visibility").
		rule(`(?i)\b(?:public|protected|private)(?:\(set\)|\b)`, Keyword).
		rule(`(?i)\b(?:readonly|static)\b`, Keyword).
		rule(`(?i)(?=(abstract|const|function)\b)`, Keyword, pop()).
		cb(`\??`+phpID, func(l *lexState, m *onigmo.MatchData) {
			l.emit(KeywordType, m.Str(0))
			l.pos = m.End(0)
			l.goTo("in_property")
		}).
		mixin("escape").
		mixin("whitespace").
		mixin("return")

	b.state("in_enum").
		rule(`:`, Punctuation, push("in_enum_base_type")).
		rule(`\{`, Punctuation, push("in_enum_body")).
		mixin("escape").
		mixin("whitespace").
		mixin("return")

	b.state("in_enum_base_type").
		rule(phpID, KeywordType, pop()).
		mixin("escape").
		mixin("whitespace").
		mixin("return")

	b.state("in_enum_body").
		rule(`\}`, Punctuation, pop()).
		groupsRule(`(?i)(case)(\s+)(`+phpID+`)`, Keyword, Text, NameConstant).
		mixin("php")

	b.state("in_property").
		rule(`\$+`+phpID, NameVariable).
		cb(`\{`, func(l *lexState, m *onigmo.MatchData) {
			l.emit(Punctuation, m.Str(0))
			l.pos = m.End(0)
			l.goTo("in_property_hooks")
		}).
		rule(`[;,]`, Punctuation, pop()).
		cb(`(?==)`, func(l *lexState, m *onigmo.MatchData) {
			l.pos = m.End(0)
			l.pop(1)
		}).
		rule(`[|&]`, Operator).
		rule(`\??`+phpID, KeywordType).
		mixin("escape").
		mixin("whitespace").
		mixin("return")

	b.state("in_property_hooks").
		rule(`\}`, Punctuation, pop()).
		rule(`(?i)\bfinal\b`, Keyword).
		rule(`&(?=get\b)`, Operator).
		groupsRuleT(`(\bset\b)(\s*)(\()`, []transition{push("in_property_hook_params")}, Keyword, Text, Punctuation).
		rule(`\b(?:get|set)\b`, Keyword).
		rule(`\{`, Punctuation, push("in_function_body")).
		rule(`[;,()\[\]]`, Punctuation).
		mixin("escape").
		mixin("whitespace").
		mixin("variables").
		mixin("values").
		mixin("names").
		mixin("operators").
		rule(`[=?]`, Operator)

	b.state("in_property_hook_params").
		rule(`\)`, Punctuation, pop()).
		rule(`\??`+phpID, KeywordType).
		mixin("escape").
		mixin("whitespace").
		mixin("variables")

	return b.done()
}()
