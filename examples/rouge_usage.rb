# frozen_string_literal: true

require "rouge"

# Rouge.highlight(text, lexer = "text", formatter = "html") tokenizes source with
# the named lexer and renders it. The default "html" formatter emits the gem's
# exact CSS short-codes as <span class="…"> (nb = builtin, s2 = double-quoted string).
puts Rouge.highlight(%q{puts "hello"}, "ruby", "html")
# => <span class="nb">puts</span> <span class="s2">"hello"</span>

# The formatter defaults to "html" when omitted (n = name, o = operator, mi = integer).
puts Rouge.highlight("x = 1", "ruby")
# => <span class="n">x</span> <span class="o">=</span> <span class="mi">1</span>

# "html_inline" bakes a theme's colors into inline style= attributes instead of classes.
puts Rouge.highlight("x = 1", "ruby", "html_inline")

# Rouge::Lexer.find(name) resolves a lexer by tag or alias and exposes its metadata;
# an unknown name returns nil, just like the gem.
lex = Rouge::Lexer.find("rb")
puts "#{lex.tag} / #{lex.title} / #{lex.aliases.inspect}" # => ruby / Ruby / ["rb"]
puts Rouge::Lexer.find("no-such-lexer").inspect           # => nil

# Rouge::Formatter.find(tag) reports whether a formatter is registered (its tag, or nil).
puts Rouge::Formatter.find("html").inspect  # => "html"
puts Rouge::Formatter.find("bogus").inspect # => nil

# An unknown lexer or formatter raises Rouge::Error.
begin
  Rouge.highlight("x", "no-such-lexer")
rescue Rouge::Error => e
  puts "rescued Rouge::Error: #{e.message}"
end
