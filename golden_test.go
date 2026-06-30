// Copyright (c) the go-ruby-rouge/rouge authors
//
// SPDX-License-Identifier: BSD-3-Clause

package rouge

import (
	"os"
	"path/filepath"
	"testing"
)

// goldenCases maps each testdata corpus file to the lexer tag it is highlighted
// with. The golden output (file + ".html") was captured from the reference
// rouge 5.0.0 gem's HTML formatter, so these tests assert byte-for-byte
// faithfulness without needing the gem at test time (CI has no Ruby on PATH for
// the cross-arch lanes, and the gem is not stdlib anywhere). The corpus is
// chosen to exercise every rule of each lexer.
var goldenCases = []struct {
	file string
	tag  string
}{
	{"go.go", "go"},
	{"css.css", "css"},
	{"css2.css", "css"},
	{"css3.css", "css"},
	{"css4.css", "css"},
	{"shell.sh", "shell"},
	{"shell2.sh", "shell"},
	{"shell3.sh", "shell"},
	{"shell4.sh", "shell"},
	{"python.py", "python"},
	{"python2.py", "python"},
	{"python3.py", "python"},
	{"python4.py", "python"},
	{"python5.py", "python"},
	{"javascript.js", "javascript"},
	{"javascript2.js", "javascript"},
	{"javascript3.js", "javascript"},
	{"javascript4.js", "javascript"},
	{"javascript5.js", "javascript"},
	{"javascript6.js", "javascript"},
	{"html.html", "html"},
	{"html2.html", "html"},
	{"html4.html", "html"},
	{"yaml.yaml", "yaml"},
	{"yaml2.yaml", "yaml"},
	{"yaml3.yaml", "yaml"},
	{"yaml4.yaml", "yaml"},
	{"yaml5.yaml", "yaml"},
	{"yaml6.yaml", "yaml"},
	{"yaml7.yaml", "yaml"},
	{"json.json", "json"},
	{"sql.sql", "sql"},
	{"sql2.sql", "sql"},
	{"diff.diff", "diff"},
	{"markdown.md", "markdown"},
	{"markdown2.md", "markdown"},
	{"markdown3.md", "markdown"},
	{"markdown4.md", "markdown"},
	{"ruby.rb", "ruby"},
	{"ruby2.rb", "ruby"},
	{"ruby3.rb", "ruby"},
	{"ruby4.rb", "ruby"},
	{"ruby5.rb", "ruby"},
	{"ruby7.rb", "ruby"},
	{"ruby8.rb", "ruby"},
	{"ruby9.rb", "ruby"},
	{"ruby10.rb", "ruby"},
	{"ruby11.rb", "ruby"},
}

// TestGolden highlights each corpus file and compares the HTML to the gem-
// captured golden, byte for byte.
func TestGolden(t *testing.T) {
	for _, c := range goldenCases {
		t.Run(c.file, func(t *testing.T) {
			src, err := os.ReadFile(filepath.Join("testdata", c.file))
			if err != nil {
				t.Fatal(err)
			}
			want, err := os.ReadFile(filepath.Join("testdata", c.file+".html"))
			if err != nil {
				t.Fatal(err)
			}
			got, err := Highlight(string(src), c.tag, "html")
			if err != nil {
				t.Fatal(err)
			}
			if got != string(want) {
				t.Errorf("%s: HTML mismatch vs gem golden\n--- got ---\n%s\n--- want ---\n%s",
					c.file, got, want)
			}
		})
	}
}

// TestGoldenPlainTextAndWrap covers the PlainText lexer and the WrapHTML page
// wrapper end to end (PlainText has no gem-divergent rules, so it is checked
// directly rather than via a golden).
func TestGoldenPlainTextAndWrap(t *testing.T) {
	inner, err := Highlight("just text\nlines", "text", "html")
	if err != nil {
		t.Fatal(err)
	}
	if inner != "just text\nlines" {
		t.Errorf("plaintext html = %q", inner)
	}
	page := WrapHTML(inner)
	if page != "<pre class=\"highlight\"><code>just text\nlines</code></pre>\n" {
		t.Errorf("wrapped page = %q", page)
	}
}
