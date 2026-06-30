<p align="center"><img src="https://raw.githubusercontent.com/go-ruby-rouge/brand/main/social/go-ruby-rouge-rouge.png" alt="go-ruby-rouge/rouge" width="720"></p>

# rouge — go-ruby-rouge

[![Docs](https://img.shields.io/badge/docs-mkdocs--material-DC2626)](https://go-ruby-rouge.github.io/docs/)
[![License](https://img.shields.io/badge/license-BSD--3--Clause-blue)](LICENSE)
[![Go](https://img.shields.io/badge/go-1.26.4%2B-00ADD8)](https://go.dev/dl/)
[![Coverage](https://img.shields.io/badge/coverage-100%25-1a7f37)](#tests--coverage)

**A pure-Go (no cgo) reimplementation of the Ruby [`rouge`](https://github.com/rouge-ruby/rouge)
syntax highlighter** — the state-machine regex lexers, the token hierarchy, and
the HTML formatters of the gem, emitting exactly the CSS short-codes (`k`, `s`,
`c`, `nf`, …) `rouge` emits. It tokenizes source and renders `<span class>` HTML
that matches the gem **byte-for-byte** on the supported lexers, **without any
Ruby runtime**.

It is a sibling of [go-ruby-regexp](https://github.com/go-ruby-regexp/regexp)
(the Onigmo engine it lexes with), [go-ruby-erb](https://github.com/go-ruby-erb/erb),
and [go-ruby-yaml](https://github.com/go-ruby-yaml/yaml), and is bound into
[go-embedded-ruby](https://github.com/go-embedded-ruby/ruby) to give `rbgo` a
CGO-free `Rouge.highlight`.

## API

```go
import "github.com/go-ruby-rouge/rouge"

// Rouge.highlight(text, lexer, formatter) -> HTML
html, err := rouge.Highlight(`puts "hello"`, "ruby", "html")
// => <span class="nb">puts</span> <span class="s2">"hello"</span>

// Rouge::Lexer.find / find_fancy / guess
lex := rouge.FindLexer("rb")          // by tag or alias (case-insensitive)
lex = rouge.FindFancy("ruby?opt=1")   // "tag" or "tag?opts"
lex = rouge.Guess(src)                // content sniff; never nil (PlainText fallback)

// Token stream + the HTML / HTMLInline formatters directly
toks := lex.Lex(src)                          // []TokenValue{Token, Value}
out := rouge.HTMLFormatter{}.Format(toks)     // <span class="…"> per token
page := rouge.WrapHTML(out)                   // <pre class="highlight"><code>…</code></pre>
inline := rouge.HTMLInlineFormatter{Theme: rouge.Github}.Format(toks) // inline styles
```

- **Token model** — `rouge.Token` mirrors `Rouge::Token::Tokens`: a tree with a
  dotted `Qualname()` (`Literal.String.Double`), the CSS `Shortname` the HTML
  formatter emits (`s2`), `Matches(ancestor)`, and `TokenByName("Keyword.Constant")`.
- **Formatters** — `HTML` (`<span class="shortcode">`, the gem's exact codes;
  `Text`/`Escape` pass through) and `HTMLInline` (inline `style=` from a `Theme`),
  registered as `"html"` / `"html_inline"`; plus `WrapHTML` for the gem's
  `<pre class="highlight"><code>` page wrapper.
- **Themes** — `Base16`, `Github`, `ThankfulEyes`: their resolved
  `rendered_rules` captured token-by-token, with `StyleFor` walking the ancestor
  chain exactly like `Rouge::Theme.get_style`.

## Lexers

Each lexer is a near-mechanical transcription of the gem's `state … rule …`
definition, validated by a **differential oracle** against the reference
`rouge` 5.0.0 gem: every committed `testdata/*` corpus file is highlighted and
compared **byte-for-byte** to the gem's HTML (the `*.html` goldens).

**Byte-faithful** (gem-identical HTML on the benign corpus):

| Lexer | Tag(s) | Notes |
|-------|--------|-------|
| Ruby | `ruby`, `rb` | symbols, `%w`/`%r`/`%(…)` sigils, heredocs (`<<`/`<<-`/`<<~`), interpolation, ternary, method-call disambiguation |
| Go | `go`, `golang` | |
| Python | `python`, `py` | f-strings, `case`/`match`, decorators, doctest |
| JavaScript | `javascript`, `js` | regex/template/object/ternary state machine |
| JSON | `json` | |
| YAML | `yaml`, `yml` | indentation state machine, anchors, block scalars, tags |
| HTML | `html` | delegates `<script>`/`<style>` to JS/CSS |
| CSS | `css` | property/builtin/color/function sets |
| Shell | `shell`, `bash`, `zsh`, `ksh`, `sh` | heredocs, `$()`/`${}`/`$(())`, `case` |
| Diff | `diff`, `patch`, `udiff` | with content detector |
| Markdown | `markdown`, `md`, `mkd` | delegates fenced code to the named lexer, frontmatter to YAML |
| SQL | `sql` | case-insensitive keyword/type sets |
| PlainText | `plaintext`, `text` | fallback |

**Documented simplifications** (honest deviations from the gem):

- **Markdown** fenced code blocks: the closed-fence form is byte-faithful; the
  gem's anonymous dynamic state for an *unclosed* fence is approximated.
- **HTML** `<script>`/`<style>` delegation is per-chunk (matching the gem); like
  the gem, a token straddling a bare `<` inside an embedded string can split
  (the gem itself emits an `Error` there).
- **Ruby** `%`-sigil string bodies are scanned in one pass (nesting +
  interpolation + escapes preserved); the heredoc `(?<!\p{Word})` guard is done
  against the preceding byte (the engine has no variable-width lookbehind).
- **Guess** uses each lexer's content detector only (a subset of the gem's
  multi-signal guesser) — enough for the unambiguous formats (diff, YAML,
  shebangs, doctype).

## Engine

Lexing runs on the sibling [go-ruby-regexp](https://github.com/go-ruby-regexp/regexp)
Onigmo engine via its `MatchAt(src, pos)` primitive, which anchors `\G` at the
scan cursor while keeping the whole buffer visible — so a rule's `^` matches only
at a true line start and lookbehind sees the real prefix, exactly the
`StringScanner` semantics a `RegexLexer` needs. Building this port upstreamed a
handful of Onigmo-faithfulness fixes to that engine (`\<punct>` and `\f \v \a \e`
escapes, the `\p{Nl}`/`\p{No}`/`\p{Cf}` property subcategories, and `MatchAt`
itself).

## Tests & coverage

Tests are **deterministic and Ruby-free**: the differential goldens were captured
from the `rouge` gem once and committed, so `go test` reproduces gem-faithfulness
without the gem on `PATH` (the gem is not stdlib, and the cross-arch / Windows CI
lanes have no Ruby). Coverage is **100%** including error branches.

```
go test ./...
```

CI builds and tests on **3 OSes** (linux/macos/windows) and the **six 64-bit Go
targets** (amd64/arm64/riscv64/loong64/ppc64le/s390x).

## License

BSD-3-Clause — see [LICENSE](LICENSE). Copyright the go-ruby-rouge/rouge authors.
The bundled themes' colour tables derive from the BSD-3-licensed `rouge` gem.
