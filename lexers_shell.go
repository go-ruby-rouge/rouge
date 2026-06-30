// Copyright (c) the go-ruby-rouge/rouge authors
//
// SPDX-License-Identifier: BSD-3-Clause

package rouge

import (
	"strings"

	onigmo "github.com/go-ruby-regexp/regexp"
)

// --- Shell ---
// A faithful transcription of Rouge::Lexers::Shell (rouge 5.0.0), covering sh /
// bash / zsh / ksh. The keyword and builtin alternations are the gem's, joined
// with "|". The heredoc terminator is tracked in lexState.heredocStr exactly as
// the gem tracks @heredocstr.

// shellKeywords and shellBuiltins are the gem's KEYWORDS / BUILTINS lists,
// pipe-joined for use inside a regex alternation.
const shellKeywords = `if|fi|else|while|do|done|for|then|return|function|select|continue|until|esac|elif|in`

const shellBuiltins = `alias|bg|bind|break|builtin|caller|cd|command|compgen|` +
	`complete|declare|dirs|disown|enable|eval|exec|exit|export|false|fc|fg|` +
	`getopts|hash|help|history|jobs|let|local|logout|mapfile|popd|pushd|pwd|` +
	`read|readonly|set|shift|shopt|source|suspend|test|time|times|trap|true|` +
	`type|typeset|ulimit|umask|unalias|unset|wait|cat|tac|nl|od|base32|base64|` +
	`fmt|pr|fold|head|tail|split|csplit|wc|sum|cksum|b2sum|md5sum|sha1sum|` +
	`sha224sum|sha256sum|sha384sum|sha512sum|sort|shuf|uniq|comm|ptx|tsort|cut|` +
	`paste|join|tr|expand|unexpand|ls|dir|vdir|dircolors|cp|dd|install|mv|rm|` +
	`shred|link|ln|mkdir|mkfifo|mknod|readlink|rmdir|unlink|chown|chgrp|chmod|` +
	`touch|df|du|stat|sync|truncate|echo|printf|yes|expr|tee|basename|dirname|` +
	`pathchk|mktemp|realpath|pwd|stty|printenv|tty|id|logname|whoami|groups|` +
	`users|who|date|arch|nproc|uname|hostname|hostid|uptime|chcon|runcon|chroot|` +
	`env|nice|nohup|stdbuf|timeout|kill|sleep|factor|numfmt|seq|tar|grep|sudo|` +
	`awk|sed|gzip|gunzip`

var shellLexer = func() *RegexLexer {
	b := newRegexLexer("shell", "shell", "bash", "zsh", "ksh", "sh")
	b.filenames("*.sh", "*.bash", "*.zsh", "*.ksh", ".bashrc", ".profile")
	b.detectWith(func(text string) bool {
		if strings.HasPrefix(text, "#compdef") || strings.HasPrefix(text, "#autoload") {
			return true
		}
		if strings.HasPrefix(text, "#!") {
			line := text
			if nl := strings.IndexByte(text, '\n'); nl >= 0 {
				line = text[:nl]
			}
			if re, err := onigmo.Compile(`(ba|z|k)?sh`); err == nil && re.MatchString(line) {
				return true
			}
		}
		return false
	})

	b.state("basic").
		rule(`#.*$`, Comment).
		rule(`(?:`+shellKeywords+`)\s*\b`, Keyword).
		rule(`case\b`, Keyword, push("case")).
		rule(`(?:`+shellBuiltins+`)\s*\b(?!(?:\.|-))`, NameBuiltin).
		rule(`[.](?=\s)`, NameBuiltin).
		groupsRule(`(\w+)(=)`, NameVariable, Operator).
		rule(`[\[\]{}()!=>]`, Operator).
		rule(`&&|\|\|`, Operator).
		rule(`<<<`, Operator).
		cb(`(<<-?)(\s*)(['"]?)(\\?)(\w+)(\3)`, func(l *lexState, m *onigmo.MatchData) {
			l.emit(Operator, m.Str(1))
			l.emit(Text, m.Str(2))
			l.emit(LiteralStringHeredoc, m.Str(3))
			l.emit(LiteralStringHeredoc, m.Str(4))
			l.emit(NameConstant, m.Str(5))
			l.emit(LiteralStringHeredoc, m.Str(6))
			l.heredocStr = m.Str(5)
			l.pos = m.End(0)
			l.push("heredoc")
		})

	b.state("heredoc").
		rule(`\n`, LiteralStringHeredoc, push("heredoc_nl")).
		rule(`[^$\n\\]+`, LiteralStringHeredoc).
		mixin("interp").
		rule(`[$]`, LiteralStringHeredoc)

	b.state("heredoc_nl").
		cb(`\s*(\w+)\s*\n`, func(l *lexState, m *onigmo.MatchData) {
			if m.Str(1) == l.heredocStr {
				l.emit(NameConstant, m.Str(0))
				l.pos = m.End(0)
				l.pop(2)
			} else {
				l.emit(LiteralStringHeredoc, m.Str(0))
				l.pos = m.End(0)
			}
		}).
		cb(``, func(l *lexState, m *onigmo.MatchData) { l.pop(1) })

	b.state("double_quotes").
		rule(`(?:\$#?)?"`, LiteralStringDouble, pop()).
		mixin("interp").
		rule("[^\"`\\\\$]+", LiteralStringDouble)

	b.state("ansi_string").
		rule(`\\.`, LiteralStringEscape).
		rule(`[^\\']+`, LiteralStringSingle).
		mixin("single_quotes")

	b.state("single_quotes").
		rule(`'`, LiteralStringSingle, pop()).
		rule(`[^']+`, LiteralStringSingle)

	b.state("data").
		rule(`\s+`, Text).
		rule(`\\.`, LiteralStringEscape).
		rule(`\$?"`, LiteralStringDouble, push("double_quotes")).
		rule(`\$'`, LiteralStringSingle, push("ansi_string")).
		rule(`'`, LiteralStringSingle, push("single_quotes")).
		rule(`\*`, Keyword).
		rule(`;`, Punctuation).
		rule(`--?[\w-]+`, NameTag).
		rule("[^=*\\s{}()$\"'`;\\\\<]+", Text).
		rule(`\d+(?= |\z)`, LiteralNumber).
		rule(`<`, Text).
		mixin("interp")

	b.state("curly").
		rule(`}`, Keyword, pop()).
		rule(`:-`, Keyword).
		rule(`[a-zA-Z0-9_]+`, NameVariable).
		rule("[^}:\"`'$]+", Punctuation).
		mixin("root")

	b.state("paren_interp").
		rule(`\)`, LiteralStringInterpol, pop()).
		rule(`\(`, Operator, push("paren_inner")).
		mixin("root")

	b.state("paren_inner").
		rule(`\(`, Operator, pushSelf()).
		rule(`\)`, Operator, pop()).
		mixin("root")

	b.state("math").
		rule(`\)\)`, Keyword, pop()).
		rule(`[-+*/%^|&!]|\*\*|\|\|`, Operator).
		rule(`\d+(?:#\w+)?`, LiteralNumber).
		mixin("root")

	b.state("case").
		rule(`esac\b`, Keyword, pop()).
		rule(`\|`, Punctuation).
		rule(`\)`, Punctuation, push("case_stanza")).
		mixin("root")

	b.state("case_stanza").
		rule(`;;`, Punctuation, pop()).
		mixin("root")

	b.state("backticks").
		rule("`", LiteralStringBacktick, pop()).
		mixin("root")

	b.state("interp").
		rule(`\\$`, LiteralStringEscape).
		rule(`\\.`, LiteralStringEscape).
		rule(`\$\(\(`, Keyword, push("math")).
		rule(`\$\(`, LiteralStringInterpol, push("paren_interp")).
		rule(`\$\{#?`, Keyword, push("curly")).
		rule("`", LiteralStringBacktick, push("backticks")).
		rule(`\$#?(?:\w+|.)`, NameVariable).
		rule(`\$[*@]`, NameVariable)

	b.state("root").
		mixin("basic").
		mixin("data")

	return b.done()
}()
