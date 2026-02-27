package handler

import (
	"testing"
	"time"
)

func TestGenerateTOTPSecret(t *testing.T) {
	secret, err := generateTOTPSecret()
	if err != nil {
		t.Fatalf("generateTOTPSecret() error: %v", err)
	}
	if len(secret) < 16 {
		t.Errorf("secret too short: %d chars", len(secret))
	}
	// Should be base32
	for _, c := range secret {
		if (c < 'A' || c > 'Z') && (c < '2' || c > '7') {
			t.Errorf("invalid base32 character: %c", c)
		}
	}
}

func TestComputeAndValidateTOTP(t *testing.T) {
	secret := "JBSWY3DPEHPK3PXP" // Known test secret
	now := time.Now()

	code, err := computeTOTP(secret, now)
	if err != nil {
		t.Fatalf("computeTOTP() error: %v", err)
	}
	if len(code) != 6 {
		t.Errorf("expected 6-digit code, got %d digits: %s", len(code), code)
	}

	// Same code should validate
	if !validateTOTP(secret, code) {
		t.Errorf("valid code %s should pass validation", code)
	}

	// Wrong code should fail
	if validateTOTP(secret, "000000") && code != "000000" {
		t.Error("invalid code should fail validation")
	}

	// Empty code should fail
	if validateTOTP(secret, "") {
		t.Error("empty code should fail validation")
	}

	// Wrong length should fail
	if validateTOTP(secret, "12345") {
		t.Error("5-digit code should fail validation")
	}
}

func TestTOTPProvisioningURI(t *testing.T) {
	uri := totpProvisioningURI("JBSWY3DPEHPK3PXP", "TestBlog", "admin")
	if uri == "" {
		t.Error("URI should not be empty")
	}
	if !contains(uri, "otpauth://totp/") {
		t.Errorf("URI should start with otpauth://totp/, got: %s", uri)
	}
	if !contains(uri, "secret=JBSWY3DPEHPK3PXP") {
		t.Errorf("URI should contain secret, got: %s", uri)
	}
	if !contains(uri, "issuer=TestBlog") {
		t.Errorf("URI should contain issuer, got: %s", uri)
	}
}

func TestSlugify(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Hello World", "hello-world"},
		{"My First Post!", "my-first-post"},
		{"CamelCase", "camelcase"},
		{"with---dashes", "with-dashes"},
		{"  spaces  ", "spaces"},
		{"special@chars#here", "specialcharshere"},
		{"", ""},
		{"123-numbers", "123-numbers"},
	}
	for _, tt := range tests {
		got := slugify(tt.input)
		if got != tt.want {
			t.Errorf("slugify(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestEscapeXML(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"hello", "hello"},
		{"<script>", "&lt;script&gt;"},
		{"a & b", "a &amp; b"},
		{`"quoted"`, "&quot;quoted&quot;"},
		{"", ""},
	}
	for _, tt := range tests {
		got := escapeXML(tt.input)
		if got != tt.want {
			t.Errorf("escapeXML(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestQuoteStrings(t *testing.T) {
	input := []string{"go", "programming", "blog"}
	result := quoteStrings(input)
	if len(result) != 3 {
		t.Fatalf("expected 3 results, got %d", len(result))
	}
	if result[0] != `"go"` {
		t.Errorf("expected %q, got %s", `"go"`, result[0])
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
