// helpers.go — shared utility functions: rendering, security, email, image processing.
package handler

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"net"
	"net/http"
	"net/smtp"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/Digvijay/skriva/internal/config"
	"github.com/Digvijay/skriva/internal/render"
	"golang.org/x/crypto/bcrypt"
)

func (h *BlogHandler) renderTemplate(w http.ResponseWriter, name string, data *render.TemplateData) {
	// Always inject a CSRF token for forms
	data.CSRFToken = h.generateCSRFToken()

	// Always inject Fediverse address for footer widget
	// Respect the ActivityPubEnabled toggle via the fediAddress() helper
	if data.FediAddress == "" {
		data.FediAddress = h.fediAddress()
	}

	result, err := h.engine.RenderPage(name, data)
	if err != nil {
		h.logger.Error("rendering template", "template", name, "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, result)
}

func (h *BlogHandler) render404(w http.ResponseWriter, r *http.Request) {
	site := h.cfg.GetSite()
	data := &render.TemplateData{
		Site:  site,
		Nav:   render.NavData{Path: r.URL.Path},
		Meta:  render.SEOMeta{Title: "Not Found | " + site.Title},
		Error: "The page you're looking for doesn't exist.",
	}

	w.WriteHeader(http.StatusNotFound)
	h.renderTemplate(w, "404.html", data)
}

// Render500 renders a themed 500 error page. Falls back to plain text if template fails.
func (h *BlogHandler) Render500(w http.ResponseWriter, r *http.Request) {
	site := h.cfg.GetSite()
	data := &render.TemplateData{
		Site:  site,
		Nav:   render.NavData{Path: r.URL.Path},
		Meta:  render.SEOMeta{Title: "Server Error | " + site.Title},
		Error: "Something went wrong. Please try again later.",
	}
	data.CSRFToken = h.generateCSRFToken()

	result, err := h.engine.RenderPage("404.html", data)
	if err != nil {
		// Fallback to plain text if theme rendering also fails
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusInternalServerError)
	fmt.Fprint(w, result)
}

// sendEmail sends an email using the configured SMTP settings.
func (h *BlogHandler) sendEmail(smtpCfg config.SMTPConfig, to, subject, body string) error {
	from := smtpCfg.From
	if from == "" {
		from = smtpCfg.Username
	}

	// SECURITY: Sanitize email fields to prevent header injection via CRLF
	to = sanitizeEmailHeader(to)
	subject = sanitizeEmailHeader(subject)
	from = sanitizeEmailHeader(from)

	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n%s",
		from, to, subject, body)

	addr := fmt.Sprintf("%s:%d", smtpCfg.Host, smtpCfg.Port)
	auth := smtp.PlainAuth("", smtpCfg.Username, smtpCfg.Password, smtpCfg.Host)

	if err := smtp.SendMail(addr, auth, from, []string{to}, []byte(msg)); err != nil {
		// SECURITY: Don't include email address in error message (PII/GDPR)
		return fmt.Errorf("sending email via SMTP: %w", err)
	}
	return nil
}

func generateFingerprint(r *http.Request) string {
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	ua := r.UserAgent()
	h := sha256.Sum256([]byte(ip + "|" + ua))
	return hex.EncodeToString(h[:16])
}

func jsonError(w http.ResponseWriter, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}

// optimizeImage resizes JPEG/PNG images if wider than maxWidth, preserving aspect ratio.
func optimizeImage(filePath, contentType string, maxWidth int) {
	// Recover from any panic to prevent crashing the server
	defer func() {
		recover() //nolint:errcheck // best-effort image optimization
	}()

	if contentType != "image/jpeg" && contentType != "image/png" {
		return
	}

	f, err := os.Open(filePath)
	if err != nil {
		return
	}

	// Check dimensions first without full decode to prevent decompression bombs
	imgCfg, _, err := image.DecodeConfig(f)
	f.Close()
	if err != nil {
		return
	}

	// Reject images with unreasonable dimensions (max 10000x10000 = 400MB decoded)
	const maxDimension = 10000
	if imgCfg.Width > maxDimension || imgCfg.Height > maxDimension {
		return
	}

	if imgCfg.Width <= maxWidth {
		return // already small enough
	}

	// Re-open and fully decode now that we know dimensions are safe
	f2, err := os.Open(filePath)
	if err != nil {
		return
	}

	img, _, err := image.Decode(f2)
	f2.Close()
	if err != nil {
		return
	}

	bounds := img.Bounds()
	origW := bounds.Dx()
	origH := bounds.Dy()

	// Simple nearest-neighbor resize (no external deps)
	newW := maxWidth
	newH := origH * maxWidth / origW
	resized := image.NewRGBA(image.Rect(0, 0, newW, newH))
	for y := 0; y < newH; y++ {
		for x := 0; x < newW; x++ {
			srcX := x * origW / newW
			srcY := y * origH / newH
			resized.Set(x, y, img.At(srcX, srcY))
		}
	}

	out, err := os.Create(filePath)
	if err != nil {
		return
	}
	defer out.Close()

	switch contentType {
	case "image/jpeg":
		jpeg.Encode(out, resized, &jpeg.Options{Quality: 85})
	case "image/png":
		png.Encode(out, resized)
	}

	// Best-effort WebP conversion: try exec cwebp if available
	go generateWebP(filePath)
}

// generateWebP creates a .webp version of an image file using cwebp if available.
func generateWebP(filePath string) {
	defer func() { _ = recover() }() // best-effort, never crash
	ext := filepath.Ext(filePath)
	if ext != ".jpg" && ext != ".jpeg" && ext != ".png" {
		return
	}
	webpPath := filePath + ".webp"
	// Check if cwebp is available
	if _, err := exec.LookPath("cwebp"); err != nil {
		return
	}
	_ = exec.Command("cwebp", "-q", "80", filePath, "-o", webpPath).Run()
}

func escapeXML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	return s
}

func slugify(s string) string {
	s = strings.ToLower(s)
	s = strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			return r
		}
		if r == ' ' || r == '-' || r == '_' {
			return '-'
		}
		return -1
	}, s)
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	return strings.Trim(s, "-")
}

func quoteStrings(ss []string) []string {
	quoted := make([]string, len(ss))
	for i, s := range ss {
		quoted[i] = fmt.Sprintf("%q", s)
	}
	return quoted
}

// HashPassword creates a bcrypt hash for initial setup.
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hashing password: %w", err)
	}
	return string(hash), nil
}

// newZipWriter creates a zip.Writer that writes to an http.ResponseWriter.
func newZipWriter(w http.ResponseWriter) *zip.Writer {
	return zip.NewWriter(w)
}

// validateExternalURL checks that a URL is safe to fetch (not internal/private IPs).
// Prevents SSRF attacks via ActivityPub, Webmention, and other outbound requests.
// safeHTTPClient returns an http.Client with SSRF-safe redirect following.
// Every redirect target is validated against private/internal IP ranges.
func safeHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects")
			}
			if err := validateExternalURL(req.URL.String()); err != nil {
				return fmt.Errorf("redirect to blocked URL: %w", err)
			}
			return nil
		},
	}
}

func validateExternalURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("only http/https allowed, got %s", u.Scheme)
	}
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("empty hostname")
	}

	// Block known internal/private hostnames
	blocked := []string{"localhost", "127.0.0.1", "0.0.0.0", "[::1]", "::1"}
	for _, b := range blocked {
		if strings.EqualFold(host, b) {
			return fmt.Errorf("internal hostname blocked: %s", host)
		}
	}

	// Block private IP ranges (works for IP literals)
	ip := net.ParseIP(host)
	if ip != nil {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || isCloudMetadata(ip) {
			return fmt.Errorf("private/internal IP blocked: %s", host)
		}
	}

	// Block common cloud metadata IPs
	if host == "169.254.169.254" || host == "metadata.google.internal" {
		return fmt.Errorf("cloud metadata endpoint blocked: %s", host)
	}

	// SECURITY: Resolve hostname and check ALL resolved IPs against private ranges.
	// This prevents DNS rebinding / SSRF via domain names pointing to internal IPs.
	if ip == nil {
		addrs, err := net.LookupHost(host)
		if err != nil {
			return fmt.Errorf("DNS resolution failed for %s: %w", host, err)
		}
		for _, addr := range addrs {
			resolved := net.ParseIP(addr)
			if resolved == nil {
				continue
			}
			if resolved.IsLoopback() || resolved.IsPrivate() || resolved.IsLinkLocalUnicast() || resolved.IsLinkLocalMulticast() || isCloudMetadata(resolved) || resolved.IsUnspecified() {
				return fmt.Errorf("hostname %s resolves to private/internal IP %s", host, addr)
			}
		}
	}

	return nil
}

// isCloudMetadata checks if an IP is a known cloud metadata endpoint.
func isCloudMetadata(ip net.IP) bool {
	metadata := net.ParseIP("169.254.169.254")
	return ip.Equal(metadata)
}

// sanitizeEmailHeader removes CRLF sequences from email header values
// to prevent email header injection attacks.
func sanitizeEmailHeader(s string) string {
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, "\n", "")
	return s
}

// sanitizeUntrustedHTML strips dangerous HTML elements and attributes from untrusted content.
// Allows basic formatting tags but removes <script>, <iframe>, event handlers, etc.
// Used for: ActivityPub replies, webmention content, Micropub-created posts.
//
// SECURITY: Runs multiple passes to defeat nested-tag reconstruction attacks
// (e.g., <scr<script>x</script>ipt> → <script> after one pass).
func sanitizeUntrustedHTML(input string) string {
	if input == "" {
		return ""
	}

	// Run in a loop until output stabilizes (max 5 passes to prevent infinite loops)
	for pass := 0; pass < 5; pass++ {
		prev := input

		// Strip <script>...</script> including content
		reScript := regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`)
		input = reScript.ReplaceAllString(input, "")

		// Strip <style>...</style> including content
		reStyle := regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`)
		input = reStyle.ReplaceAllString(input, "")

		// Strip <iframe>...</iframe> including content
		reIframe := regexp.MustCompile(`(?is)<iframe[^>]*>.*?</iframe>`)
		input = reIframe.ReplaceAllString(input, "")

		// Strip <object>, <embed>, <applet>, <form>, <input>, <button>, <select>, <textarea>
		for _, tag := range []string{"object", "embed", "applet", "form", "input", "button", "select", "textarea", "link", "meta", "base"} {
			// Strip both self-closing and paired tags
			re1 := regexp.MustCompile(`(?is)<` + tag + `[^>]*/>`)
			input = re1.ReplaceAllString(input, "")
			re2 := regexp.MustCompile(`(?is)<` + tag + `[^>]*>.*?</` + tag + `>`)
			input = re2.ReplaceAllString(input, "")
			// Strip any remaining opening tags
			re3 := regexp.MustCompile(`(?is)<` + tag + `[^>]*>`)
			input = re3.ReplaceAllString(input, "")
		}

		// SECURITY: Strip orphan dangerous tags that may not be properly closed
		// (e.g., "<sCript" at end of input, or "<script " without closing ">")
		for _, tag := range []string{"script", "style", "iframe", "object", "embed", "applet"} {
			reOrphan := regexp.MustCompile(`(?i)<\s*` + tag + `[^>]*$`)
			input = reOrphan.ReplaceAllString(input, "")
			// Also strip any remaining opening/closing tags for these dangerous elements
			reOpen := regexp.MustCompile(`(?i)<\s*` + tag + `[^>]*>`)
			input = reOpen.ReplaceAllString(input, "")
			reClose := regexp.MustCompile(`(?i)</\s*` + tag + `\s*>`)
			input = reClose.ReplaceAllString(input, "")
		}

		// Strip all event handler attributes (on*) — supports quoted, unquoted, and tab/space around =
		reEvent := regexp.MustCompile(`(?i)\s+on\w+\s*=\s*(?:"[^"]*"|'[^']*'|[^\s>]*)`)
		input = reEvent.ReplaceAllString(input, "")

		// Strip javascript:/data:/vbscript: URLs in href/src/action — now also matches UNQUOTED values
		reJSURL := regexp.MustCompile(`(?i)(href|src|action)\s*=\s*(?:"(?:javascript|data|vbscript):[^"]*"|'(?:javascript|data|vbscript):[^']*'|(?:javascript|data|vbscript):[^\s>]*)`)
		input = reJSURL.ReplaceAllString(input, "")

		// If nothing changed this pass, we've stabilized
		if input == prev {
			break
		}
	}

	return strings.TrimSpace(input)
}

// hasScope checks if a space-separated scope string contains a specific scope.
// Uses exact word matching to prevent substring attacks (e.g., "nocreate" matching "create").
func hasScope(scopeStr, target string) bool {
	for _, s := range strings.Fields(scopeStr) {
		if s == target {
			return true
		}
	}
	return false
}
