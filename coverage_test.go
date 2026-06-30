// Copyright (c) the go-ruby-rouge/rouge authors
//
// SPDX-License-Identifier: BSD-3-Clause

package rouge

import "testing"

// reconstruct asserts that lexing src consumes the whole input (the concatenated
// token values equal src) and returns the token count. It is used to drive
// lexer rule branches that are awkward to reach through a gem-golden corpus
// (empty-pattern pops, fallthrough decline arms, rare delimiters); these assert
// the lexer makes progress and consumes everything rather than gem parity.
func reconstruct(t *testing.T, l Lexer, src string) {
	t.Helper()
	var b string
	for _, tv := range l.Lex(src) {
		b += tv.Value
	}
	if b != src {
		t.Errorf("lex did not reconstruct input\n got: %q\nwant: %q", b, src)
	}
}

// TestRubyCoverageEdges drives Ruby-lexer rule arms not covered by the golden
// corpus: empty-name heredocs, def with an explicit receiver and parenthesised
// args, a no-space ternary colon, method-call slash/space disambiguation, and a
// %r regex with flags.
func TestRubyCoverageEdges(t *testing.T) {
	for _, src := range []string{
		"x = foo<<HEREDOC\nbody\nHEREDOC\n", // heredoc after a word (isWordByte guard)
		"s = <<\"\"\nbody\n\n",              // empty-name quoted heredoc
		"def obj.meth(a)\n  a\nend\n",       // funcname with receiver + defexpr "("
		"def (x).y\nend\n",                  // funcname defexpr via "("
		"def(z)\nend\n",                     // def immediately followed by "(" -> funcname defexpr
		"r = cond ?x:y\n",                   // ternary, no-space colon
		"a = b.c (d)\n",                     // method_call with spaced "("
		"v = obj.meth\nx = 1 / 2\n",         // method_call slash -> operator
		"obj.foo / 2\n",                     // method_call_spaced "/" disambiguation
		"obj.foo bar\n",                     // method_call_spaced whitespace -> expr_start
		"re = %r/ab/imx\n",                  // %r regex sigil + flags
		"re2 = %r(x)\n",                     // %r with paren delim
		"v = %r{ab}i\n",                     // %r{}: regex_flags push (typ == "r")
		"q = %w{a b}\n",                     // %w with brace delim
		"class (expr)\nend\n",               // classname "(" arm (anonymous class)
		"def ((a))\nend\n",                  // defexpr nested "(" arm
		"x.y/z\n",                           // method_call "/" immediately (no space)
		"x.y%z\n",                           // method_call "%" immediately
		"obj.call arg\n",                    // method_call_spaced whitespace -> expr_start
		"a.b\n",                             // method_call then newline (pop arm)
		"%r(unterminated",                   // unterminated %r -> end-of-loop regex_flags push
		"def",                               // funcname: EOF right after def -> empty pop arm
		"class \n",                          // classname: whitespace then EOF -> empty pop arm
		"x = <<E\nbody no terminator\n",     // heredoc with no terminator -> test_heredoc empty pop
		"x = a ? b\n",                       // ternary without ':' -> root mixin then queue pop
	} {
		reconstruct(t, rubyLexer, src)
	}
}

// TestYAMLCoverageEdges drives YAML rule arms: block sequence indicators after
// indentation, a literal block scalar with blank lines and an indent indicator,
// and a flow-context plain scalar.
func TestYAMLCoverageEdges(t *testing.T) {
	for _, src := range []string{
		"a:\n  - x\n  - y\n",            // "- " collection indicator at indent
		"s: |\n  one\n\n  three\n",      // block scalar with a blank line
		"t: |1\n  body\n",               // block scalar with explicit indent indicator
		"m: {a: 1, b: 2}\n",             // flow mapping plain scalars
		"l: [one, two]\n",               // flow sequence plain scalars
		"k:\n  ? complex\n  : value\n",  // explicit key/value indicators
		"root:\n  child:\n    - item\n", // nested mapping then a "- " indicator
		"blk: |\n  line\n  \n  more\n",  // block scalar with a spaces-only blank line
		"a: 1\n  \nb: 2\n",              // a spaces-only line at the document root
		"x:\n  - a\n",                   // "- " preceded only by indentation (continue_indent arm)
		"y: |2\n    deeper\n  back\n",   // block scalar explicit indent then a dedent line
		"- top\n- level\n",              // sequence at column 0
		"a: |-\n  text\n",               // block scalar with a chomping flag only (inc empty)
		"a:\n  \n  b: 1\n",              // indentation followed by a blank line (\s*?\n arm)
		"m:\n   - one\n   - two\n",      // spaces before "-" collection indicator
		"a:\n  ? key\n  : val\n",        // explicit "? key / : val" with leading indentation
		"a: |1\n     \n  x\n",           // explicit-indent block scalar, over-long blank line
		"a: |9\n  \n         deep\n",    // explicit indent > blank-line width (indentMark clamp)
		"a: |-5\n  text\n",              // block-scalar header digit in the second group
		"a: |+2\n  text\n",              // chomping "+" then an indent digit (second group)
	} {
		reconstruct(t, yamlLexer, src)
	}
}

// TestPythonCoverageEdges drives Python rule arms: from/import early pops, a
// post-dot non-call (the pop arm), and an escaped vs raw string body.
func TestPythonCoverageEdges(t *testing.T) {
	for _, src := range []string{
		"from\n",                          // from with nothing after -> pop arm
		"import\n",                        // import with nothing after -> pop arm
		"from x\n",                        // from <name> then no import -> from_import pop arm
		"x = a.\n",                        // post_dot with no call -> pop arm
		"s = \"a\\tb\"\n",                 // escaped string body (non-raw)
		"r = r\"a\\tb\"\n",                // raw string body
		"x.y.z\n",                         // dotted attribute chain
		"s = \"\"\"\ntext line\n\"\"\"\n", // triple-quoted string, plain line (newline non-prompt)
		"s = '''\nhello\nworld'''\n",      // triple-quoted string spanning plain lines
		"s = \"\"\"a{b}c\"\"\"\n",         // non-f triple string with "{" (the non-interp else arm)
	} {
		reconstruct(t, pythonLexer, src)
	}
}

// TestJavaScriptCoverageEdges drives JS rule arms: a regex containing a newline
// (the error/pop arm), a nested object literal (push-self), and a regex group.
func TestJavaScriptCoverageEdges(t *testing.T) {
	for _, src := range []string{
		"x = /ab\nc/\n",                // newline in regex -> error/pop
		"o = { a: { b: { c: 1 } } }\n", // nested object literals
		"p = { x: 1, y: { z: 2 } }\n",  // object with a nested object value (push-self)
		"r = /[a-z\nx]/\n",             // newline inside a regex char group
		"r2 = /(?:ab)+/\n",             // regex with a group
		"o = {a:{b:1}}\n",              // tightly-nested object literals (push-self)
		"x = {{a:1}}\n",                // object literal containing a bare "{" (object push-self)
	} {
		reconstruct(t, javascriptLexer, src)
	}
}
