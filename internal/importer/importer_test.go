package importer

import (
	"testing"
)

func TestWPHTMLToMarkdown(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string // substring that should be present
	}{
		{
			name:  "paragraphs",
			input: "<p>Hello world</p><p>Second paragraph</p>",
			want:  "Hello world",
		},
		{
			name:  "headings",
			input: "<h2>Introduction</h2><p>Body text</p>",
			want:  "## Introduction",
		},
		{
			name:  "bold",
			input: "<p>This is <strong>bold</strong> text</p>",
			want:  "**bold**",
		},
		{
			name:  "italic",
			input: "<p>This is <em>italic</em> text</p>",
			want:  "*italic*",
		},
		{
			name:  "links",
			input: `<a href="https://example.com">click here</a>`,
			want:  "[click here](https://example.com)",
		},
		{
			name:  "images",
			input: `<img src="photo.jpg" alt="My photo" />`,
			want:  "![My photo](photo.jpg)",
		},
		{
			name:  "code blocks",
			input: "<pre><code>func main() {}</code></pre>",
			want:  "```",
		},
		{
			name:  "inline code",
			input: "<p>Use <code>fmt.Println</code> to print</p>",
			want:  "`fmt.Println`",
		},
		{
			name:  "unordered list",
			input: "<ul><li>One</li><li>Two</li></ul>",
			want:  "- One",
		},
		{
			name:  "blockquote",
			input: "<blockquote>Wise words</blockquote>",
			want:  "Wise words",
		},
		{
			name:  "entities",
			input: "<p>AT&amp;T &lt;3 &quot;quotes&quot;</p>",
			want:  `AT&T <3 "quotes"`,
		},
		{
			name:  "empty",
			input: "",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := wpHTMLToMarkdown(tt.input)
			if tt.want != "" && !contains(got, tt.want) {
				t.Errorf("wpHTMLToMarkdown() =\n%q\nshould contain:\n%q", got, tt.want)
			}
			if tt.want == "" && got != "" {
				t.Errorf("wpHTMLToMarkdown('') = %q, want empty", got)
			}
		})
	}
}

func TestWPStripHTML(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"<p>Hello</p>", "Hello"},
		{"<strong>Bold</strong> and <em>italic</em>", "Bold and italic"},
		{"no tags", "no tags"},
		{"", ""},
	}
	for _, tt := range tests {
		got := wpStripHTML(tt.input)
		if got != tt.want {
			t.Errorf("wpStripHTML(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestSlugifyWP(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Hello World", "hello-world"},
		{"My First Post!", "my-first-post"},
		{"with---dashes", "with-dashes"},
		{"", ""},
	}
	for _, tt := range tests {
		got := slugifyWP(tt.input)
		if got != tt.want {
			t.Errorf("slugifyWP(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestHTMLToMarkdown_Ghost(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "heading",
			input: "<h2>Test</h2>",
			want:  "## Test",
		},
		{
			name:  "paragraph",
			input: "<p>Hello world</p>",
			want:  "Hello world",
		},
		{
			name:  "link",
			input: `<a href="https://go.dev">Go</a>`,
			want:  "[Go](https://go.dev)",
		},
		{
			name:  "code",
			input: "<pre><code>code here</code></pre>",
			want:  "code here",
		},
		{
			name:  "empty",
			input: "",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := htmlToMarkdown(tt.input)
			if tt.want != "" && !contains(got, tt.want) {
				t.Errorf("htmlToMarkdown() =\n%q\nshould contain:\n%q", got, tt.want)
			}
		})
	}
}

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"photo.jpg", "photo.jpg"},
		{"my image (1).png", "my-image--1-.png"},
		{"../../etc/passwd", "..-..-etc-passwd"},
	}
	for _, tt := range tests {
		got := sanitizeFilename(tt.input)
		if got != tt.want {
			t.Errorf("sanitizeFilename(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestUnescapeHTML(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"&amp;", "&"},
		{"&lt;tag&gt;", "<tag>"},
		{"&quot;quoted&quot;", `"quoted"`},
		{"&#39;apos&#39;", "'apos'"},
		{"plain", "plain"},
	}
	for _, tt := range tests {
		got := unescapeHTML(tt.input)
		if got != tt.want {
			t.Errorf("unescapeHTML(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func contains(s, sub string) bool {
	return len(sub) == 0 || len(s) >= len(sub) && containsStr(s, sub)
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
