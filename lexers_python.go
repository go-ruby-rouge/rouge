// Copyright (c) the go-ruby-rouge/rouge authors
//
// SPDX-License-Identifier: BSD-3-Clause

package rouge

import (
	"strings"

	onigmo "github.com/go-ruby-regexp/regexp"
)

// --- Python ---
// A faithful transcription of Rouge::Lexers::Python (rouge 5.0.0). The keyword /
// builtin / exception word sets are the gem's. The f-string / quoted-string
// register (the gem's StringRegister) is held in lexState.strReg as a stack of
// [type, delim] pairs.

var pythonKeywords = stringSet(`assert break continue del elif else except exec
finally for global if lambda pass print raise return try while yield as with
from import async await nonlocal`)

var pythonBuiltins = stringSet(`__import__ abs aiter all anext any apply ascii
basestring bin bool buffer breakpoint bytearray bytes callable chr classmethod
cmp coerce compile complex delattr dict dir divmod enumerate eval exec execfile
exit file filter float format frozenset getattr globals hasattr hash help hex id
input int intern isinstance issubclass iter len list locals long map max
memoryview min next object oct open ord pow print property range raw_input reduce
reload repr reversed round set setattr slice sorted staticmethod str sum super
tuple type unichr unicode vars xrange zip`)

var pythonBuiltinsPseudo = stringSet(`None Ellipsis NotImplemented False True`)

var pythonExceptions = stringSet(`ArithmeticError AssertionError AttributeError
BaseException BaseExceptionGroup BlockingIOError BrokenPipeError BufferError
BytesWarning ChildProcessError ConnectionAbortedError ConnectionError
ConnectionRefusedError ConnectionResetError DeprecationWarning EOFError
EnvironmentError EncodingWarning Exception ExceptionGroup FileExistsError
FileNotFoundError FloatingPointError FutureWarning GeneratorExit IOError
ImportError ImportWarning IndentationError IndexError InterruptedError
IsADirectoryError KeyError KeyboardInterrupt LookupError MemoryError
ModuleNotFoundError NameError NotADirectoryError NotImplemented
NotImplementedError OSError OverflowError OverflowWarning
PendingDeprecationWarning PermissionError ProcessLookupError
PythonFinalizationError RecursionError ReferenceError ResourceWarning
RuntimeError RuntimeWarning StandardError StopAsyncIteration StopIteration
SyntaxError SyntaxWarning SystemError SystemExit TabError TimeoutError TypeError
UnboundLocalError UnicodeDecodeError UnicodeEncodeError UnicodeError
UnicodeTranslateError UnicodeWarning UserWarning ValueError VMSError Warning
WindowsError ZeroDivisionError`)

// strReg helpers: a [type, delim] pair stack on lexState (the gem's
// StringRegister).
func srRegister(l *lexState, typ, delim string) { l.strReg = append(l.strReg, [2]string{typ, delim}) }
func srRemove(l *lexState) {
	if n := len(l.strReg); n > 0 {
		l.strReg = l.strReg[:n-1]
	}
}
func srDelim(l *lexState, delim string) bool {
	n := len(l.strReg)
	return n > 0 && l.strReg[n-1][1] == delim
}
func srType(l *lexState, typ string) bool {
	n := len(l.strReg)
	return n > 0 && strings.Contains(l.strReg[n-1][0], typ)
}

var pythonLexer = func() *RegexLexer {
	const (
		identifier       = `[[:alpha:]_][[:alnum:]_]*`
		dottedIdentifier = `[[:alpha:]_.][[:alnum:]_.]*`
		inlineWS         = `(?:[ \t]|\\\n)*?`
		inlineContent    = `(?:[^\\\n]|\\[\n.])*?`
		operatorWords    = `(?:in|is|and|or|not)\b`
		operators        = `(?:<<|>>|//|[*][*])=?|!=|[-~+/*%=<>&^|@]=?|!=`
		digits           = `[0-9](?:_?[0-9])*`
		decimal          = `(?:(?:` + digits + `)?\.` + digits + `|` + digits + `\.)`
		exponent         = `(?i:e[+-]?` + digits + `)`
	)

	b := newRegexLexer("python", "Python", "py")
	b.filenames("*.py", "*.pyi", "*.pyw")
	b.detectWith(func(text string) bool {
		if strings.HasPrefix(text, "#!") {
			line := text
			if nl := strings.IndexByte(text, '\n'); nl >= 0 {
				line = text[:nl]
			}
			if re, err := onigmo.Compile(`pythonw?(?:[23](?:\.\d+)?)?`); err == nil && re.MatchString(line) {
				return true
			}
		}
		return false
	})
	b.start("newline")

	b.state("inline_whitespace").
		rule(`[ \t]+`, Text).
		rule(`\\\n`, LiteralStringEscape)

	b.state("root").
		rule(`(?m)\n+`, Text, push("newline")).
		groupsRule(`(?mi)^(:)(\s*)([ru]{0,2}""".*?""")`, Punctuation, Text, LiteralStringDoc).
		rule(`\.\.\.\B$`, NameBuiltinPseudo).
		mixin("inline_whitespace").
		rule(`#(?:.*)?\n?`, CommentSingle, push("newline")).
		rule(`[\[\]{}:(),;]`, Punctuation).
		rule(`[.]`, Punctuation, push("post_dot")).
		rule(`\\`, LiteralStringEscape).
		rule(`(?i)@`+dottedIdentifier, NameDecorator).
		rule(operatorWords, OperatorWord).
		rule(operators, Operator).
		rule(`def\b`, Keyword, push("funcname")).
		rule(`class\b`, Keyword, push("classname")).
		rule("`.*?`", LiteralStringBacktick).
		cb(`(?i)([rtfbu]{0,2})('''|"""|['"])`, func(l *lexState, m *onigmo.MatchData) {
			l.emit(LiteralStringAffix, m.Str(1))
			l.emit(LiteralStringHeredoc, m.Str(2))
			srRegister(l, strings.ToLower(m.Str(1)), m.Str(2))
			l.pos = m.End(0)
			l.push("generic_string")
		}).
		cb(`(?<!\.)`+identifier, func(l *lexState, m *onigmo.MatchData) {
			w := m.Str(0)
			switch {
			case pythonKeywords[w]:
				l.emit(Keyword, w)
			case pythonExceptions[w]:
				l.emit(NameException, w)
			case pythonBuiltins[w]:
				l.emit(NameBuiltin, w)
			case pythonBuiltinsPseudo[w]:
				l.emit(NameBuiltinPseudo, w)
			default:
				l.emit(Name, w)
			}
			l.pos = m.End(0)
		}).
		rule(identifier, Name).
		rule(`(?i)`+decimal+`(?:`+exponent+`)?j?`, LiteralNumberFloat).
		rule(`(?i)`+digits+exponent+`j?`, LiteralNumberFloat).
		rule(`(?i)`+digits+`j`, LiteralNumberFloat).
		rule(`(?i)0b(?:_?[0-1])+`, LiteralNumberBin).
		rule(`(?i)0o(?:_?[0-7])+`, LiteralNumberOct).
		rule(`(?i)0x(?:_?[a-f0-9])+`, LiteralNumberHex).
		rule(`\d+L`, LiteralNumberIntegerLong).
		rule(`(?:[1-9](?:_?[0-9])*|0(?:_?0)*)`, LiteralNumberInteger)

	b.state("import").
		mixin("inline_whitespace").
		rule(dottedIdentifier, NameNamespace, pop()).
		cb(``, func(l *lexState, m *onigmo.MatchData) { l.pop(1) })

	b.state("from").
		mixin("inline_whitespace").
		cb(dottedIdentifier, func(l *lexState, m *onigmo.MatchData) {
			l.emit(NameNamespace, m.Str(0))
			l.pos = m.End(0)
			l.goTo("from_import")
		}).
		cb(``, func(l *lexState, m *onigmo.MatchData) { l.pop(1) })

	b.state("from_import").
		mixin("inline_whitespace").
		rule(`import\b`, KeywordNamespace, pop()).
		cb(``, func(l *lexState, m *onigmo.MatchData) { l.pop(1) })

	b.state("post_dot").
		mixin("inline_whitespace").
		rule(`(?m)[A-Z]\w*(?=`+inlineWS+`[(])`, NameClass).
		rule(`(?m)(?:`+identifier+`)(?=`+inlineWS+`[(])`, NameFunction).
		cb(``, func(l *lexState, m *onigmo.MatchData) { l.pop(1) })

	b.state("newline").
		mixin("inline_whitespace").
		rule(`from\b`, KeywordNamespace, push("from")).
		rule(`import\b`, KeywordNamespace, push("import")).
		rule(`(?:case|match)(?=`+inlineWS+`(?:`+operatorWords+`|if\b|`+operators+`))`, NameOther, pop()).
		cb(`(?:case|match)(?=`+inlineContent+`:`+inlineWS+`[#\n])`, func(l *lexState, m *onigmo.MatchData) {
			l.emit(Keyword, m.Str(0))
			l.pos = m.End(0)
			if m.Str(0) == "case" {
				l.goTo("case_pattern")
			} else {
				l.pop(1)
			}
		}).
		cb(``, func(l *lexState, m *onigmo.MatchData) { l.pop(1) })

	b.state("funcname").
		mixin("inline_whitespace").
		rule(identifier, NameFunction, pop())

	b.state("classname").
		mixin("inline_whitespace").
		rule(identifier, NameClass, pop())

	b.state("case_pattern").
		cb(`\n`, func(l *lexState, m *onigmo.MatchData) {
			l.emit(Text, m.Str(0))
			l.pos = m.End(0)
			l.goTo("newline")
		}).
		rule(`_\b`, Keyword).
		mixin("root")

	b.state("raise").
		rule(`from\b`, Keyword).
		rule(`raise\b`, Keyword).
		rule(`yield\b`, Keyword).
		rule(`\n`, Text, pop()).
		rule(`;`, Punctuation, pop()).
		mixin("root")

	b.state("yield").
		mixin("raise")

	b.state("generic_string").
		rule(`\n`, LiteralString, push("generic_string_newline")).
		rule(`[^'"\\{\n]+`, LiteralString).
		rule(`{{`, LiteralString).
		cb(`'''|"""|['"]`, func(l *lexState, m *onigmo.MatchData) {
			l.emit(LiteralStringHeredoc, m.Str(0))
			l.pos = m.End(0)
			if srDelim(l, m.Str(0)) {
				srRemove(l)
				l.pop(1)
			}
		}).
		rule(`(?=\\)`, LiteralString, push("generic_escape")).
		cb(`{`, func(l *lexState, m *onigmo.MatchData) {
			if srType(l, "f") {
				l.emit(LiteralStringInterpol, m.Str(0))
				l.pos = m.End(0)
				l.push("generic_interpol")
			} else {
				l.emit(LiteralString, m.Str(0))
				l.pos = m.End(0)
			}
		})

	b.state("generic_string_newline").
		rule(`[ \t]+`, LiteralString).
		cb(`(?:>>>|\.\.\.)\B`, func(l *lexState, m *onigmo.MatchData) {
			l.emit(GenericPrompt, m.Str(0))
			l.pos = m.End(0)
			l.goTo("doctest")
		}).
		cb(``, func(l *lexState, m *onigmo.MatchData) { l.pop(1) })

	b.state("generic_escape").
		cb(`\\(?:[\\abfnrtv"']|\n|newline|N\{[a-zA-Z][a-zA-Z ]+[a-zA-Z]\}|u[a-fA-F0-9]{4}|U[a-fA-F0-9]{8}|x[a-fA-F0-9]{2}|[0-7]{1,3})`, func(l *lexState, m *onigmo.MatchData) {
			if srType(l, "r") {
				l.emit(LiteralString, m.Str(0))
			} else {
				l.emit(LiteralStringEscape, m.Str(0))
			}
			l.pos = m.End(0)
			l.pop(1)
		}).
		rule(`\\.`, LiteralString, pop())

	b.state("doctest").
		rule(`\n\n`, Text, pop()).
		cb(`'''|"""`, func(l *lexState, m *onigmo.MatchData) {
			l.emit(LiteralStringHeredoc, m.Str(0))
			l.pos = m.End(0)
			if l.inState("generic_string") {
				l.pop(2)
			}
		}).
		mixin("root")

	b.state("generic_interpol").
		cb(`[^{}!:]+`, func(l *lexState, m *onigmo.MatchData) {
			l.pos = m.End(0)
			l.recurse(m.Str(0))
		}).
		rule(`![asr]`, LiteralStringInterpol).
		rule(`:`, LiteralStringInterpol).
		rule(`{`, LiteralStringInterpol, push("generic_interpol")).
		rule(`}`, LiteralStringInterpol, pop())

	return b.done()
}()
