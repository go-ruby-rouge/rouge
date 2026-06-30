// Copyright (c) the go-ruby-rouge/rouge authors
//
// SPDX-License-Identifier: BSD-3-Clause

package rouge

import (
	"strings"
	"testing"
)

// --- token model ---

func TestTokenQualnameAndString(t *testing.T) {
	if LiteralStringDouble.Qualname() != "Literal.String.Double" {
		t.Errorf("qualname = %q", LiteralStringDouble.Qualname())
	}
	if LiteralStringDouble.String() != "Literal.String.Double" {
		t.Errorf("String = %q", LiteralStringDouble.String())
	}
	if Text.Qualname() != "Text" {
		t.Errorf("root qualname = %q", Text.Qualname())
	}
}

func TestTokenShortcodes(t *testing.T) {
	for _, tc := range []struct {
		tok   *Token
		short string
	}{
		{Keyword, "k"}, {LiteralString, "s"}, {Comment, "c"}, {NameFunction, "nf"},
		{Text, ""}, {Operator, "o"}, {Punctuation, "p"}, {LiteralNumberInteger, "mi"},
		{NameVariableInstance, "vi"}, {GenericHeading, "gh"},
	} {
		if tc.tok.Shortname != tc.short {
			t.Errorf("%s shortcode = %q, want %q", tc.tok.Qualname(), tc.tok.Shortname, tc.short)
		}
	}
}

func TestTokenMatches(t *testing.T) {
	if !LiteralStringDouble.Matches(LiteralString) {
		t.Error("Double should match its String ancestor")
	}
	if !LiteralStringDouble.Matches(Literal) {
		t.Error("Double should match its Literal ancestor")
	}
	if !LiteralStringDouble.Matches(LiteralStringDouble) {
		t.Error("a token matches itself")
	}
	if LiteralString.Matches(LiteralStringDouble) {
		t.Error("ancestor should not match descendant")
	}
	if Keyword.Matches(Name) {
		t.Error("unrelated tokens should not match")
	}
}

func TestTokenByName(t *testing.T) {
	if TokenByName("Keyword.Constant") != KeywordConstant {
		t.Error("TokenByName(Keyword.Constant)")
	}
	if TokenByName("Text") != Text {
		t.Error("TokenByName(Text)")
	}
	if TokenByName("No.Such.Token") != nil {
		t.Error("unknown name should be nil")
	}
}

// --- lexer registry / find / fancy / guess ---

func TestFindLexer(t *testing.T) {
	if FindLexer("ruby") == nil || FindLexer("rb") == nil {
		t.Error("ruby/rb should resolve")
	}
	if FindLexer("RUBY") == nil { // case-insensitive fallback
		t.Error("RUBY should resolve case-insensitively")
	}
	if FindLexer("nope-lang") != nil {
		t.Error("unknown lexer should be nil")
	}
	if FindLexer("text").Tag() != "plaintext" {
		t.Errorf("text -> %s", FindLexer("text").Tag())
	}
}

func TestLexerMetadata(t *testing.T) {
	rb := FindLexer("ruby")
	if rb.Title() != "Ruby" {
		t.Errorf("title = %q", rb.Title())
	}
	if rb.Tag() != "ruby" {
		t.Errorf("tag = %q", rb.Tag())
	}
	found := false
	for _, a := range rb.Aliases() {
		if a == "rb" {
			found = true
		}
	}
	if !found {
		t.Error("ruby should alias rb")
	}
	// PlainText metadata.
	pt := FindLexer("plaintext")
	if pt.Title() != "Plain Text" || pt.Tag() != "plaintext" {
		t.Errorf("plaintext meta = %q/%q", pt.Title(), pt.Tag())
	}
	if pt.Aliases()[0] != "text" {
		t.Errorf("plaintext alias = %v", pt.Aliases())
	}
}

func TestFindFancy(t *testing.T) {
	if FindFancy("") != nil {
		t.Error("empty spec -> nil")
	}
	if FindFancy("guess") != nil {
		t.Error("guess spec -> nil")
	}
	if FindFancy("ruby?foo=bar") == nil {
		t.Error("ruby?opts should resolve to ruby")
	}
	if FindFancy("  ruby  ") == nil {
		t.Error("spec should be trimmed")
	}
	if FindFancy("unknown-x") != nil {
		t.Error("unknown fancy -> nil")
	}
}

func TestGuess(t *testing.T) {
	// Diff has an unambiguous content detector.
	if Guess("--- a/x\n+++ b/x\n@@ -1 +1 @@\n").Tag() != "diff" {
		t.Error("guess diff")
	}
	// YAML directive.
	if Guess("%YAML 1.2\n").Tag() != "yaml" {
		t.Error("guess yaml")
	}
	// Shell shebang.
	if Guess("#!/bin/bash\necho hi\n").Tag() != "shell" {
		t.Error("guess shell")
	}
	// Ruby, Python, JavaScript, and HTML content detectors.
	if Guess("#!/usr/bin/env ruby\nputs 1\n").Tag() != "ruby" {
		t.Error("guess ruby")
	}
	if Guess("#!/usr/bin/python3\nprint(1)\n").Tag() != "python" {
		t.Error("guess python")
	}
	if Guess("#!/usr/bin/env node\nlet x=1\n").Tag() != "javascript" {
		t.Error("guess javascript")
	}
	if Guess("<!DOCTYPE html>\n<html></html>\n").Tag() != "html" {
		t.Error("guess html")
	}
	// Nothing claims plain prose -> PlainText fallback.
	if Guess("just some prose with no markers").Tag() != "plaintext" {
		t.Error("guess fallback to plaintext")
	}
}

// TestGuessDetectBranches exercises the remaining content-detector arms: the
// diff "Index:" prefix; the HTML <html> tag and the <?xml early-return (which
// must NOT be claimed as HTML); and the shell #compdef/#autoload starts.
func TestGuessDetectBranches(t *testing.T) {
	if Guess("Index: foo.c\n--- a\n+++ b\n").Tag() != "diff" {
		t.Error("guess diff via Index:")
	}
	if Guess("<html><body></body></html>").Tag() != "html" {
		t.Error("guess html via <html>")
	}
	// A bare XML declaration is explicitly not claimed by HTML's detector; with no
	// other claimant it falls back to plaintext.
	if Guess("<?xml version=\"1.0\"?>\n<root/>").Tag() != "plaintext" {
		t.Error("xml decl should not be guessed as html")
	}
	if Guess("#compdef mytool\n").Tag() != "shell" {
		t.Error("guess shell via #compdef")
	}
	if Guess("#autoload\nfoo\n").Tag() != "shell" {
		t.Error("guess shell via #autoload")
	}
	// A shebang that matches no detector (e.g. perl) is not guessed as shell.
	if Guess("#!/usr/bin/perl\nprint 1;\n").Tag() == "shell" {
		t.Error("perl shebang should not be shell")
	}
}

func TestPlainTextLex(t *testing.T) {
	if got := plainText.Lex(""); got != nil {
		t.Errorf("empty -> %v, want nil", got)
	}
	got := plainText.Lex("abc")
	if len(got) != 1 || got[0].Token != Text || got[0].Value != "abc" {
		t.Errorf("plaintext lex = %v", got)
	}
}

// --- Highlight + errors ---

func TestHighlight(t *testing.T) {
	out, err := Highlight(`puts "hi"`, "ruby", "html")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `<span class="nb">puts</span>`) {
		t.Errorf("highlight output = %q", out)
	}
}

func TestHighlightUnknownLexer(t *testing.T) {
	_, err := Highlight("x", "nope", "html")
	if err == nil {
		t.Fatal("expected error")
	}
	ue, ok := err.(*UnknownError)
	if !ok || ue.Kind != "lexer" || ue.Name != "nope" {
		t.Errorf("err = %#v", err)
	}
	if !strings.Contains(err.Error(), "unknown lexer nope") {
		t.Errorf("error string = %q", err.Error())
	}
}

func TestHighlightUnknownFormatter(t *testing.T) {
	_, err := Highlight("x", "ruby", "nope")
	if err == nil {
		t.Fatal("expected error")
	}
	ue := err.(*UnknownError)
	if ue.Kind != "formatter" {
		t.Errorf("kind = %q", ue.Kind)
	}
}

// --- formatters ---

func TestHTMLFormatter(t *testing.T) {
	f := HTMLFormatter{}
	if f.Tag() != "html" {
		t.Errorf("tag = %q", f.Tag())
	}
	toks := []TokenValue{
		{Text, "plain "},
		{Keyword, "def"},
		{Escape, "<raw>"}, // Escape passes through unescaped
	}
	got := f.Format(toks)
	want := `plain <span class="k">def</span><raw>`
	if got != want {
		t.Errorf("format = %q, want %q", got, want)
	}
}

func TestEscapeHTML(t *testing.T) {
	// Fast path: no special chars.
	if escapeHTML("plain") != "plain" {
		t.Error("plain should pass through")
	}
	// &, <, > escaped; \r dropped.
	if got := escapeHTML("a&b<c>d\re"); got != "a&amp;b&lt;c&gt;de" {
		t.Errorf("escape = %q", got)
	}
}

func TestWrapHTML(t *testing.T) {
	got := WrapHTML(`<span class="k">def</span>`)
	want := "<pre class=\"highlight\"><code><span class=\"k\">def</span></code></pre>\n"
	if got != want {
		t.Errorf("wrap = %q", got)
	}
}

func TestHTMLInlineFormatter(t *testing.T) {
	f := HTMLInlineFormatter{Theme: Github}
	if f.Tag() != "html_inline" {
		t.Errorf("tag = %q", f.Tag())
	}
	toks := []TokenValue{
		{Text, "plain"},
		{Keyword, "def"},
		{Escape, "&amp;"}, // Escape: escaped but no span
	}
	got := f.Format(toks)
	if !strings.Contains(got, `<span style="color: #cf222e">def</span>`) {
		t.Errorf("inline format = %q", got)
	}
	if !strings.HasPrefix(got, "plain") {
		t.Errorf("text should pass through: %q", got)
	}
}

func TestFindFormatter(t *testing.T) {
	if FindFormatter("html") == nil {
		t.Error("html formatter should be registered")
	}
	if FindFormatter("html_inline") == nil {
		t.Error("html_inline should be registered")
	}
	if FindFormatter("nope") != nil {
		t.Error("unknown formatter -> nil")
	}
}

// --- themes ---

func TestThemes(t *testing.T) {
	for _, name := range []string{"base16", "github", "thankful_eyes"} {
		th := FindTheme(name)
		if th == nil {
			t.Fatalf("theme %q not found", name)
		}
		if th.Name() != name {
			t.Errorf("theme name = %q, want %q", th.Name(), name)
		}
	}
	if FindTheme("nope") != nil {
		t.Error("unknown theme -> nil")
	}
}

func TestThemeStyleFor(t *testing.T) {
	// Exact match.
	if got := Github.StyleFor(Keyword); got != "color: #cf222e" {
		t.Errorf("Keyword style = %q", got)
	}
	// Ancestor resolution: Keyword.Constant has its own; Keyword.Reserved
	// falls back to Keyword.
	if got := Github.StyleFor(KeywordReserved); got != "color: #cf222e" {
		t.Errorf("KeywordReserved should resolve to Keyword: %q", got)
	}
	// A token with no style and whose ancestors have none falls back to Text.
	if got := Base16.StyleFor(NameDecorator); got != Base16.rules["Text"] {
		t.Errorf("undefined token should fall back to Text: %q", got)
	}
}

// --- coalesce / RegexLexer edge cases ---

func TestCoalesce(t *testing.T) {
	// Single element is returned as-is.
	in := []TokenValue{{Text, "a"}}
	if out := coalesce(in); len(out) != 1 || out[0].Value != "a" {
		t.Errorf("single coalesce = %v", out)
	}
	// Adjacent same-token runs merge; different tokens stay split.
	in = []TokenValue{{Text, "a"}, {Text, "b"}, {Keyword, "c"}, {Keyword, "d"}, {Text, "e"}}
	out := coalesce(in)
	if len(out) != 3 || out[0].Value != "ab" || out[1].Value != "cd" || out[2].Value != "e" {
		t.Errorf("coalesce = %v", out)
	}
	// Empty input.
	if coalesce(nil) != nil {
		t.Error("nil coalesce -> nil")
	}
}

// TestErrorByte exercises the no-rule-matched Error path: a control byte that no
// JSON rule accepts is emitted as an Error token.
func TestErrorByte(t *testing.T) {
	out := jsonLexer.Lex("\x01")
	if len(out) != 1 || out[0].Token != Error {
		t.Errorf("expected one Error token, got %v", out)
	}
}
