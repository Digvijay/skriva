package handler

import (
	"strings"
	"testing"
)

func FuzzSanitizeUntrustedHTML(f *testing.F) {
	f.Add("<script>alert(1)</script>")
	f.Add("<img onerror=alert(1) src=x>")
	f.Add("<scr<script>ipt>alert(1)</script>")
	f.Add("<a href=\"javascript:alert(1)\">click</a>")
	f.Add("")
	f.Add("<p>Hello <b>World</b></p>")
	f.Fuzz(func(t *testing.T, input string) {
		result := sanitizeUntrustedHTML(input)
		// Must never contain <script
		if containsIgnoreCase(result, "<script") {
			t.Errorf("sanitizer failed: output contains <script: %q", result)
		}
	})
}

func FuzzValidateExternalURL(f *testing.F) {
	f.Add("https://example.com")
	f.Add("http://localhost")
	f.Add("http://127.0.0.1")
	f.Add("http://169.254.169.254")
	f.Add("javascript:alert(1)")
	f.Add("")
	f.Fuzz(func(t *testing.T, rawURL string) {
		validateExternalURL(rawURL) // must not panic
	})
}

func containsIgnoreCase(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}
