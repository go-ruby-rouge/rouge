package rouge

// Theme maps tokens to inline CSS declarations for the HTMLInline formatter,
// mirroring the resolved output of Rouge::Theme#style_for(tok).rendered_rules.
// Lookup walks the token's ancestor chain and returns the rules of the most
// specific ancestor that has a style, falling back to the base (Text) style,
// exactly like Rouge::Theme.get_style.
type Theme struct {
	// name is the theme's registry name, e.g. "base16".
	name string
	// rules maps a token's qualified name to its already-resolved, ";"-joined
	// CSS declarations (e.g. "color: #383838"), in the same shape
	// rendered_rules produces.
	rules map[string]string
}

// Name returns the theme's name.
func (t *Theme) Name() string { return t.name }

// StyleFor returns the ";"-joined inline CSS declarations for tok, resolving up
// the ancestor chain the way Rouge::Theme.get_style does: the closest ancestor
// (including tok itself) with a defined style wins; if none is defined the base
// Text style is used; if even Text is undefined the result is empty.
func (t *Theme) StyleFor(tok *Token) string {
	for c := tok; c != nil; c = c.Parent {
		if r, ok := t.rules[c.Qualname()]; ok {
			return r
		}
	}
	return t.rules["Text"]
}

// themeRegistry maps a theme name to its theme, populated at init. Mirrors
// Rouge::Theme.registry.
var themeRegistry = map[string]*Theme{}

func registerTheme(t *Theme) { themeRegistry[t.name] = t }

// FindTheme returns the theme registered under name, or nil. Mirrors
// Rouge::Theme.find.
func FindTheme(name string) *Theme { return themeRegistry[name] }

// The three bundled themes. Their rule tables are the resolved
// rendered_rules output of the corresponding gem themes, captured token by
// token, so HTMLInline reproduces the gem's inline styles exactly.

// Base16 is the rouge "base16" theme.
var Base16 = &Theme{name: "base16", rules: map[string]string{
	"Text":                    "color: #383838",
	"Error":                   "color: #181818;background-color: #ab4642",
	"Comment":                 "color: #585858",
	"Comment.Preproc":         "color: #f7ca88",
	"Name.Tag":                "color: #f7ca88",
	"Operator":                "color: #d8d8d8",
	"Punctuation":             "color: #d8d8d8",
	"Generic.Inserted":        "color: #a1b56c",
	"Generic.Deleted":         "color: #ab4642",
	"Generic.Heading":         "color: #7cafc2;background-color: #181818;font-weight: bold",
	"Generic.Emph":            "font-style: italic",
	"Generic.EmphStrong":      "font-weight: bold;font-style: italic",
	"Generic.Strong":          "font-weight: bold",
	"Keyword":                 "color: #ba8baf",
	"Keyword.Constant":        "color: #dc9656",
	"Keyword.Type":            "color: #dc9656",
	"Keyword.Declaration":     "color: #dc9656",
	"Literal.String":          "color: #a1b56c",
	"Literal.String.Affix":    "color: #ba8baf",
	"Literal.String.Regex":    "color: #86c1b9",
	"Literal.String.Interpol": "color: #a16946",
	"Literal.String.Escape":   "color: #a16946",
	"Name.Namespace":          "color: #f7ca88",
	"Name.Class":              "color: #f7ca88",
	"Name.Constant":           "color: #f7ca88",
	"Name.Attribute":          "color: #7cafc2",
	"Literal.Number":          "color: #a1b56c",
	"Literal.String.Symbol":   "color: #a1b56c",
}}

// Github is the rouge "github" theme.
var Github = &Theme{name: "github", rules: map[string]string{
	"Text":                    "color: #24292f;background-color: #f6f8fa",
	"Keyword":                 "color: #cf222e",
	"Generic.Error":           "color: #f6f8fa",
	"Generic.Deleted":         "color: #82071e;background-color: #ffebe9",
	"Name.Builtin":            "color: #953800",
	"Name.Class":              "color: #953800",
	"Name.Constant":           "color: #953800",
	"Name.Namespace":          "color: #953800",
	"Literal.String.Regex":    "color: #116329",
	"Name.Attribute":          "color: #116329",
	"Name.Tag":                "color: #116329",
	"Generic.Inserted":        "color: #116329;background-color: #dafbe1",
	"Generic.EmphStrong":      "font-weight: bold;font-style: italic",
	"Keyword.Constant":        "color: #0550ae",
	"Literal":                 "color: #0550ae",
	"Literal.String.Backtick": "color: #0550ae",
	"Name.Builtin.Pseudo":     "color: #0550ae",
	"Name.Exception":          "color: #0550ae",
	"Name.Label":              "color: #0550ae",
	"Name.Property":           "color: #0550ae",
	"Name.Variable":           "color: #0550ae",
	"Operator":                "color: #0550ae",
	"Generic.Heading":         "color: #0550ae;font-weight: bold",
	"Generic.Subheading":      "color: #0550ae;font-weight: bold",
	"Literal.String":          "color: #0a3069",
	"Name.Decorator":          "color: #8250df",
	"Name.Function":           "color: #8250df",
	"Error":                   "color: #f6f8fa;background-color: #82071e",
	"Comment":                 "color: #6e7781",
	"Generic.Lineno":          "color: #6e7781",
	"Generic.Traceback":       "color: #6e7781",
	"Name.Entity":             "color: #24292f",
	"Literal.String.Interpol": "color: #24292f",
	"Generic.Emph":            "color: #24292f;font-style: italic",
	"Generic.Strong":          "color: #24292f;font-weight: bold",
}}

// ThankfulEyes is the rouge "thankful_eyes" theme.
var ThankfulEyes = &Theme{name: "thankful_eyes", rules: map[string]string{
	"Text":                    "color: #faf6e4;background-color: #122b3b",
	"Generic.Lineno":          "color: #dee5e7;background-color: #4e5d62",
	"Generic.Prompt":          "color: #a8e1fe;font-weight: bold",
	"Comment":                 "color: #6c8b9f;font-style: italic",
	"Comment.Preproc":         "color: #b2fd6d;font-weight: bold",
	"Error":                   "color: #fefeec;background-color: #cc0000",
	"Generic.Error":           "color: #cc0000;font-weight: bold;font-style: italic",
	"Keyword":                 "color: #f6dd62;font-weight: bold",
	"Operator":                "color: #4df4ff;font-weight: bold",
	"Punctuation":             "color: #4df4ff",
	"Generic.Deleted":         "color: #cc0000",
	"Generic.Inserted":        "color: #b2fd6d",
	"Generic.Emph":            "font-style: italic",
	"Generic.EmphStrong":      "font-weight: bold;font-style: italic",
	"Generic.Strong":          "font-weight: bold",
	"Generic.Traceback":       "color: #dee5e7;background-color: #4e5d62",
	"Keyword.Constant":        "color: #f696db;font-weight: bold",
	"Keyword.Namespace":       "color: #ffb000;font-weight: bold",
	"Keyword.Pseudo":          "color: #ffb000;font-weight: bold",
	"Keyword.Reserved":        "color: #ffb000;font-weight: bold",
	"Generic.Heading":         "color: #ffb000;font-weight: bold",
	"Generic.Subheading":      "color: #ffb000;font-weight: bold",
	"Keyword.Type":            "color: #b2fd6d;font-weight: bold",
	"Name.Constant":           "color: #b2fd6d;font-weight: bold",
	"Name.Class":              "color: #b2fd6d;font-weight: bold",
	"Name.Decorator":          "color: #b2fd6d;font-weight: bold",
	"Name.Namespace":          "color: #b2fd6d;font-weight: bold",
	"Name.Builtin.Pseudo":     "color: #b2fd6d;font-weight: bold",
	"Name.Exception":          "color: #b2fd6d;font-weight: bold",
	"Name.Label":              "color: #ffb000;font-weight: bold",
	"Name.Tag":                "color: #ffb000;font-weight: bold",
	"Literal.Number":          "color: #f696db;font-weight: bold",
	"Literal.Date":            "color: #f696db;font-weight: bold",
	"Literal.String.Symbol":   "color: #f696db;font-weight: bold",
	"Literal.String":          "color: #fff0a6;font-weight: bold",
	"Literal.String.Affix":    "color: #f6dd62;font-weight: bold",
	"Literal.String.Escape":   "color: #4df4ff;font-weight: bold",
	"Literal.String.Char":     "color: #4df4ff;font-weight: bold",
	"Literal.String.Interpol": "color: #4df4ff;font-weight: bold",
	"Name.Builtin":            "font-weight: bold",
	"Name.Entity":             "color: #999999;font-weight: bold",
	"Text.Whitespace":         "color: #BBBBBB",
	"Generic.Output":          "color: #BBBBBB",
	"Name.Function":           "color: #a8e1fe",
	"Name.Property":           "color: #a8e1fe",
	"Name.Attribute":          "color: #a8e1fe",
	"Name.Variable":           "color: #a8e1fe;font-weight: bold",
}}
