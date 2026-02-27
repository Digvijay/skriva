// helpers_test.go — unit tests for unexported helper functions in the handler package.
package handler

import (
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

// --- escapeXML ---

func TestEscapeXML_Extended(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"empty", "", ""},
		{"no special chars", "hello world", "hello world"},
		{"ampersand", "a&b", "a&amp;b"},
		{"less than", "a<b", "a&lt;b"},
		{"greater than", "a>b", "a&gt;b"},
		{"double quote", `a"b`, "a&quot;b"},
		{"all entities", `<foo & "bar">`, "&lt;foo &amp; &quot;bar&quot;&gt;"},
		{"double ampersand", "a&&b", "a&amp;&amp;b"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := escapeXML(tt.input)
			if got != tt.want {
				t.Errorf("escapeXML(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// --- slugify ---

func TestSlugify_Extended(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"simple", "Hello World", "hello-world"},
		{"with special chars", "My Post! #1", "my-post-1"},
		{"underscores", "hello_world", "hello-world"},
		{"multiple dashes", "hello---world", "hello-world"},
		{"leading trailing dashes", "-hello-world-", "hello-world"},
		{"numbers", "2025 in review", "2025-in-review"},
		{"unicode stripped", "café latte", "caf-latte"},
		{"empty", "", ""},
		{"only special chars", "!@#$%", ""},
		{"mixed case", "Go Series Part 1", "go-series-part-1"},
		{"spaces and hyphens", "hello  -  world", "hello-world"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := slugify(tt.input)
			if got != tt.want {
				t.Errorf("slugify(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// --- quoteStrings ---

func TestQuoteStrings_Extended(t *testing.T) {
	tests := []struct {
		name  string
		input []string
		want  []string
	}{
		{"empty slice", []string{}, []string{}},
		{"single", []string{"go"}, []string{`"go"`}},
		{"multiple", []string{"go", "blogging"}, []string{`"go"`, `"blogging"`}},
		{"with special chars", []string{`a"b`}, []string{`"a\"b"`}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := quoteStrings(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("quoteStrings(%v) length = %d, want %d", tt.input, len(got), len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("quoteStrings(%v)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
				}
			}
		})
	}
}

// --- HashPassword ---

func TestHashPassword(t *testing.T) {
	password := "securepassword123"

	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword() error: %v", err)
	}

	if hash == "" {
		t.Fatal("HashPassword() returned empty hash")
	}

	// Verify the hash matches the original password
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		t.Errorf("bcrypt.CompareHashAndPassword() failed: hash should match original password")
	}

	// Verify the hash does NOT match a different password
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte("wrongpassword")); err == nil {
		t.Error("bcrypt.CompareHashAndPassword() should fail for wrong password")
	}

	// Two hashes of the same password should be different (bcrypt uses random salt)
	hash2, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword() second call error: %v", err)
	}
	if hash == hash2 {
		t.Error("HashPassword() should produce different hashes for same password (random salt)")
	}
}

// --- sanitizeEmailHeader ---

func TestSanitizeEmailHeader(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"clean string", "hello@example.com", "hello@example.com"},
		{"with CR", "hello\r@example.com", "hello@example.com"},
		{"with LF", "hello\n@example.com", "hello@example.com"},
		{"with CRLF", "hello\r\n@example.com", "hello@example.com"},
		{"multiple CRLF", "a\r\nb\r\nc", "abc"},
		{"injection attempt", "test@example.com\r\nBcc: attacker@evil.com", "test@example.comBcc: attacker@evil.com"},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeEmailHeader(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeEmailHeader(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// --- hasScope ---

func TestHasScope(t *testing.T) {
	tests := []struct {
		name      string
		scopeStr  string
		target    string
		wantMatch bool
	}{
		{"exact match", "read write admin", "write", true},
		{"first scope", "read write admin", "read", true},
		{"last scope", "read write admin", "admin", true},
		{"no match", "read write admin", "delete", false},
		{"substring should not match", "nocreate readonly", "create", false},
		{"empty scope string", "", "read", false},
		{"empty target", "read write", "", false},
		{"single scope match", "write", "write", true},
		{"single scope no match", "write", "read", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasScope(tt.scopeStr, tt.target)
			if got != tt.wantMatch {
				t.Errorf("hasScope(%q, %q) = %v, want %v", tt.scopeStr, tt.target, got, tt.wantMatch)
			}
		})
	}
}

// --- isCloudMetadata ---

func TestIsCloudMetadata(t *testing.T) {
	tests := []struct {
		name string
		ip   string
		want bool
	}{
		{"metadata IP", "169.254.169.254", true},
		{"localhost", "127.0.0.1", false},
		{"private IP", "10.0.0.1", false},
		{"public IP", "8.8.8.8", false},
		{"IPv6 loopback", "::1", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip := net.ParseIP(tt.ip)
			if ip == nil {
				t.Fatalf("failed to parse IP: %s", tt.ip)
			}
			got := isCloudMetadata(ip)
			if got != tt.want {
				t.Errorf("isCloudMetadata(%s) = %v, want %v", tt.ip, got, tt.want)
			}
		})
	}
}

// --- validateExternalURL ---

func TestValidateExternalURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{"valid https", "https://example.com/page", false},
		{"valid http", "http://example.com/page", false},
		{"localhost blocked", "http://localhost/admin", true},
		{"127.0.0.1 blocked", "http://127.0.0.1:8080/", true},
		{"0.0.0.0 blocked", "http://0.0.0.0/", true},
		{"::1 blocked", "http://[::1]/", true},
		{"metadata IP blocked", "http://169.254.169.254/latest/meta-data/", true},
		{"metadata hostname blocked", "http://metadata.google.internal/", true},
		{"ftp scheme blocked", "ftp://example.com/file", true},
		{"javascript scheme blocked", "javascript:alert(1)", true},
		{"empty hostname", "http:///path", true},
		{"invalid URL", "://invalid", true},
		{"private IP 10.x", "http://10.0.0.1/", true},
		{"private IP 192.168.x", "http://192.168.1.1/", true},
		{"private IP 172.16.x", "http://172.16.0.1/", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateExternalURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateExternalURL(%q) error = %v, wantErr %v", tt.url, err, tt.wantErr)
			}
		})
	}
}

// --- safeHTTPClient ---

func TestSafeHTTPClient(t *testing.T) {
	client := safeHTTPClient()
	if client == nil {
		t.Fatal("safeHTTPClient() returned nil")
	}
	if client.Timeout == 0 {
		t.Error("safeHTTPClient() should have a non-zero timeout")
	}
	if client.CheckRedirect == nil {
		t.Error("safeHTTPClient() should have a redirect checker")
	}
}

// --- jsonError ---

func TestJsonError(t *testing.T) {
	tests := []struct {
		name       string
		message    string
		status     int
		wantStatus int
	}{
		{"bad request", "Invalid input", http.StatusBadRequest, http.StatusBadRequest},
		{"unauthorized", "Not authorized", http.StatusUnauthorized, http.StatusUnauthorized},
		{"internal error", "Server error", http.StatusInternalServerError, http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			jsonError(w, tt.message, tt.status)

			resp := w.Result()
			if resp.StatusCode != tt.wantStatus {
				t.Errorf("jsonError() status = %d, want %d", resp.StatusCode, tt.wantStatus)
			}

			ct := resp.Header.Get("Content-Type")
			if ct != "application/json" {
				t.Errorf("jsonError() Content-Type = %q, want application/json", ct)
			}

			var body map[string]string
			if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
				t.Fatalf("failed to decode JSON response: %v", err)
			}
			if body["error"] != tt.message {
				t.Errorf("jsonError() body[error] = %q, want %q", body["error"], tt.message)
			}
		})
	}
}

// --- generateFingerprint ---

func TestGenerateFingerprint(t *testing.T) {
	req1 := httptest.NewRequest("GET", "/", http.NoBody)
	req1.RemoteAddr = "192.168.1.1:12345"
	req1.Header.Set("User-Agent", "TestBrowser/1.0")

	fp1 := generateFingerprint(req1)
	if fp1 == "" {
		t.Fatal("generateFingerprint() returned empty string")
	}
	if len(fp1) != 32 { // hex-encoded 16 bytes = 32 chars
		t.Errorf("generateFingerprint() length = %d, want 32", len(fp1))
	}

	// Same request should produce same fingerprint
	req2 := httptest.NewRequest("GET", "/", http.NoBody)
	req2.RemoteAddr = "192.168.1.1:12345"
	req2.Header.Set("User-Agent", "TestBrowser/1.0")

	fp2 := generateFingerprint(req2)
	if fp1 != fp2 {
		t.Errorf("same IP+UA should produce same fingerprint: %q != %q", fp1, fp2)
	}

	// Different IP should produce different fingerprint
	req3 := httptest.NewRequest("GET", "/", http.NoBody)
	req3.RemoteAddr = "10.0.0.1:12345"
	req3.Header.Set("User-Agent", "TestBrowser/1.0")

	fp3 := generateFingerprint(req3)
	if fp1 == fp3 {
		t.Error("different IPs should produce different fingerprints")
	}

	// Different User-Agent should produce different fingerprint
	req4 := httptest.NewRequest("GET", "/", http.NoBody)
	req4.RemoteAddr = "192.168.1.1:12345"
	req4.Header.Set("User-Agent", "DifferentBrowser/2.0")

	fp4 := generateFingerprint(req4)
	if fp1 == fp4 {
		t.Error("different User-Agents should produce different fingerprints")
	}
}

// --- optimizeImage ---

func TestOptimizeImage_SmallImage(t *testing.T) {
	// Create a small test PNG (10x10 pixels) — below any reasonable maxWidth threshold
	tmpDir := t.TempDir()
	imgPath := filepath.Join(tmpDir, "small.png")

	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	for y := 0; y < 10; y++ {
		for x := 0; x < 10; x++ {
			img.Set(x, y, color.RGBA{R: 255, G: 0, B: 0, A: 255})
		}
	}

	f, err := os.Create(imgPath)
	if err != nil {
		t.Fatalf("creating test image: %v", err)
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		t.Fatalf("encoding test PNG: %v", err)
	}
	f.Close()

	// Get file info before optimization
	infoBefore, err := os.Stat(imgPath)
	if err != nil {
		t.Fatalf("stat before: %v", err)
	}

	// Call optimizeImage with maxWidth=1200 — image is 10px wide, should be a no-op
	optimizeImage(imgPath, "image/png", 1200)

	// Verify the file still exists and wasn't corrupted
	infoAfter, err := os.Stat(imgPath)
	if err != nil {
		t.Fatalf("stat after: %v", err)
	}

	// Size should be unchanged since image is below threshold
	if infoBefore.Size() != infoAfter.Size() {
		t.Errorf("small image should not be modified: size before=%d, after=%d", infoBefore.Size(), infoAfter.Size())
	}
}

func TestOptimizeImage_UnsupportedType(t *testing.T) {
	tmpDir := t.TempDir()
	txtPath := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(txtPath, []byte("not an image"), 0o644)

	// Should return without error or panic for unsupported content types
	optimizeImage(txtPath, "text/plain", 1200)

	// File should be unchanged
	data, err := os.ReadFile(txtPath)
	if err != nil {
		t.Fatalf("reading file: %v", err)
	}
	if string(data) != "not an image" {
		t.Error("non-image file should not be modified")
	}
}
