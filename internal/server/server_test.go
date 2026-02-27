package server

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestIsAdminPath(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"/admin/", true},
		{"/admin/login", true},
		{"/admin/api/posts", true},
		{"/", false},
		{"/tag/go", false},
		{"/rss.xml", false},
		{"/admi", false},
	}
	for _, tt := range tests {
		got := isAdminPath(tt.path)
		if got != tt.want {
			t.Errorf("isAdminPath(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestExtractIP(t *testing.T) {
	tests := []struct {
		remoteAddr string
		want       string
	}{
		{"192.168.1.1:12345", "192.168.1.1"},
		{"[::1]:8080", "::1"},
		{"127.0.0.1:0", "127.0.0.1"},
		{"invalid", "invalid"}, // no port — returns as-is
	}
	for _, tt := range tests {
		r := &http.Request{RemoteAddr: tt.remoteAddr}
		got := extractIP(r)
		if got != tt.want {
			t.Errorf("extractIP(%q) = %q, want %q", tt.remoteAddr, got, tt.want)
		}
	}
}

func TestRateLimiter(t *testing.T) {
	rl := newRateLimiter()

	// Should allow up to the limit
	for i := 0; i < 5; i++ {
		if !rl.allow("1.2.3.4", 5, time.Minute) {
			t.Errorf("request %d should be allowed", i+1)
		}
	}

	// 6th should be blocked
	if rl.allow("1.2.3.4", 5, time.Minute) {
		t.Error("6th request should be rate limited")
	}

	// Different IP should be independent
	if !rl.allow("5.6.7.8", 5, time.Minute) {
		t.Error("different IP should not be rate limited")
	}
}

func TestStatusRecorder(t *testing.T) {
	w := httptest.NewRecorder()
	sr := &statusRecorder{ResponseWriter: w, status: 200}

	sr.WriteHeader(http.StatusNotFound)
	if sr.status != 404 {
		t.Errorf("status = %d, want 404", sr.status)
	}
}

func TestSecurityHeaders(t *testing.T) {
	s := &Server{}

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := s.securityHeaders(inner)

	// Test public path
	req := httptest.NewRequest("GET", "/", http.NoBody)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	resp := w.Result()
	if resp.Header.Get("X-Content-Type-Options") != "nosniff" {
		t.Error("missing X-Content-Type-Options header")
	}
	if resp.Header.Get("X-Frame-Options") != "DENY" {
		t.Error("missing X-Frame-Options header")
	}
	if resp.Header.Get("Referrer-Policy") != "strict-origin-when-cross-origin" {
		t.Error("missing Referrer-Policy header")
	}
	csp := resp.Header.Get("Content-Security-Policy")
	if csp == "" {
		t.Error("missing Content-Security-Policy header")
	}
	if strings.Contains(csp, "unsafe-eval") {
		t.Error("public CSP should not contain unsafe-eval")
	}

	// Test admin path — should have relaxed CSP
	req2 := httptest.NewRequest("GET", "/admin/settings", http.NoBody)
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req2)

	csp2 := w2.Result().Header.Get("Content-Security-Policy")
	if !strings.Contains(csp2, "unsafe-eval") {
		t.Error("admin CSP should contain unsafe-eval for editor")
	}
}

func TestBodyRecorder(t *testing.T) {
	w := httptest.NewRecorder()
	br := &bodyRecorder{ResponseWriter: w, body: make([]byte, 0), status: 200}

	br.WriteHeader(http.StatusCreated)
	if br.status != 201 {
		t.Errorf("status = %d, want 201", br.status)
	}

	n, err := br.Write([]byte("hello"))
	if err != nil {
		t.Fatalf("Write error: %v", err)
	}
	if n != 5 {
		t.Errorf("Write returned %d, want 5", n)
	}
	if string(br.body) != "hello" {
		t.Errorf("body = %q, want 'hello'", string(br.body))
	}
}

func TestRecoverer_NoPanic(t *testing.T) {
	logger := slog.Default()
	s := &Server{logger: logger}

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	handler := s.recoverer(inner)
	req := httptest.NewRequest("GET", "/", http.NoBody)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	if w.Body.String() != "ok" {
		t.Errorf("body = %q, want 'ok'", w.Body.String())
	}
}

func TestRecoverer_WithPanic(t *testing.T) {
	logger := slog.Default()
	s := &Server{logger: logger}

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	})

	handler := s.recoverer(inner)
	req := httptest.NewRequest("GET", "/test", http.NoBody)
	w := httptest.NewRecorder()

	// Should not panic — the middleware catches it
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", w.Code)
	}
}

func TestRequestLogger(t *testing.T) {
	logger := slog.Default()
	s := &Server{logger: logger}

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	handler := s.requestLogger(inner)
	req := httptest.NewRequest("GET", "/test", http.NoBody)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestRequestLogger_StatusRecording(t *testing.T) {
	logger := slog.Default()
	s := &Server{logger: logger}

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("created"))
	})

	handler := s.requestLogger(inner)
	req := httptest.NewRequest("POST", "/api/posts", http.NoBody)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want 201", w.Code)
	}
}

func TestEtagMiddleware_GeneratesETag(t *testing.T) {
	logger := slog.Default()
	s := &Server{logger: logger}

	body := "<html><body>Hello World</body></html>"
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(body))
	})

	handler := s.etagMiddleware(inner)
	req := httptest.NewRequest("GET", "/hello", http.NoBody)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	etag := w.Header().Get("ETag")
	if etag == "" {
		t.Error("expected ETag header to be set")
	}
	if !strings.HasPrefix(etag, `"`) || !strings.HasSuffix(etag, `"`) {
		t.Errorf("ETag should be quoted, got %q", etag)
	}
	if w.Header().Get("Cache-Control") != "no-cache" {
		t.Errorf("Cache-Control = %q, want 'no-cache'", w.Header().Get("Cache-Control"))
	}
}

func TestEtagMiddleware_Returns304OnMatch(t *testing.T) {
	logger := slog.Default()
	s := &Server{logger: logger}

	body := "<html><body>Consistent Content</body></html>"
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(body))
	})

	handler := s.etagMiddleware(inner)

	// First request to get ETag
	req1 := httptest.NewRequest("GET", "/page", http.NoBody)
	w1 := httptest.NewRecorder()
	handler.ServeHTTP(w1, req1)

	etag := w1.Header().Get("ETag")
	if etag == "" {
		t.Fatal("first request should generate an ETag")
	}

	// Second request with If-None-Match
	req2 := httptest.NewRequest("GET", "/page", http.NoBody)
	req2.Header.Set("If-None-Match", etag)
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req2)

	if w2.Code != http.StatusNotModified {
		t.Errorf("status = %d, want 304", w2.Code)
	}
}

func TestEtagMiddleware_SkipsNonGET(t *testing.T) {
	logger := slog.Default()
	s := &Server{logger: logger}

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("posted"))
	})

	handler := s.etagMiddleware(inner)
	req := httptest.NewRequest("POST", "/api/posts", http.NoBody)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Header().Get("ETag") != "" {
		t.Error("ETag should not be set for POST requests")
	}
}

func TestEtagMiddleware_SkipsAdminPaths(t *testing.T) {
	logger := slog.Default()
	s := &Server{logger: logger}

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("admin content"))
	})

	handler := s.etagMiddleware(inner)
	req := httptest.NewRequest("GET", "/admin/settings", http.NoBody)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Header().Get("ETag") != "" {
		t.Error("ETag should not be set for admin paths")
	}
}

func TestEtagMiddleware_SkipsStaticPaths(t *testing.T) {
	logger := slog.Default()
	s := &Server{logger: logger}

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("static file"))
	})

	handler := s.etagMiddleware(inner)

	skipPaths := []string{"/static/logo.png", "/theme/css/theme.css", "/media/post/image.jpg", "/api/stats", "/healthz", "/metrics"}
	for _, path := range skipPaths {
		req := httptest.NewRequest("GET", path, http.NoBody)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Header().Get("ETag") != "" {
			t.Errorf("ETag should not be set for %s", path)
		}
	}
}

func TestRateLimitMiddleware_AllowsGET(t *testing.T) {
	logger := slog.Default()
	s := &Server{logger: logger}
	rl := newRateLimiter()

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := s.rateLimitMiddleware(inner, rl)

	// GET requests should never be rate limited
	for i := 0; i < 100; i++ {
		req := httptest.NewRequest("GET", "/admin/api/posts", http.NoBody)
		req.RemoteAddr = "1.2.3.4:1234"
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("GET request %d was rate limited", i+1)
		}
	}
}

func TestRateLimitMiddleware_LimitsLogin(t *testing.T) {
	logger := slog.Default()
	s := &Server{logger: logger}
	rl := newRateLimiter()

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := s.rateLimitMiddleware(inner, rl)

	// Login should be limited to 5 per minute
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest("POST", "/admin/api/login", http.NoBody)
		req.RemoteAddr = "10.0.0.1:1234"
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("login request %d should be allowed", i+1)
		}
	}

	// 6th should be rate limited
	req := httptest.NewRequest("POST", "/admin/api/login", http.NoBody)
	req.RemoteAddr = "10.0.0.1:1234"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("6th login request status = %d, want 429", w.Code)
	}
}

func TestRateLimitMiddleware_LimitsComments(t *testing.T) {
	logger := slog.Default()
	s := &Server{logger: logger}
	rl := newRateLimiter()

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := s.rateLimitMiddleware(inner, rl)

	// Comments limited to 3 per minute
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest("POST", "/api/comment/hello-world", http.NoBody)
		req.RemoteAddr = "10.0.0.2:1234"
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("comment request %d should be allowed", i+1)
		}
	}

	// 4th should be limited
	req := httptest.NewRequest("POST", "/api/comment/hello-world", http.NoBody)
	req.RemoteAddr = "10.0.0.2:1234"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("4th comment request status = %d, want 429", w.Code)
	}
}

func TestRateLimitMiddleware_IndependentPerIP(t *testing.T) {
	logger := slog.Default()
	s := &Server{logger: logger}
	rl := newRateLimiter()

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := s.rateLimitMiddleware(inner, rl)

	// Exhaust limit for IP A
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest("POST", "/admin/api/login", http.NoBody)
		req.RemoteAddr = "10.0.0.10:1234"
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
	}

	// IP B should still be allowed
	req := httptest.NewRequest("POST", "/admin/api/login", http.NoBody)
	req.RemoteAddr = "10.0.0.11:1234"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("different IP should not be rate limited, got status %d", w.Code)
	}
}

func TestRateLimitMiddleware_SkipsUnmatchedPOST(t *testing.T) {
	logger := slog.Default()
	s := &Server{logger: logger}
	rl := newRateLimiter()

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := s.rateLimitMiddleware(inner, rl)

	// POST to a path not in the rate-limit switch should pass through
	for i := 0; i < 100; i++ {
		req := httptest.NewRequest("POST", "/some/other/path", http.NoBody)
		req.RemoteAddr = "10.0.0.20:1234"
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("unmatched POST request %d was rate limited", i+1)
		}
	}
}

func TestSecurityHeaders_HSTS(t *testing.T) {
	s := &Server{}

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := s.securityHeaders(inner)
	req := httptest.NewRequest("GET", "/", http.NoBody)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	hsts := w.Result().Header.Get("Strict-Transport-Security")
	if !strings.Contains(hsts, "max-age=") {
		t.Error("missing HSTS max-age directive")
	}
	if !strings.Contains(hsts, "includeSubDomains") {
		t.Error("HSTS should include includeSubDomains")
	}

	pp := w.Result().Header.Get("Permissions-Policy")
	if pp == "" {
		t.Error("missing Permissions-Policy header")
	}
}

func TestRateLimiter_Expiry(t *testing.T) {
	rl := newRateLimiter()

	// Use a very short window
	for i := 0; i < 3; i++ {
		rl.allow("expiry-test", 3, 50*time.Millisecond)
	}

	// Should be blocked now
	if rl.allow("expiry-test", 3, 50*time.Millisecond) {
		t.Error("should be rate limited at the limit")
	}

	// Wait for the window to expire
	time.Sleep(60 * time.Millisecond)

	// Should be allowed again
	if !rl.allow("expiry-test", 3, 50*time.Millisecond) {
		t.Error("should be allowed after window expires")
	}
}

func TestRateLimiter_MaxIPs(t *testing.T) {
	rl := &rateLimiter{
		requests: make(map[string][]time.Time),
		maxIPs:   3,
	}

	// Fill up to the maxIPs cap
	for i := 0; i < 3; i++ {
		ip := strings.Repeat("a", i+1) // "a", "aa", "aaa"
		if !rl.allow(ip, 10, time.Minute) {
			t.Errorf("IP %q should be allowed", ip)
		}
	}

	// New IP should be rejected due to maxIPs cap
	if rl.allow("new-ip", 10, time.Minute) {
		t.Error("new IP should be rejected when maxIPs is reached")
	}

	// Existing IP should still work
	if !rl.allow("a", 10, time.Minute) {
		t.Error("existing IP should still be allowed")
	}
}
