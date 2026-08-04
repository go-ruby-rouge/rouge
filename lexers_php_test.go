// Copyright (c) the go-ruby-rouge/rouge authors
//
// SPDX-License-Identifier: BSD-3-Clause

package rouge

import (
	"os"
	"path/filepath"
	"testing"
)

// phpGoldenCases pair a PHP corpus file with the lexer configuration it is
// highlighted under. The .html goldens were captured from the reference rouge
// 5.0.0 gem's HTML formatter with funcnamehighlighting:false (the configuration
// this port models — see lexers_php.go), so they assert byte-for-byte
// faithfulness without needing the gem at test time. The inline case starts in
// PHP code (start_inline); the template case starts in the HTML-delegating root
// so it exercises the <?php … ?> / <?= … ?> tag transitions and escape.
var phpGoldenCases = []struct {
	file   string
	inline bool
}{
	{"php.php", true},
	{"php_tmpl.php", false},
	{"php_tmpl2.php", false},
}

// TestGoldenPHP highlights each PHP corpus file with the matching lexer
// configuration and compares the HTML to the gem-captured golden, byte for byte.
func TestGoldenPHP(t *testing.T) {
	for _, c := range phpGoldenCases {
		t.Run(c.file, func(t *testing.T) {
			src, err := os.ReadFile(filepath.Join("testdata", c.file))
			if err != nil {
				t.Fatal(err)
			}
			want, err := os.ReadFile(filepath.Join("testdata", c.file+".html"))
			if err != nil {
				t.Fatal(err)
			}
			var lx Lexer
			if c.inline {
				lx = FindFancy("php?start_inline=1")
			} else {
				lx = FindLexer("php")
			}
			got := HTMLFormatter{}.Format(lx.Lex(string(src)))
			if got != string(want) {
				t.Errorf("%s: HTML mismatch vs gem golden\n--- got ---\n%s\n--- want ---\n%s",
					c.file, got, want)
			}
		})
	}
}

// TestPHPEdges drives PHP lexer arms awkward to reach through the gem-golden
// corpus, asserting byte-for-byte against strings captured from the reference
// rouge 5.0.0 gem (funcnamehighlighting:false). It covers the `names` state's
// bare-catch classification (catch not followed by "(") and the guess-mode start
// state (an inline snippet with no leading angle bracket, which the start state
// hands straight to the php state).
func TestPHPEdges(t *testing.T) {
	inline := FindFancy("php?start_inline=1")
	if got, want := (HTMLFormatter{}).Format(inline.Lex("catch $e;")),
		`<span class="k">catch</span> <span class="nv">$e</span><span class="p">;</span>`; got != want {
		t.Errorf("bare catch:\n got: %q\nwant: %q", got, want)
	}
	// FindLexer("php") is guess-mode: the start state gotos php for a snippet
	// with no leading '<'.
	if got, want := (HTMLFormatter{}).Format(FindLexer("php").Lex("$x = 1;")),
		`<span class="nv">$x</span> <span class="o">=</span> <span class="mi">1</span><span class="p">;</span>`; got != want {
		t.Errorf("guess start:\n got: %q\nwant: %q", got, want)
	}
	// start_inline=0 keeps the require-<?php (root) behaviour.
	if got, want := (HTMLFormatter{}).Format(FindFancy("php?start_inline=0").Lex("<?php echo 1; ?>")),
		`<span class="cp">&lt;?php</span> <span class="k">echo</span> <span class="mi">1</span><span class="p">;</span> <span class="cp">?&gt;</span>`; got != want {
		t.Errorf("start_inline=0:\n got: %q\nwant: %q", got, want)
	}
}

// TestPHPDetect covers Rouge::Lexers::PHP.detect? (detectPHP) directly, since
// Guess's lexer ordering makes the individual arms hard to hit deterministically.
func TestPHPDetect(t *testing.T) {
	for _, c := range []struct {
		text string
		want bool
	}{
		{"#!/usr/bin/php\n$x = 1;", true}, // php shebang
		{"#!/bin/sh\necho hi", false},     // shebang without php
		{"<?hh\nclass X {}", false},       // Hack, explicitly not PHP
		{"<?php\necho 1;", true},          // opening tag
		{"just prose", false},             // no marker
	} {
		if got := detectPHP(c.text); got != c.want {
			t.Errorf("detectPHP(%q) = %v, want %v", c.text, got, c.want)
		}
	}
}

// TestPHPTruthy covers phpTruthy's falsey spellings and the truthy default.
func TestPHPTruthy(t *testing.T) {
	for v, want := range map[string]bool{
		"1": true, "true": true, "yes": true, "on": true,
		"": false, "0": false, "false": false, "no": false, "off": false, "nil": false,
	} {
		if got := phpTruthy(v); got != want {
			t.Errorf("phpTruthy(%q) = %v, want %v", v, got, want)
		}
	}
}

// TestPHPFancyOptions covers the FindFancy option paths for PHP: no option key
// leaves the base (guess) lexer, an unknown-option spec is ignored, and a
// non-PHP lexer ignores options entirely.
func TestPHPFancyOptions(t *testing.T) {
	base := FindLexer("php")
	if got := FindFancy("php?foo=bar"); got != base {
		t.Errorf("php?foo=bar should keep the base lexer")
	}
	// A lexer without an options hook returns itself for any spec.
	if got := FindFancy("ruby?x=1"); got != FindLexer("ruby") {
		t.Errorf("ruby?x=1 should keep the base ruby lexer")
	}
	// start_inline configures a distinct variant.
	if got := FindFancy("php?start_inline=1"); got == base {
		t.Errorf("php?start_inline=1 should be a configured variant, not the base")
	}
}
