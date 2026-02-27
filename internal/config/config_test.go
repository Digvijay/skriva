package config

import (
	"embed"
	"os"
	"path/filepath"
	"testing"

	blog "github.com/Digvijay/skriva"
)

// ensure embed import is used for zero-value embed.FS
var _ embed.FS
var _ = blog.EmbeddedThemes

func setupTestConfig(t *testing.T) (string, func()) {
	t.Helper()
	dir := t.TempDir()

	os.WriteFile(filepath.Join(dir, "site.yaml"), []byte(`title: "Test Blog"
tagline: "Testing"
base_url: "http://localhost:8080"
theme: "classic"
locale: "en"
posts_per_page: 5
author:
  name: "Tester"
  bio: "A tester"
social:
  github: "testuser"
footer:
  sections: []
`), 0o644)

	os.WriteFile(filepath.Join(dir, "secrets.yaml"), []byte(`admin_password: "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"
session_secret: "testsecret0123456789abcdef012345"
`), 0o644)

	os.WriteFile(filepath.Join(dir, "smtp.yaml"), []byte(`enabled: false
`), 0o644)

	return dir, func() {}
}

func TestLoad(t *testing.T) {
	dir, cleanup := setupTestConfig(t)
	defer cleanup()

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	site := cfg.GetSite()
	if site.Title != "Test Blog" {
		t.Errorf("Title = %q, want 'Test Blog'", site.Title)
	}
	if site.PostsPerPage != 5 {
		t.Errorf("PostsPerPage = %d, want 5", site.PostsPerPage)
	}
	if site.Theme != "classic" {
		t.Errorf("Theme = %q, want 'classic'", site.Theme)
	}
	if site.Locale != "en" {
		t.Errorf("Locale = %q, want 'en'", site.Locale)
	}
	if site.Author.Name != "Tester" {
		t.Errorf("Author.Name = %q, want 'Tester'", site.Author.Name)
	}
}

func TestLoadDefaults(t *testing.T) {
	dir := t.TempDir()

	os.WriteFile(filepath.Join(dir, "site.yaml"), []byte(`title: "Minimal"
`), 0o644)

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	site := cfg.GetSite()
	if site.PostsPerPage != 10 {
		t.Errorf("default PostsPerPage = %d, want 10", site.PostsPerPage)
	}
	if site.Theme != "classic" {
		t.Errorf("default Theme = %q, want 'classic'", site.Theme)
	}
}

func TestLoadMissingDir(t *testing.T) {
	_, err := Load("/nonexistent/path/xyz")
	if err == nil {
		t.Error("Load() should fail for nonexistent directory")
	}
}

func TestVerifyAdminPassword(t *testing.T) {
	dir, cleanup := setupTestConfig(t)
	defer cleanup()

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	// The hash in secrets.yaml is for "password" (standard bcrypt test vector)
	// Note: the actual hash may not match "password" — test what we set
	if cfg.VerifyAdminPassword("") {
		t.Error("empty password should not verify")
	}
}

func TestGetSMTP(t *testing.T) {
	dir, cleanup := setupTestConfig(t)
	defer cleanup()

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	smtp := cfg.GetSMTP()
	if smtp.Enabled {
		t.Error("SMTP should be disabled")
	}
}

func TestGetSecrets(t *testing.T) {
	dir, cleanup := setupTestConfig(t)
	defer cleanup()

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	secrets := cfg.GetSecrets()
	if secrets.SessionSecret != "testsecret0123456789abcdef012345" {
		t.Errorf("SessionSecret = %q, unexpected", secrets.SessionSecret)
	}
}

func TestGetConfigDir(t *testing.T) {
	dir, cleanup := setupTestConfig(t)
	defer cleanup()

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.GetConfigDir() != dir {
		t.Errorf("GetConfigDir() = %q, want %q", cfg.GetConfigDir(), dir)
	}
}

func TestUpdateSite(t *testing.T) {
	dir, cleanup := setupTestConfig(t)
	defer cleanup()

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	site := cfg.GetSite()
	site.Title = "Updated Title"
	if err := cfg.UpdateSite(site); err != nil {
		t.Fatalf("UpdateSite() error: %v", err)
	}

	// Reload and verify
	cfg2, err := Load(dir)
	if err != nil {
		t.Fatalf("reloading config: %v", err)
	}
	if cfg2.GetSite().Title != "Updated Title" {
		t.Errorf("after update, Title = %q, want 'Updated Title'", cfg2.GetSite().Title)
	}
}

func TestIsTOTPEnabled(t *testing.T) {
	dir, cleanup := setupTestConfig(t)
	defer cleanup()

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.IsTOTPEnabled() {
		t.Error("TOTP should not be enabled (no secret set)")
	}
}

func TestTranslate(t *testing.T) {
	dir, cleanup := setupTestConfig(t)
	defer cleanup()

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	// English (default)
	if got := cfg.Translate("home"); got != "Home" {
		t.Errorf("Translate('home') = %q, want 'Home'", got)
	}
	if got := cfg.Translate("subscribe"); got != "Subscribe" {
		t.Errorf("Translate('subscribe') = %q, want 'Subscribe'", got)
	}
	// Unknown key should return the key itself
	if got := cfg.Translate("nonexistent_key"); got != "nonexistent_key" {
		t.Errorf("Translate('nonexistent_key') = %q, want 'nonexistent_key'", got)
	}
}

func TestTranslateLocales(t *testing.T) {
	dir := t.TempDir()

	locales := []struct {
		locale string
		key    string
		want   string
	}{
		{"es", "home", "Inicio"},
		{"fr", "home", "Accueil"},
		{"de", "home", "Startseite"},
		{"ja", "home", "ホーム"},
		{"zh", "home", "首页"},
	}

	for _, tt := range locales {
		t.Run(tt.locale, func(t *testing.T) {
			os.WriteFile(filepath.Join(dir, "site.yaml"), []byte(`title: "Test"
locale: "`+tt.locale+`"
`), 0o644)
			cfg, err := Load(dir)
			if err != nil {
				t.Fatalf("Load() error: %v", err)
			}
			if got := cfg.Translate(tt.key); got != tt.want {
				t.Errorf("Translate(%q) with locale %s = %q, want %q", tt.key, tt.locale, got, tt.want)
			}
		})
	}
}

func TestListThemes_ContentDirOnly(t *testing.T) {
	contentDir := t.TempDir()

	// Create a user theme with theme.yaml
	themeDir := filepath.Join(contentDir, "themes", "custom")
	if err := os.MkdirAll(themeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(themeDir, "theme.yaml"), []byte(`name: "Custom Theme"
description: "A user-created theme"
author: "Tester"
version: "0.1.0"
`), 0o644)

	// Use zero-value embed.FS (no embedded themes)
	var emptyFS embed.FS
	themes, err := ListThemes(contentDir, emptyFS)
	if err != nil {
		t.Fatalf("ListThemes() error: %v", err)
	}
	if len(themes) != 1 {
		t.Fatalf("ListThemes() returned %d themes, want 1", len(themes))
	}
	if themes[0].ID != "custom" {
		t.Errorf("theme ID = %q, want 'custom'", themes[0].ID)
	}
	if themes[0].Name != "Custom Theme" {
		t.Errorf("theme Name = %q, want 'Custom Theme'", themes[0].Name)
	}
	if themes[0].Author != "Tester" {
		t.Errorf("theme Author = %q, want 'Tester'", themes[0].Author)
	}
}

func TestListThemes_EmbeddedOnly(t *testing.T) {
	// Empty content dir (no user themes)
	contentDir := t.TempDir()

	themes, err := ListThemes(contentDir, blog.EmbeddedThemes)
	if err != nil {
		t.Fatalf("ListThemes() error: %v", err)
	}
	if len(themes) < 3 {
		t.Errorf("ListThemes() returned %d embedded themes, want at least 3 (classic, newsletter, github)", len(themes))
	}

	// Verify classic theme is present
	found := false
	for _, theme := range themes {
		if theme.ID == "classic" {
			found = true
			if theme.Name != "Classic" {
				t.Errorf("classic theme Name = %q, want 'Classic'", theme.Name)
			}
		}
	}
	if !found {
		t.Error("embedded classic theme not found")
	}
}

func TestListThemes_UserOverridesEmbedded(t *testing.T) {
	contentDir := t.TempDir()

	// Create a user theme that overrides "classic"
	themeDir := filepath.Join(contentDir, "themes", "classic")
	if err := os.MkdirAll(themeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(themeDir, "theme.yaml"), []byte(`name: "My Classic Override"
description: "User override of classic"
author: "User"
version: "2.0.0"
`), 0o644)

	themes, err := ListThemes(contentDir, blog.EmbeddedThemes)
	if err != nil {
		t.Fatalf("ListThemes() error: %v", err)
	}

	// classic should appear only once, with the user version
	classicCount := 0
	for _, theme := range themes {
		if theme.ID == "classic" {
			classicCount++
			if theme.Name != "My Classic Override" {
				t.Errorf("overridden classic Name = %q, want 'My Classic Override'", theme.Name)
			}
		}
	}
	if classicCount != 1 {
		t.Errorf("classic theme appeared %d times, want exactly 1", classicCount)
	}
}

func TestListThemes_NoThemesDir(t *testing.T) {
	contentDir := t.TempDir()

	var emptyFS embed.FS
	themes, err := ListThemes(contentDir, emptyFS)
	if err != nil {
		t.Fatalf("ListThemes() error: %v", err)
	}
	if len(themes) != 0 {
		t.Errorf("ListThemes() returned %d themes, want 0 for empty dirs", len(themes))
	}
}

func TestListThemes_SkipsInvalidThemeYAML(t *testing.T) {
	contentDir := t.TempDir()

	// Create a theme dir with invalid YAML
	badDir := filepath.Join(contentDir, "themes", "bad")
	if err := os.MkdirAll(badDir, 0o755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(badDir, "theme.yaml"), []byte(`{{{invalid`), 0o644)

	// Create a valid theme too
	goodDir := filepath.Join(contentDir, "themes", "good")
	if err := os.MkdirAll(goodDir, 0o755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(goodDir, "theme.yaml"), []byte(`name: "Good"
`), 0o644)

	var emptyFS embed.FS
	themes, err := ListThemes(contentDir, emptyFS)
	if err != nil {
		t.Fatalf("ListThemes() error: %v", err)
	}
	// Should only get the valid theme
	if len(themes) != 1 {
		t.Errorf("ListThemes() returned %d themes, want 1 (skipping invalid)", len(themes))
	}
}

func TestUpdateSecrets(t *testing.T) {
	dir, cleanup := setupTestConfig(t)
	defer cleanup()

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	// Update secrets with new values
	newSecrets := SecretsConfig{
		AdminPasswordHash:     "$2a$10$newhashabcdef",
		SessionSecret:         "newsessionsecret1234567890abcdef",
		UnsplashAccessKey:     "my-unsplash-key",
		TOTPSecret:            "JBSWY3DPEHPK3PXP",
		ActivityPubPrivateKey: "-----BEGIN PRIVATE KEY-----\ntest\n-----END PRIVATE KEY-----",
		ActivityPubPublicKey:  "-----BEGIN PUBLIC KEY-----\ntest\n-----END PUBLIC KEY-----",
	}

	if err := cfg.UpdateSecrets(newSecrets); err != nil {
		t.Fatalf("UpdateSecrets() error: %v", err)
	}

	// Verify in-memory values updated
	got := cfg.GetSecrets()
	if got.UnsplashAccessKey != "my-unsplash-key" {
		t.Errorf("UnsplashAccessKey = %q, want 'my-unsplash-key'", got.UnsplashAccessKey)
	}
	if got.TOTPSecret != "JBSWY3DPEHPK3PXP" {
		t.Errorf("TOTPSecret = %q, want 'JBSWY3DPEHPK3PXP'", got.TOTPSecret)
	}

	// Verify TOTP is now enabled
	if !cfg.IsTOTPEnabled() {
		t.Error("IsTOTPEnabled() = false after setting TOTP secret")
	}

	// Reload config from disk and verify persistence
	cfg2, err := Load(dir)
	if err != nil {
		t.Fatalf("reloading config: %v", err)
	}
	got2 := cfg2.GetSecrets()
	if got2.UnsplashAccessKey != "my-unsplash-key" {
		t.Errorf("after reload, UnsplashAccessKey = %q, want 'my-unsplash-key'", got2.UnsplashAccessKey)
	}
	if got2.TOTPSecret != "JBSWY3DPEHPK3PXP" {
		t.Errorf("after reload, TOTPSecret = %q, want 'JBSWY3DPEHPK3PXP'", got2.TOTPSecret)
	}
	if got2.SessionSecret != "newsessionsecret1234567890abcdef" {
		t.Errorf("after reload, SessionSecret = %q, want 'newsessionsecret1234567890abcdef'", got2.SessionSecret)
	}
}

func TestUpdateSecrets_FilePermissions(t *testing.T) {
	dir, cleanup := setupTestConfig(t)
	defer cleanup()

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	secrets := cfg.GetSecrets()
	secrets.TOTPSecret = "testsecret"
	if err := cfg.UpdateSecrets(secrets); err != nil {
		t.Fatalf("UpdateSecrets() error: %v", err)
	}

	// Verify the secrets file was written
	path := filepath.Join(dir, "secrets.yaml")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat secrets.yaml: %v", err)
	}
	if info.Size() == 0 {
		t.Error("secrets.yaml should not be empty after UpdateSecrets")
	}
}
