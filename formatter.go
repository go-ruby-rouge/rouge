package rouge

import "strings"

// Formatter renders a token stream into a string (HTML, inline-styled HTML,
// ...), mirroring Rouge::Formatter.
type Formatter interface {
	// Format renders the whole token stream to a string.
	Format(tokens []TokenValue) string
	// Tag returns the formatter's registry tag (e.g. "html").
	Tag() string
}

// formatterRegistry maps a tag to a formatter, populated at init.
var formatterRegistry = map[string]Formatter{}

func registerFormatter(f Formatter) { formatterRegistry[f.Tag()] = f }

// FindFormatter returns the formatter registered under tag, or nil. Mirrors
// Rouge::Formatter.find.
func FindFormatter(tag string) Formatter { return formatterRegistry[tag] }

// htmlEscapeTable maps the four characters Rouge's HTML formatter escapes. \r is
// dropped entirely, matching TABLE_FOR_ESCAPE_HTML.
func escapeHTML(s string) string {
	if !strings.ContainsAny(s, "&<>\r") {
		return s
	}
	var b strings.Builder
	b.Grow(len(s) + 8)
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '&':
			b.WriteString("&amp;")
		case '<':
			b.WriteString("&lt;")
		case '>':
			b.WriteString("&gt;")
		case '\r':
			// dropped
		default:
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

// HTMLFormatter renders the token stream as <span class="shortcode"> elements,
// mirroring Rouge::Formatters::HTML. The bare Text token and the Escape token
// pass their value through without a wrapping span. Highlight does not add a
// <pre>/<code> wrapper; use WrapHTML for the gem's themed-page wrapper.
type HTMLFormatter struct{}

// Tag returns "html".
func (HTMLFormatter) Tag() string { return "html" }

// Format renders the stream. Each token becomes its escaped value wrapped in a
// span carrying the token's CSS short-code, except Text (passed through) and
// Escape (passed through unescaped, as in Rouge's escape?-aware path).
func (HTMLFormatter) Format(tokens []TokenValue) string {
	var b strings.Builder
	for _, tv := range tokens {
		b.WriteString(spanHTML(tv.Token, tv.Value))
	}
	return b.String()
}

// spanHTML renders one (token, value) the way Rouge::Formatters::HTML#span /
// #safe_span do.
func spanHTML(t *Token, val string) string {
	if t == Escape {
		// Escape tokens are emitted verbatim (their value is already raw HTML
		// in the gem's escape path). None of this port's lexers emit Escape, so
		// this branch exists for parity completeness.
		return val
	}
	safe := escapeHTML(val)
	if t == Text {
		return safe
	}
	return `<span class="` + t.Shortname + `">` + safe + `</span>`
}

// WrapHTML wraps formatted HTML in the gem's themed-page container,
// <pre class="highlight"><code>...</code></pre>, matching the markup
// Rouge::Formatters::HTMLPygments (and the rougify CLI) produce around the inner
// spans. The inner string must already be HTML from HTMLFormatter.
func WrapHTML(inner string) string {
	return `<pre class="highlight"><code>` + inner + "</code></pre>\n"
}

// HTMLInlineFormatter renders each token with an inline style attribute computed
// from a Theme, mirroring Rouge::Formatters::HTMLInline. Text passes through
// without a span.
type HTMLInlineFormatter struct {
	// Theme supplies per-token CSS declarations.
	Theme *Theme
}

// Tag returns "html_inline".
func (HTMLInlineFormatter) Tag() string { return "html_inline" }

// Format renders the stream with inline styles from the formatter's Theme.
func (f HTMLInlineFormatter) Format(tokens []TokenValue) string {
	var b strings.Builder
	for _, tv := range tokens {
		if tv.Token == Text || tv.Token == Escape {
			b.WriteString(escapeHTML(tv.Value))
			continue
		}
		rules := f.Theme.StyleFor(tv.Token)
		b.WriteString(`<span style="` + rules + `">` + escapeHTML(tv.Value) + `</span>`)
	}
	return b.String()
}
