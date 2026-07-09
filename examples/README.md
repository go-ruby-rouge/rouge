# rouge examples

Runnable pure-Ruby usage of the `rouge` syntax highlighter, verified under the [rbgo](https://github.com/go-embedded-ruby) interpreter.

```sh
rbgo examples/rouge_usage.rb
```

| File | Shows |
| --- | --- |
| `rouge_usage.rb` | Highlight source with `Rouge.highlight` (the `html` formatter's CSS short-codes, the `html` default, and the `html_inline` themed formatter), look up a lexer with `Rouge::Lexer.find` and read its `#tag` / `#title` / `#aliases`, probe formatters with `Rouge::Formatter.find`, and rescue `Rouge::Error` on an unknown lexer. |
