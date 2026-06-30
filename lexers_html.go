// Copyright (c) the go-ruby-rouge/rouge authors
//
// SPDX-License-Identifier: BSD-3-Clause

package rouge

import (
	"strings"

	onigmo "github.com/go-ruby-regexp/regexp"
)

// --- HTML ---
// A faithful transcription of Rouge::Lexers::HTML (rouge 5.0.0). Embedded
// <script> and <style> bodies are delegated to the JavaScript and CSS lexers.
//
// Simplified: the gem delegates each "[^<]+" chunk to a single, continuation-
// preserving sub-lexer instance (so a string split across "<" boundaries keeps
// its state). This port re-lexes each delegated chunk from the sub-lexer's root,
// which is byte-identical for the common case where script/style bodies do not
// straddle a bare "<". This is documented in the README.

// htmlTagName is the gem's opening/closing tag-name class. The "·" (U+00B7
// middle dot) is written as its literal UTF-8 bytes.
const htmlTagName = `[\p{L}:_-][\p{Word}\p{Cf}:.·-]*`

var htmlLexer = func() *RegexLexer {
	b := newRegexLexer("html", "HTML")
	b.filenames("*.htm", "*.html", "*.xhtml")
	b.detectWith(func(text string) bool {
		if re, err := onigmo.Compile(`(?i)<!doctype\s+[^>]*\bhtml\b`); err == nil && re.MatchString(text) {
			return true
		}
		if strings.HasPrefix(text, "<?xml") {
			return false
		}
		if re, err := onigmo.Compile(`<\s*html\b`); err == nil && re.MatchString(text) {
			return true
		}
		return false
	})

	b.state("root").
		rule(`(?m)[^<&]+`, Text).
		rule(`&\S*?;`, NameEntity).
		rule(`(?im)<!DOCTYPE .*?>`, CommentPreproc).
		rule(`(?m)<!\[CDATA\[.*?\]\]>`, CommentPreproc).
		rule(`<!--`, Comment, push("comment")).
		rule(`(?m)<\?.*?\?>`, CommentPreproc).
		cb(`(?m)<\s*script\s*`, func(l *lexState, m *onigmo.MatchData) {
			l.emit(NameTag, m.Str(0))
			l.pos = m.End(0)
			l.push("script_content")
			l.push("tag")
		}).
		cb(`(?m)<\s*style\s*`, func(l *lexState, m *onigmo.MatchData) {
			l.emit(NameTag, m.Str(0))
			l.pos = m.End(0)
			l.push("style_content")
			l.push("tag")
		}).
		rule(`</`, NameTag, push("tag_end")).
		rule(`<`, NameTag, push("tag_start")).
		rule(`(?m)<\s*`+htmlTagName, NameTag, push("tag")).
		rule(`(?m)<\s*/\s*`+htmlTagName+`\s*>`, NameTag)

	b.state("tag_end").
		mixin("tag_end_end").
		cb(htmlTagName, func(l *lexState, m *onigmo.MatchData) {
			l.emit(NameTag, m.Str(0))
			l.pos = m.End(0)
			l.goTo("tag_end_end")
		})

	b.state("tag_end_end").
		rule(`\s+`, Text).
		rule(`>`, NameTag, pop())

	b.state("tag_start").
		rule(`\s+`, Text).
		cb(htmlTagName, func(l *lexState, m *onigmo.MatchData) {
			l.emit(NameTag, m.Str(0))
			l.pos = m.End(0)
			l.goTo("tag")
		}).
		cb(``, func(l *lexState, m *onigmo.MatchData) { l.goTo("tag") })

	b.state("comment").
		rule(`[^-]+`, Comment).
		rule(`-->`, Comment, pop()).
		rule(`-`, Comment)

	b.state("tag").
		rule(`(?m)\s+`, Text).
		rule(`(?m)[\p{L}:_\[\]()*.-][\p{Word}\p{Cf}:.·\[\]()*-]*\s*=\s*`, NameAttribute, push("attr")).
		rule(`[\p{L}:_*#-][\p{Word}\p{Cf}:.·*#-]*`, NameAttribute).
		rule(`(?m)/?\s*>`, NameTag, pop())

	b.state("attr").
		cb(`"`, func(l *lexState, m *onigmo.MatchData) {
			l.emit(LiteralString, m.Str(0))
			l.pos = m.End(0)
			l.goTo("dq")
		}).
		cb(`'`, func(l *lexState, m *onigmo.MatchData) {
			l.emit(LiteralString, m.Str(0))
			l.pos = m.End(0)
			l.goTo("sq")
		}).
		rule(`[^\s>]+`, LiteralString, pop())

	b.state("dq").
		rule(`"`, LiteralString, pop()).
		rule(`[^"]+`, LiteralString)

	b.state("sq").
		rule(`'`, LiteralString, pop()).
		rule(`[^']+`, LiteralString)

	// script_content / style_content delegate the embedded body to the JavaScript
	// / CSS lexer, matching the gem. Like the gem, delegation is per "[^<]+" chunk
	// (plus a bare "<"); the gem reuses one continuation-preserving sub-lexer
	// instance, which this port approximates by re-lexing each chunk from the
	// sub-lexer's root. The two coincide except when a token straddles a bare "<"
	// (e.g. a "<" inside a CSS string), where the gem itself emits an Error token;
	// this divergence is documented in the README.
	b.state("script_content").
		cb(`[^<]+`, htmlDelegateChunk(javascriptLexer)).
		rule(`(?m)<\s*/\s*script\s*>`, NameTag, pop()).
		cb(`<`, htmlDelegateChunk(javascriptLexer))

	b.state("style_content").
		cb(`[^<]+`, htmlDelegateChunk(cssLexer)).
		rule(`(?m)<\s*/\s*style\s*>`, NameTag, pop()).
		cb(`<`, htmlDelegateChunk(cssLexer))

	return b.done()
}()

// htmlDelegateChunk returns a callback that delegates the matched chunk to sub
// and advances the cursor (the gem's `delegate @lang`).
func htmlDelegateChunk(sub Lexer) func(l *lexState, m *onigmo.MatchData) {
	return func(l *lexState, m *onigmo.MatchData) {
		l.pos = m.End(0)
		l.delegate(sub, m.Str(0))
	}
}
