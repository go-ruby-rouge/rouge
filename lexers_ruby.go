// Copyright (c) the go-ruby-rouge/rouge authors
//
// SPDX-License-Identifier: BSD-3-Clause

package rouge

import (
	"strings"

	onigmo "github.com/go-ruby-regexp/regexp"
)

// --- Ruby ---
// A faithful transcription of Rouge::Lexers::Ruby (rouge 5.0.0). The keyword /
// pseudo-keyword / builtin word sets are the gem's. The here-document queue lives
// in lexState.heredocQueue (the gem's @heredoc_queue); fallthrough! is expressed
// with cbFall.
//
// Simplified: the gem's %-sigil strings (%(...), %w[...], %r{...}, ...) build an
// anonymous per-match state whose rules depend on the matched delimiter. This
// port scans the sigil body in a single callback (sigilBody) honouring nesting
// for asymmetric delimiters, interpolation, and escapes — behaviourally faithful,
// but the token stream coalesces the body into runs rather than reproducing the
// gem's exact per-rule split inside the body. This is documented in the README.

var rubyKeywords = stringSet(`BEGIN END alias begin break case defined? do else
elsif end ensure for if in next redo rescue raise retry return super then undef
unless until when while yield`)

var rubyKeywordsPseudo = stringSet(`loop include extend raise alias_method attr
catch throw private module_function public protected true false nil __FILE__
__LINE__`)

var rubyBuiltinsG = stringSet(`attr_reader attr_writer attr_accessor __id__
__send__ abort ancestors at_exit autoload binding callcc caller catch chomp chop
class_eval class_variables clone const_defined? const_get const_missing const_set
constants display dup eval exec exit extend fail fork format freeze getc gets
global_variables gsub hash id included_modules inspect instance_eval
instance_method instance_methods instance_variable_get instance_variable_set
instance_variables lambda load local_variables loop method method_missing methods
module_eval name object_id open p print printf private_class_method
private_instance_methods private_methods proc protected_instance_methods
protected_methods public_class_method public_instance_methods public_methods putc
puts raise rand readline readlines require require_relative scan select self send
set_trace_func singleton_methods sleep split sprintf srand sub syscall system
taint test throw to_a to_s trace_var trap untaint untrace_var warn`)

var rubyBuiltinsQ = stringSet(`autoload block_given const_defined eql equal frozen
include instance_of is_a iterator kind_of method_defined nil
private_method_defined protected_method_defined public_method_defined respond_to
tainted`)

var rubyBuiltinsB = stringSet(`chomp chop exit gsub sub`)

// rubyDelimClose maps an opening sigil delimiter to its closing partner.
var rubyDelimClose = map[byte]byte{'{': '}', '[': ']', '(': ')', '<': '>'}

// isWordByte reports whether b is an ASCII word byte ([A-Za-z0-9_]). It stands
// in for the engine-unsupported variable-width (?<!\p{Word}) lookbehind in the
// heredoc rule; non-ASCII word characters before a heredoc opener do not occur
// in practice.
func isWordByte(b byte) bool {
	return b == '_' || (b >= '0' && b <= '9') || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

var rubyLexer = func() *RegexLexer {
	b := newRegexLexer("ruby", "Ruby", "rb")
	b.filenames("*.rb", "*.ruby", "*.rake", "*.gemspec", "Rakefile", "Gemfile")
	b.detectWith(func(text string) bool {
		if strings.HasPrefix(text, "#!") {
			line := text
			if nl := strings.IndexByte(text, '\n'); nl >= 0 {
				line = text[:nl]
			}
			return strings.Contains(line, "ruby")
		}
		return false
	})
	b.start("expr_start")

	b.state("symbols").
		rule(`(?xi):@{0,2}[\p{Ll}_]\p{Word}*[!?]?`, LiteralStringSymbol).
		rule(":(?:\\*\\*|[-+]@|[/%&|^`~]|\\[\\]=?|<<|>>|<=?>|<=?|===?)", LiteralStringSymbol).
		rule(`:'(?:\\\\|\\'|[^'])*'`, LiteralStringSymbol).
		rule(`:"`, LiteralStringSymbol, push("simple_sym"))

	b.state("sigil_strings").
		cbFall(`%([rqswQWxiI])?([^\p{Word}\s])`, sigilString)

	b.state("strings").
		mixin("symbols").
		rule(`\b[\p{Ll}_]\p{Word}*?[?!]?:\s+`, LiteralStringSymbol, push("expr_start")).
		rule(`'(?:\\\\|\\'|[^'])*'`, LiteralStringSingle).
		rule(`"`, LiteralStringDouble, push("simple_string")).
		rule("(?<!\\.)`", LiteralStringBacktick, push("simple_backtick"))

	b.state("regex_flags").
		rule(`[mixounse]*`, LiteralStringRegex, pop())

	// simple_string / simple_sym / simple_backtick.
	for _, sc := range []struct {
		name string
		tok  *Token
		fin  string
	}{
		{"simple_string", LiteralStringDouble, `"`},
		{"simple_sym", LiteralStringSymbol, `"`},
		{"simple_backtick", LiteralStringBacktick, "`"},
	} {
		b.state(sc.name).
			mixin("string_intp_escaped").
			rule(`(?m)[^\\`+sc.fin+`#]+`, sc.tok).
			rule(`[\\#]`, sc.tok).
			rule(sc.fin, sc.tok, pop())
	}

	b.state("whitespace").
		mixin("inline_whitespace").
		rule(`(?m)\n\s*`, Text, push("expr_start")).
		rule(`#.*$`, CommentSingle).
		rule(`(?m)=begin\b.*?\n=end\b`, CommentMultiline)

	b.state("inline_whitespace").
		rule(`[ \t\r]+`, Text)

	b.state("root").
		mixin("whitespace").
		rule(`__END__`, CommentPreproc, push("end_part")).
		rule(`0_?[0-7]+(?:_[0-7]+)*`, LiteralNumberOct).
		rule(`0x[0-9A-Fa-f]+(?:_[0-9A-Fa-f]+)*`, LiteralNumberHex).
		rule(`0b[01]+(?:_[01]+)*`, LiteralNumberBin).
		rule(`[\d]+(?:_\d+)*(?:\.[\d]+(?:_\d+)*(?i:e[+-]?\d+)?|(?i:e[+-]?\d+))`, LiteralNumberFloat).
		rule(`[\d]+(?:_\d+)*`, LiteralNumberInteger).
		rule(`(?i)@@[\p{Ll}_]\p{Word}*`, NameVariableClass).
		rule(`(?i)@[\p{Ll}_]\p{Word}*`, NameVariableInstance).
		rule(`\$\p{Word}+`, NameVariableGlobal).
		rule("\\$[!@&`'+~=/\\\\,;.<>_*$?:\"]", NameVariableGlobal).
		rule(`\$-[0adFiIlpvw]`, NameVariableGlobal).
		rule(`::`, Operator).
		mixin("strings").
		cbFall(`\w+[?]?`, func(l *lexState, m *onigmo.MatchData) bool {
			w := m.Str(0)
			switch {
			case rubyKeywords[w]:
				l.emit(Keyword, w)
			case rubyKeywordsPseudo[w]:
				l.emit(KeywordPseudo, w)
			default:
				return false
			}
			l.pos = m.End(0)
			l.push("expr_start")
			return true
		}).
		rule(`(?:not|and|or)\b`, OperatorWord, push("expr_start")).
		groupsRule(`(?x)(module)(\s+)([\p{L}_][\p{L}0-9_]*(?:::[\p{L}_][\p{L}0-9_]*)*)`, Keyword, Text, NameNamespace).
		groupsRuleT(`(def\b)(\s*)`, []transition{push("funcname")}, Keyword, Text).
		groupsRuleT(`(class\b)(\s*)`, []transition{push("classname")}, Keyword, Text).
		cbFall(`(\w+)([?!])?`, func(l *lexState, m *onigmo.MatchData) bool {
			switch {
			case m.Str(2) == "?" && rubyBuiltinsQ[m.Str(1)]:
				l.emit(NameBuiltin, m.Str(0))
			case m.Str(2) == "!" && rubyBuiltinsB[m.Str(1)]:
				l.emit(NameBuiltin, m.Str(0))
			default:
				return false
			}
			l.pos = m.End(0)
			l.push("expr_start")
			return true
		}).
		cbFall(`(?<![.])\w+`, func(l *lexState, m *onigmo.MatchData) bool {
			if rubyBuiltinsG[m.Str(0)] {
				l.emit(NameBuiltin, m.Str(0))
				l.pos = m.End(0)
				l.push("method_call")
				return true
			}
			return false
		}).
		mixin("has_heredocs").
		rule(`\.{2,3}`, Operator, push("expr_start")).
		rule(`[\p{Lu}][\p{L}0-9_]*`, NameConstant, push("method_call")).
		groupsRuleT(`(\.|::)(\s*)([\p{Ll}_]\p{Word}*[!?]?|[*%&^`+"`"+`~+\-/\[<>=])`,
			[]transition{push("method_call")}, Punctuation, Text, NameFunction).
		rule(`[\p{L}_]\p{Word}*[?!]`, Name, push("expr_start")).
		rule(`[\p{L}_]\p{Word}*`, Name, push("method_call")).
		rule(`\*\*|<<?|>>?|>=|<=|<=>|=~|={3}|!~|&&?|\|\||\.`, Operator, push("expr_start")).
		rule(`[-+/*%=<>&!^|~]=?`, Operator, push("expr_start")).
		cb(`[?]`, func(l *lexState, m *onigmo.MatchData) {
			l.emit(Punctuation, m.Str(0))
			l.pos = m.End(0)
			l.push("ternary")
			l.push("expr_start")
		}).
		rule(`[\[({,:\\;/]`, Punctuation, push("expr_start")).
		rule(`[\])}]`, Punctuation)

	b.state("has_heredocs").
		// The gem guards this rule with (?<!\p{Word}); the engine has no
		// variable-width lookbehind, so the word-boundary check is done in the
		// callback against the preceding byte (ASCII word chars), which is faithful
		// for real heredoc openers. Documented in the README.
		cbFall(`(<<[-~]?)(["`+"`"+`']?)([\p{L}_]\p{Word}*)(\2)`, func(l *lexState, m *onigmo.MatchData) bool {
			if l.pos > 0 && isWordByte(l.src[l.pos-1]) {
				return false
			}
			l.emit(Operator, m.Str(1))
			l.emit(NameConstant, m.Str(2)+m.Str(3)+m.Str(4))
			l.heredocQueue = append(l.heredocQueue, heredocEntry{
				tolerant: m.Str(1) == "<<-" || m.Str(1) == "<<~",
				name:     m.Str(3),
			})
			l.pos = m.End(0)
			if !l.inState("heredoc_queue") {
				l.push("heredoc_queue")
			}
			return true
		}).
		cb(`(<<[-~]?)(["'])(\2)`, func(l *lexState, m *onigmo.MatchData) {
			l.emit(Operator, m.Str(1))
			l.emit(NameConstant, m.Str(2)+m.Str(3))
			l.heredocQueue = append(l.heredocQueue, heredocEntry{
				tolerant: m.Str(1) == "<<-" || m.Str(1) == "<<~",
				name:     "",
			})
			l.pos = m.End(0)
			if !l.inState("heredoc_queue") {
				l.push("heredoc_queue")
			}
		})

	b.state("heredoc_queue").
		cb(`(?=\n)`, func(l *lexState, m *onigmo.MatchData) { l.goTo("resolve_heredocs") }).
		mixin("root")

	b.state("resolve_heredocs").
		mixin("string_intp_escaped").
		rule(`\n`, LiteralStringHeredoc, push("test_heredoc")).
		rule(`[#\\\n]`, LiteralStringHeredoc).
		rule(`[^#\\\n]+`, LiteralStringHeredoc)

	b.state("test_heredoc").
		cb(`[^#\\\n]*$`, func(l *lexState, m *onigmo.MatchData) {
			var first heredocEntry
			if len(l.heredocQueue) > 0 {
				first = l.heredocQueue[0]
			}
			check := m.Str(0)
			if first.tolerant {
				check = strings.TrimSpace(check)
			} else {
				check = strings.TrimRight(check, " \t\r\n")
			}
			if check == first.name {
				if len(l.heredocQueue) > 0 {
					l.heredocQueue = l.heredocQueue[1:]
				}
				if len(l.heredocQueue) == 0 {
					l.pop(1)
				}
				l.emit(NameConstant, m.Str(0))
			} else {
				l.emit(LiteralStringHeredoc, m.Str(0))
			}
			l.pos = m.End(0)
			l.pop(1)
		}).
		cb(``, func(l *lexState, m *onigmo.MatchData) { l.pop(1) })

	b.state("funcname").
		rule(`\s+`, Text).
		rule(`\(`, Punctuation, push("defexpr")).
		groupsRuleT(`(?x)(?:([\p{L}_]\p{Word}*)(\.))?([\p{L}_]\p{Word}*[!?]?|\*\*?|[-+]@?|[/%&|^`+"`"+`~]|\[\]=?|<=>?|<<?|>>?|>=|===?)`,
			[]transition{pop()}, NameClass, Operator, NameFunction).
		cb(``, func(l *lexState, m *onigmo.MatchData) { l.pop(1) })

	b.state("classname").
		rule(`\s+`, Text).
		rule(`\p{Word}+(?:::\p{Word}+)+`, NameClass).
		cb(`\(`, func(l *lexState, m *onigmo.MatchData) {
			l.emit(Punctuation, m.Str(0))
			l.pos = m.End(0)
			l.push("defexpr")
			l.push("expr_start")
		}).
		cb(`<<`, func(l *lexState, m *onigmo.MatchData) {
			l.emit(Operator, m.Str(0))
			l.pos = m.End(0)
			l.goTo("expr_start")
		}).
		rule(`[\p{Lu}_]\p{Word}*`, NameClass, pop()).
		cb(``, func(l *lexState, m *onigmo.MatchData) { l.pop(1) })

	b.state("ternary").
		groupsRuleT(`(:)(\s+)`, []transition{gotoState("expr_start")}, Punctuation, Text).
		cb(`:(?![^#\n]*?[:\\])`, func(l *lexState, m *onigmo.MatchData) {
			l.emit(Punctuation, m.Str(0))
			l.pos = m.End(0)
			l.goTo("expr_start")
		}).
		mixin("root")

	b.state("defexpr").
		groupsRuleT(`(\))(\.|::)?`, []transition{pop()}, Punctuation, Operator).
		cb(`\(`, func(l *lexState, m *onigmo.MatchData) {
			l.emit(Punctuation, m.Str(0))
			l.pos = m.End(0)
			l.push("defexpr")
			l.push("expr_start")
		}).
		mixin("root")

	b.state("in_interp").
		rule(`}`, LiteralStringInterpol, pop()).
		mixin("root")

	b.state("string_intp").
		rule(`[#][{]`, LiteralStringInterpol, push("in_interp")).
		rule(`(?i)#(?:@@?|\$)[\p{Ll}_]\p{Word}*`, LiteralStringInterpol)

	b.state("string_intp_escaped").
		mixin("string_intp").
		rule(`\\(?:[\\abefnrstv#"']|x[a-fA-F0-9]{1,2}|[0-7]{1,3})`, LiteralStringEscape).
		rule(`\\.`, LiteralStringEscape)

	b.state("method_call").
		cb(`/|%`, func(l *lexState, m *onigmo.MatchData) {
			l.emit(Operator, m.Str(0))
			l.pos = m.End(0)
			l.goTo("expr_start")
		}).
		cb(`(?=\n)`, func(l *lexState, m *onigmo.MatchData) { l.pop(1) }).
		cb(``, func(l *lexState, m *onigmo.MatchData) { l.goTo("method_call_spaced") })

	b.state("method_call_spaced").
		mixin("whitespace").
		cb(`[%/]=`, func(l *lexState, m *onigmo.MatchData) {
			l.emit(Operator, m.Str(0))
			l.pos = m.End(0)
			l.goTo("expr_start")
		}).
		cb(`(/)(?=\S|\s*/)`, func(l *lexState, m *onigmo.MatchData) {
			l.emit(LiteralStringRegex, m.Str(0))
			l.pos = m.End(0)
			l.goTo("slash_regex")
		}).
		mixin("sigil_strings").
		cb(`(?=\s*/)`, func(l *lexState, m *onigmo.MatchData) { l.pop(1) }).
		cb(`\s+`, func(l *lexState, m *onigmo.MatchData) {
			l.emit(Text, m.Str(0))
			l.pos = m.End(0)
			l.goTo("expr_start")
		}).
		cb(``, func(l *lexState, m *onigmo.MatchData) { l.pop(1) })

	b.state("expr_start").
		mixin("inline_whitespace").
		cb(`/`, func(l *lexState, m *onigmo.MatchData) {
			l.emit(LiteralStringRegex, m.Str(0))
			l.pos = m.End(0)
			l.goTo("slash_regex")
		}).
		rule(`(?x)[?](?:\\[MC]-)*(?:\\(?:[\\abefnrstv\#"']|x[a-fA-F0-9]{1,2}|[0-7]{1,3})|\S)(?!\p{Word})`, LiteralStringChar, pop()).
		groupsRuleT(`(\s*)(%[rqswQWxiI]? \S* )`, []transition{pop()}, Text, LiteralStringOther).
		mixin("sigil_strings").
		cb(``, func(l *lexState, m *onigmo.MatchData) { l.pop(1) })

	b.state("slash_regex").
		mixin("string_intp").
		rule(`\\\\`, LiteralStringRegex).
		rule(`\\/`, LiteralStringRegex).
		rule(`[\\#]`, LiteralStringRegex).
		rule(`(?m)[^\\/#]+`, LiteralStringRegex).
		cb(`/`, func(l *lexState, m *onigmo.MatchData) {
			l.emit(LiteralStringRegex, m.Str(0))
			l.pos = m.End(0)
			l.goTo("regex_flags")
		})

	b.state("end_part").
		rule(`(?m).+`, CommentPreproc, pop())

	return b.done()
}()

// sigilString lexes a %-sigil string/array/regex literal. m.Str(1) is the
// optional type letter (r, q, w, ...); m.Str(2) is the opening delimiter. It
// scans the body honouring nesting for asymmetric delimiters, interpolation
// (when the type permits), and escapes, then advances past the closing
// delimiter. It always handles the match (returns true).
func sigilString(l *lexState, m *onigmo.MatchData) bool {
	typ := m.Str(1)
	open := m.Str(2)[0]
	close := open
	if c, ok := rubyDelimClose[open]; ok {
		close = c
	}
	tok := LiteralStringOther
	interp := typ == "" || strings.ContainsAny(typ, "rQWxI")
	if typ == "r" {
		tok = LiteralStringRegex
	}
	// Emit the leading %TYPEDELIM marker as the string token.
	l.emit(tok, m.Str(0))
	l.pos = m.End(0)

	depth := 1
	for l.pos < len(l.src) {
		c := l.src[l.pos]
		switch {
		case c == '\\' && l.pos+1 < len(l.src):
			l.emit(LiteralStringEscape, l.src[l.pos:l.pos+2])
			l.pos += 2
			continue
		case interp && c == '#' && l.pos+1 < len(l.src) && l.src[l.pos+1] == '{':
			// Interpolation: delegate the #{...} balanced span to the root lexer.
			end := matchBalancedBrace(l.src, l.pos+1)
			l.emit(LiteralStringInterpol, l.src[l.pos:l.pos+2])
			if end > l.pos+2 {
				l.recurse(l.src[l.pos+2 : end-1])
			}
			l.emit(LiteralStringInterpol, "}")
			l.pos = end
			continue
		case open != close && c == open:
			depth++
			l.emit(tok, string(c))
			l.pos++
			continue
		case c == close:
			depth--
			l.emit(tok, string(c))
			l.pos++
			if depth == 0 {
				if typ == "r" {
					l.push("regex_flags")
				}
				return true
			}
			continue
		default:
			l.emit(tok, string(c))
			l.pos++
		}
	}
	if typ == "r" {
		l.push("regex_flags")
	}
	return true
}

// matchBalancedBrace returns the index just past the '}' that closes the '{' at
// position open in s, accounting for nested braces. If unbalanced it returns
// len(s).
func matchBalancedBrace(s string, open int) int {
	depth := 0
	for i := open; i < len(s); i++ {
		switch s[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i + 1
			}
		}
	}
	return len(s)
}
