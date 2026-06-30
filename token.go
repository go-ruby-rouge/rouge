// Package rouge is a pure-Go (CGO=0), MRI-faithful port of the Ruby `rouge`
// syntax-highlighting gem (https://github.com/rouge-ruby/rouge). It tokenizes
// source code with state-machine regex lexers and formats the token stream as
// HTML, emitting exactly the CSS short-codes (k, s, c, nf, ...) the gem emits.
//
// The token hierarchy, the regex-lexer engine, and the HTML formatters mirror
// the gem's data structures so the rendered HTML matches byte-for-byte on the
// supported lexers.
package rouge

// Token is a syntactic token type, e.g. Keyword or Literal.String. Tokens form
// a tree mirroring Rouge::Token::Tokens: every token has a Name, a one- or
// two-letter CSS Shortname the HTML formatter emits, and a Parent. Tokens are
// compared by identity (pointer), so the package-level token variables below are
// the canonical instances.
type Token struct {
	// Name is the simple token name, e.g. "Function".
	Name string
	// Shortname is the CSS class the HTML formatter emits, e.g. "nf". The root
	// Text token has the empty short-code.
	Shortname string
	// Parent is the enclosing token, nil for the root Text token.
	Parent *Token
	// qualname caches the dotted fully-qualified name, e.g. "Name.Function".
	qualname string
}

// Qualname is the dotted fully-qualified name of the token, e.g.
// "Literal.String.Double", matching Rouge::Token#qualname.
func (t *Token) Qualname() string { return t.qualname }

// String returns the token's qualified name.
func (t *Token) String() string { return t.qualname }

// Matches reports whether t is other or a descendant of other, mirroring
// Rouge::Token.matches? (other appears in t's ancestor chain).
func (t *Token) Matches(other *Token) bool {
	for c := t; c != nil; c = c.Parent {
		if c == other {
			return true
		}
	}
	return false
}

// tokenRegistry maps a qualified name to its canonical *Token, mirroring
// Rouge::Token.cache. It backs TokenByName.
var tokenRegistry = map[string]*Token{}

// tok constructs a token under parent and registers it by qualified name. A nil
// parent makes a top-level token whose qualname is just its name (Text, Keyword,
// ...); a child qualifies as parent.qualname + "." + name (Text.Whitespace,
// Literal.String.Double).
func tok(parent *Token, name, short string) *Token {
	t := &Token{Name: name, Shortname: short, Parent: parent}
	if parent == nil {
		t.qualname = name
	} else {
		t.qualname = parent.qualname + "." + name
	}
	tokenRegistry[t.qualname] = t
	return t
}

// TokenByName returns the canonical token for a dotted qualified name (e.g.
// "Keyword.Constant"), or nil if no such token exists. It mirrors
// Rouge::Token::Tokens[qualname].
func TokenByName(qualname string) *Token {
	return tokenRegistry[qualname]
}

// The token hierarchy. These mirror Rouge::Token::Tokens one-for-one, including
// the CSS short-codes, which are kept in sync with pygments STANDARD_TYPES.
var (
	// Text is the root token; it has the empty short-code.
	Text           = tok(nil, "Text", "")
	TextWhitespace = tok(Text, "Whitespace", "w")

	Escape = tok(nil, "Escape", "esc")
	Error  = tok(nil, "Error", "err")
	Other  = tok(nil, "Other", "x")

	Keyword            = tok(nil, "Keyword", "k")
	KeywordConstant    = tok(Keyword, "Constant", "kc")
	KeywordDeclaration = tok(Keyword, "Declaration", "kd")
	KeywordNamespace   = tok(Keyword, "Namespace", "kn")
	KeywordPseudo      = tok(Keyword, "Pseudo", "kp")
	KeywordReserved    = tok(Keyword, "Reserved", "kr")
	KeywordType        = tok(Keyword, "Type", "kt")
	KeywordVariable    = tok(Keyword, "Variable", "kv")

	Name                 = tok(nil, "Name", "n")
	NameAttribute        = tok(Name, "Attribute", "na")
	NameBuiltin          = tok(Name, "Builtin", "nb")
	NameBuiltinPseudo    = tok(NameBuiltin, "Pseudo", "bp")
	NameClass            = tok(Name, "Class", "nc")
	NameConstant         = tok(Name, "Constant", "no")
	NameDecorator        = tok(Name, "Decorator", "nd")
	NameEntity           = tok(Name, "Entity", "ni")
	NameException        = tok(Name, "Exception", "ne")
	NameFunction         = tok(Name, "Function", "nf")
	NameFunctionMagic    = tok(NameFunction, "Magic", "fm")
	NameProperty         = tok(Name, "Property", "py")
	NameLabel            = tok(Name, "Label", "nl")
	NameNamespace        = tok(Name, "Namespace", "nn")
	NameOther            = tok(Name, "Other", "nx")
	NameTag              = tok(Name, "Tag", "nt")
	NameVariable         = tok(Name, "Variable", "nv")
	NameVariableClass    = tok(NameVariable, "Class", "vc")
	NameVariableGlobal   = tok(NameVariable, "Global", "vg")
	NameVariableInstance = tok(NameVariable, "Instance", "vi")
	NameVariableMagic    = tok(NameVariable, "Magic", "vm")

	Literal     = tok(nil, "Literal", "l")
	LiteralDate = tok(Literal, "Date", "ld")

	LiteralString          = tok(Literal, "String", "s")
	LiteralStringAffix     = tok(LiteralString, "Affix", "sa")
	LiteralStringBacktick  = tok(LiteralString, "Backtick", "sb")
	LiteralStringChar      = tok(LiteralString, "Char", "sc")
	LiteralStringDelimiter = tok(LiteralString, "Delimiter", "dl")
	LiteralStringDoc       = tok(LiteralString, "Doc", "sd")
	LiteralStringDouble    = tok(LiteralString, "Double", "s2")
	LiteralStringEscape    = tok(LiteralString, "Escape", "se")
	LiteralStringHeredoc   = tok(LiteralString, "Heredoc", "sh")
	LiteralStringInterpol  = tok(LiteralString, "Interpol", "si")
	LiteralStringOther     = tok(LiteralString, "Other", "sx")
	LiteralStringRegex     = tok(LiteralString, "Regex", "sr")
	LiteralStringSingle    = tok(LiteralString, "Single", "s1")
	LiteralStringSymbol    = tok(LiteralString, "Symbol", "ss")

	LiteralNumber            = tok(Literal, "Number", "m")
	LiteralNumberBin         = tok(LiteralNumber, "Bin", "mb")
	LiteralNumberFloat       = tok(LiteralNumber, "Float", "mf")
	LiteralNumberHex         = tok(LiteralNumber, "Hex", "mh")
	LiteralNumberInteger     = tok(LiteralNumber, "Integer", "mi")
	LiteralNumberIntegerLong = tok(LiteralNumberInteger, "Long", "il")
	LiteralNumberOct         = tok(LiteralNumber, "Oct", "mo")
	LiteralNumberOther       = tok(LiteralNumber, "Other", "mx")

	Operator     = tok(nil, "Operator", "o")
	OperatorWord = tok(Operator, "Word", "ow")

	Punctuation          = tok(nil, "Punctuation", "p")
	PunctuationIndicator = tok(Punctuation, "Indicator", "pi")

	Comment            = tok(nil, "Comment", "c")
	CommentHashbang    = tok(Comment, "Hashbang", "ch")
	CommentDoc         = tok(Comment, "Doc", "cd")
	CommentMultiline   = tok(Comment, "Multiline", "cm")
	CommentPreproc     = tok(Comment, "Preproc", "cp")
	CommentPreprocFile = tok(Comment, "PreprocFile", "cpf")
	CommentSingle      = tok(Comment, "Single", "c1")
	CommentSpecial     = tok(Comment, "Special", "cs")

	Generic           = tok(nil, "Generic", "g")
	GenericDeleted    = tok(Generic, "Deleted", "gd")
	GenericEmph       = tok(Generic, "Emph", "ge")
	GenericEmphStrong = tok(Generic, "EmphStrong", "ges")
	GenericError      = tok(Generic, "Error", "gr")
	GenericHeading    = tok(Generic, "Heading", "gh")
	GenericInserted   = tok(Generic, "Inserted", "gi")
	GenericLineno     = tok(Generic, "Lineno", "gl")
	GenericOutput     = tok(Generic, "Output", "go")
	GenericPrompt     = tok(Generic, "Prompt", "gp")
	GenericStrong     = tok(Generic, "Strong", "gs")
	GenericSubheading = tok(Generic, "Subheading", "gu")
	GenericTraceback  = tok(Generic, "Traceback", "gt")
)
