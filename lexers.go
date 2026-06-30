package rouge

import (
	"strings"

	onigmo "github.com/go-ruby-regexp/regexp"
)

// This file ports the core set of Rouge lexers and registers every lexer,
// formatter, and theme at package init. Each lexer is a near-mechanical
// transcription of the gem's `state ... rule ...` definition; deviations from
// byte-for-byte parity are noted in the README and in comments.

func init() {
	registerLexer(plainText)
	registerLexer(jsonLexer)
	registerLexer(diffLexer)
	registerLexer(goLexer)
	registerLexer(sqlLexer)
	registerLexer(cssLexer)
	registerLexer(shellLexer)
	registerLexer(yamlLexer)
	registerLexer(pythonLexer)
	registerLexer(javascriptLexer)
	registerLexer(htmlLexer)
	registerLexer(markdownLexer)
	registerLexer(rubyLexer)

	registerFormatter(HTMLFormatter{})
	// HTMLInline needs a theme; the registered default uses Github, mirroring a
	// common gem default. Construct HTMLInlineFormatter{Theme: ...} directly for
	// another theme.
	registerFormatter(HTMLInlineFormatter{Theme: Github})

	registerTheme(Base16)
	registerTheme(Github)
	registerTheme(ThankfulEyes)
}

// --- PlainText ---

// plainTextLexer is the no-op lexer: it emits the whole input as a single Text
// token, mirroring Rouge::Lexers::PlainText.
type plainTextLexer struct{}

func (plainTextLexer) Tag() string       { return "plaintext" }
func (plainTextLexer) Title() string     { return "Plain Text" }
func (plainTextLexer) Aliases() []string { return []string{"text"} }
func (plainTextLexer) Lex(text string) []TokenValue {
	if text == "" {
		return nil
	}
	return []TokenValue{{Token: Text, Value: text}}
}

var plainText Lexer = plainTextLexer{}

// --- JSON ---
// A faithful transcription of Rouge::Lexers::JSON.

var jsonLexer = func() *RegexLexer {
	b := newRegexLexer("json", "JSON")
	b.filenames("*.json", "Pipfile.lock")

	b.state("whitespace").
		rule(`\s+`, TextWhitespace)

	b.state("root").
		mixin("whitespace").
		rule(`{`, Punctuation, push("object")).
		rule(`\[`, Punctuation, push("array")).
		mixin("name").
		mixin("value").
		rule(`[\]}]`, Punctuation)

	b.state("object").
		mixin("whitespace").
		mixin("name").
		mixin("value").
		rule(`}`, Punctuation, pop()).
		rule(`,`, Punctuation)

	b.state("name").
		groupsRule(`("(?:\\.|[^"\\\n])*?")(\s*)(:)`, NameLabel, TextWhitespace, Punctuation)

	b.state("value").
		mixin("whitespace").
		mixin("constants").
		rule(`"`, LiteralStringDouble, push("string")).
		rule(`\[`, Punctuation, push("array")).
		rule(`{`, Punctuation, push("object"))

	b.state("string").
		rule(`[^\\"]+`, LiteralStringDouble).
		rule(`\\.`, LiteralStringEscape).
		rule(`"`, LiteralStringDouble, pop())

	b.state("array").
		mixin("value").
		rule(`\]`, Punctuation, pop()).
		rule(`,`, Punctuation)

	b.state("constants").
		rule(`(?:true|false|null)`, KeywordConstant).
		rule(`-?(?:0|[1-9]\d*)\.\d+(?:e[+-]?\d+)?`, LiteralNumberFloat).
		rule(`-?(?:0|[1-9]\d*)(?:e[+-]?\d+)?`, LiteralNumberInteger)

	return b.done()
}()

// --- Diff ---
// A faithful transcription of Rouge::Lexers::Diff, including its content
// detector.

var diffLexer = func() *RegexLexer {
	b := newRegexLexer("diff", "diff", "patch", "udiff")
	b.filenames("*.diff", "*.patch")
	b.detectWith(func(text string) bool {
		if strings.HasPrefix(text, "Index: ") {
			return true
		}
		for _, p := range []string{
			`\Adiff[^\n]*?\ba/[^\n]*\bb/`,
			`---.*?\n[+][+][+]`,
			`[+][+][+].*?\n---`,
		} {
			if re, err := onigmo.Compile(p); err == nil && re.MatchString(text) {
				return true
			}
		}
		return false
	})

	b.state("root").
		rule(`^ .*$\n?`, Text).
		rule(`^---$\n?`, Punctuation).
		rule(`(^\++.*$\n?)|(^>+[ \t]+.*$\n?)|(^>+$\n?)`, GenericInserted).
		rule(`(^-+.*$\n?)|(^<+[ \t]+.*$\n?)|(^<+$\n?)`, GenericDeleted).
		rule(`^!.*$\n?`, GenericStrong).
		rule(`^([Ii]ndex|diff).*$\n?`, GenericHeading).
		groupsRule(`(^@@[^@]*@@)([^\n]*\n)`, Punctuation, Text).
		rule(`^\w.*$\n?`, Punctuation).
		rule(`^=.*$\n?`, GenericHeading).
		rule(`.+$\n?`, Text)

	return b.done()
}()

// --- Go ---
// A faithful transcription of Rouge::Lexers::Go. The long named patterns are
// inlined; \b, POSIX classes, and negative lookahead are supported by the
// onigmo backend.

var goLexer = func() *RegexLexer {
	const (
		newline      = `\n`
		unicodeChar  = `[^\n]`
		letter       = `(?:[[:alpha:]]|_)`
		unicodeDigit = `[[:digit:]]`
		hexDigit     = `[0-9A-Fa-f]`
		octalDigit   = `[0-7]`
		binaryDigit  = `[01]`
		decimalDigit = `[0-9]`

		lineComment    = `//(?:(?!\n).)*`
		generalComment = `/\*(?:(?!\*/)(?:.|\n))*\*/`
		comment        = lineComment + `|` + generalComment

		keyword = `\b(?:break|default|func|interface|select|case|defer|go|map|struct|chan|else|goto|package|switch|const|fallthrough|if|range|type|continue|for|import|return|var)\b`

		identifier = `(?!` + keyword + `)` + letter + `(?:` + letter + `|` + unicodeDigit + `)*`

		operator  = `\+=|\+\+|\+|&\^=|&\^|&=|&&|&|==|=|\!=|\!|-=|--|-|\|=|\|\||\||<=|<-|<<=|<<|<|\*=|\*|\^=|\^|>>=|>>|>=|>|\/|\/=|:=|%|%=|\.\.\.|\.|:`
		separator = `\(|\)|\[|\]|\{|\}|,|;`

		decimalLit = decimalDigit + `(?:_?` + decimalDigit + `)*`
		binaryLit  = `0[bB]_*` + binaryDigit + `(?:_?` + binaryDigit + `)*`
		octalLit   = `0[oO]?_*` + octalDigit + `(?:_?` + octalDigit + `)*`
		hexLit     = `0[xX]_*` + hexDigit + `(?:_?` + hexDigit + `)*`
		intLit     = binaryLit + `|` + hexLit + `|` + octalLit + `|` + decimalLit

		decimals = decimalDigit + `(?:_?` + decimalDigit + `)*`
		exponent = `[eE][+\-]?` + decimals
		floatLit = `(?:` + decimals + `\.` + `(?:` + decimals + `)?` + `(?:` + exponent + `)?` +
			`|` + decimals + exponent +
			`|\.` + decimals + `(?:` + exponent + `)?)`

		imaginaryLit = `(?:` + decimals + `|` + floatLit + `)i`

		escapedChar  = `\\[abfnrtv\\'"]`
		littleUValue = `\\u` + hexDigit + `{4}`
		bigUValue    = `\\U` + hexDigit + `{8}`
		unicodeValue = `(?:` + unicodeChar + `|` + littleUValue + `|` + bigUValue + `|` + escapedChar + `)`
		octalByte    = `\\` + octalDigit + `{3}`
		hexByte      = `\\x` + hexDigit + `{2}`
		byteValue    = `(?:` + octalByte + `|` + hexByte + `)`
		charLit      = `'(?:` + unicodeValue + `|` + byteValue + `)'`
		escapeSeq    = `(?:` + escapedChar + `|` + littleUValue + `|` + bigUValue + `|` + hexByte + `)`

		predeclaredTypes     = `\b(?:bool|byte|complex64|complex128|error|float32|float64|int8|int16|int32|int64|int|rune|string|uint8|uint16|uint32|uint64|uintptr|uint)\b`
		predeclaredConstants = `\b(?:true|false|iota|nil)\b`
		predeclaredFunctions = `\b(?:append|cap|close|complex|copy|delete|imag|len|make|new|panic|print|println|real|recover)\b`

		whiteSpace = `\s+`
	)

	b := newRegexLexer("go", "Go", "golang")
	b.filenames("*.go")

	b.state("simple_tokens").
		rule(comment, Comment).
		rule(keyword, Keyword).
		rule(predeclaredTypes, KeywordType).
		rule(predeclaredFunctions, NameBuiltin).
		rule(predeclaredConstants, NameConstant).
		rule(imaginaryLit, LiteralNumber).
		rule(floatLit, LiteralNumber).
		rule(intLit, LiteralNumber).
		rule(charLit, LiteralStringChar).
		rule(operator, Operator).
		rule(separator, Punctuation).
		rule(identifier, Name).
		rule(whiteSpace, Text)

	b.state("root").
		mixin("simple_tokens").
		rule("`", LiteralString, push("raw_string")).
		rule(`"`, LiteralString, push("interpreted_string"))

	b.state("interpreted_string").
		rule(escapeSeq, LiteralStringEscape).
		rule(`\\.`, Error).
		rule(`"`, LiteralString, pop()).
		rule(`[^"\\]+`, LiteralString)

	b.state("raw_string").
		rule("`", LiteralString, pop()).
		rule("[^`]+", LiteralString)

	return b.done()
}()
