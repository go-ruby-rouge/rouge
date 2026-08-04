package rouge

import "strings"

// Lexer tokenizes source text into a stream of (token, value) pairs. Every
// lexer in this package is a *RegexLexer, but the interface lets callers and
// formatters work against the abstraction, mirroring Rouge::Lexer.
type Lexer interface {
	// Lex returns the token stream for text. The stream never contains empty
	// values.
	Lex(text string) []TokenValue
	// Tag returns the lexer's primary tag.
	Tag() string
	// Title returns the human-readable title.
	Title() string
	// Aliases returns alternate names.
	Aliases() []string
}

// lexerRegistry maps every tag and alias to its lexer. It is populated by
// registerLexer at package init.
var lexerRegistry = map[string]Lexer{}

// lexerList is the registration-ordered list of distinct lexers, used by Guess.
var lexerList []Lexer

// registerLexer records a lexer under its tag and all aliases. A duplicate tag
// is an authoring bug and panics.
func registerLexer(l Lexer) {
	if _, dup := lexerRegistry[l.Tag()]; dup {
		panic("rouge: duplicate lexer tag: " + l.Tag())
	}
	lexerRegistry[l.Tag()] = l
	for _, a := range l.Aliases() {
		lexerRegistry[a] = l
	}
	lexerList = append(lexerList, l)
}

// FindLexer returns the lexer registered under the given tag or alias, or nil
// if none matches. The lookup is case-insensitive on the tag, mirroring
// Rouge::Lexer.find.
func FindLexer(name string) Lexer {
	if l, ok := lexerRegistry[name]; ok {
		return l
	}
	return lexerRegistry[strings.ToLower(name)]
}

// FindFancy resolves a "fancy" lexer spec of the form "tag" or
// "tag?option=value&...", mirroring Rouge::Lexer.find_fancy. When the spec
// carries options and the resolved lexer understands them (its options hook is
// set, e.g. PHP's start_inline), FindFancy returns a configured variant;
// otherwise the options are ignored. An empty or "guess" name returns nil so
// callers can fall back to Guess. An unknown tag returns nil.
func FindFancy(spec string) Lexer {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return nil
	}
	name, optStr := spec, ""
	if i := strings.IndexByte(spec, '?'); i >= 0 {
		name, optStr = spec[:i], spec[i+1:]
	}
	if name == "guess" {
		return nil
	}
	l := FindLexer(name)
	if l == nil || optStr == "" {
		return l
	}
	if rl, ok := l.(*RegexLexer); ok && rl.options != nil {
		return rl.options(rl, parseFancyOpts(optStr))
	}
	return l
}

// parseFancyOpts splits the query part of a fancy spec ("a=1&b=2") into a map.
// A bare key with no '=' maps to the empty string.
func parseFancyOpts(optStr string) map[string]string {
	opts := map[string]string{}
	for _, part := range strings.Split(optStr, "&") {
		k, v, _ := strings.Cut(part, "=")
		opts[k] = v
	}
	return opts
}

// Guess picks a lexer for text using each lexer's content sniffer (Rouge's
// self.detect?). The first lexer (in registration order) that claims the text
// wins; if none does, PlainText is returned, so Guess never returns nil. This is
// a deliberately small subset of Rouge's multi-signal guesser: it uses only the
// content detectors, which is enough for the formats whose detectors are
// unambiguous (e.g. diff). See README "Simplified" notes.
func Guess(text string) Lexer {
	for _, l := range lexerList {
		if rl, ok := l.(*RegexLexer); ok && rl.detect != nil && rl.detect(text) {
			return l
		}
	}
	return FindLexer("text")
}

// Highlight tokenizes text with the named lexer and renders it with the named
// formatter, returning the formatted string. It mirrors Rouge.highlight. An
// unknown lexer or formatter name returns an error.
func Highlight(text, lexerName, formatterName string) (string, error) {
	l := FindLexer(lexerName)
	if l == nil {
		return "", &UnknownError{Kind: "lexer", Name: lexerName}
	}
	f := FindFormatter(formatterName)
	if f == nil {
		return "", &UnknownError{Kind: "formatter", Name: formatterName}
	}
	return f.Format(l.Lex(text)), nil
}

// UnknownError reports an unknown lexer or formatter name from Highlight.
type UnknownError struct {
	// Kind is "lexer" or "formatter".
	Kind string
	// Name is the unrecognized name.
	Name string
}

func (e *UnknownError) Error() string { return "rouge: unknown " + e.Kind + " " + e.Name }
