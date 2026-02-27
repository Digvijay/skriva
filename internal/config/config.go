// Package config handles loading and validating YAML configuration files
// from the config directory. It supports hot-reload via fsnotify.
package config

import (
	"crypto/rand"
	"embed"
	"encoding/hex"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/fsnotify/fsnotify"
	"golang.org/x/crypto/bcrypt"
	"gopkg.in/yaml.v3"
)

// Config holds all configuration loaded from the config directory.
type Config struct {
	Site    SiteConfig    `yaml:"-"`
	Secrets SecretsConfig `yaml:"-"`
	SMTP    SMTPConfig    `yaml:"-"`

	mu     sync.RWMutex
	dir    string
	logger *slog.Logger
}

// SiteConfig holds blog site settings from site.yaml.
type SiteConfig struct {
	Title        string       `yaml:"title" json:"title"`
	Tagline      string       `yaml:"tagline" json:"tagline"`
	BaseURL      string       `yaml:"base_url" json:"base_url"`
	Theme        string       `yaml:"theme" json:"theme"`
	Locale       string       `yaml:"locale" json:"locale"`
	PostsPerPage int          `yaml:"posts_per_page" json:"posts_per_page"`
	Author       AuthorConfig `yaml:"author" json:"author"`
	Social       SocialConfig `yaml:"social" json:"social"`
	Footer       FooterConfig `yaml:"footer" json:"footer"`
	// IndieWeb / Fediverse feature toggles (all default to true for backward compat)
	ActivityPubEnabled bool `yaml:"activitypub_enabled" json:"activitypub_enabled"`
	WebmentionEnabled  bool `yaml:"webmention_enabled" json:"webmention_enabled"`
	IndieAuthEnabled   bool `yaml:"indieauth_enabled" json:"indieauth_enabled"`
	MicropubEnabled    bool `yaml:"micropub_enabled" json:"micropub_enabled"`
}

// AuthorConfig holds author information.
type AuthorConfig struct {
	Name   string `yaml:"name" json:"name"`
	Bio    string `yaml:"bio" json:"bio"`
	Avatar string `yaml:"avatar" json:"avatar"`
}

// SocialConfig holds social media links.
type SocialConfig struct {
	Bluesky  string `yaml:"bluesky" json:"bluesky"`
	Github   string `yaml:"github" json:"github"`
	Linkedin string `yaml:"linkedin" json:"linkedin"`
	Youtube  string `yaml:"youtube" json:"youtube"`
	Twitter  string `yaml:"twitter" json:"twitter"`
}

// FooterConfig holds footer section configuration.
type FooterConfig struct {
	Sections []FooterSection `yaml:"sections" json:"sections"`
}

// FooterSection is a group of links in the footer.
type FooterSection struct {
	Title string       `yaml:"title" json:"title"`
	Links []FooterLink `yaml:"links" json:"links"`
}

// FooterLink is a single link in a footer section.
type FooterLink struct {
	Label string `yaml:"label" json:"label"`
	URL   string `yaml:"url" json:"url"`
}

// SecretsConfig holds sensitive configuration from secrets.yaml.
type SecretsConfig struct {
	AdminPasswordHash     string `yaml:"admin_password"`
	SessionSecret         string `yaml:"session_secret"`
	UnsplashAccessKey     string `yaml:"unsplash_access_key"`
	TOTPSecret            string `yaml:"totp_secret"`
	ActivityPubPrivateKey string `yaml:"activitypub_private_key"`
	ActivityPubPublicKey  string `yaml:"activitypub_public_key"`
}

// SMTPConfig holds email configuration from smtp.yaml.
type SMTPConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	From     string `yaml:"from"`
}

// Load reads all configuration files from the given directory.
func Load(dir string) (*Config, error) {
	c := &Config{
		dir: dir,
	}

	if err := c.loadAll(); err != nil {
		return nil, err
	}

	// Apply defaults
	if c.Site.PostsPerPage <= 0 {
		c.Site.PostsPerPage = 10
	}
	if c.Site.Theme == "" {
		c.Site.Theme = "classic"
	}

	// IndieWeb/Fediverse features default to enabled.
	// We detect "never configured" by checking if site.yaml lacks all four keys.
	// Once the user saves settings, these will be explicit in the file.
	c.applyIndieWebDefaults()

	// Auto-generate session secret if empty
	if c.Secrets.SessionSecret == "" {
		secret, err := generateSessionSecret()
		if err != nil {
			return nil, fmt.Errorf("generating session secret: %w", err)
		}
		c.Secrets.SessionSecret = secret
		// Persist back to secrets.yaml
		if err := c.saveSecrets(); err != nil {
			return nil, fmt.Errorf("saving session secret: %w", err)
		}
	}

	return c, nil
}

// GetSite returns a copy of the site configuration (thread-safe).
func (c *Config) GetSite() SiteConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Site
}

// GetSecrets returns a copy of the secrets configuration (thread-safe).
func (c *Config) GetSecrets() SecretsConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Secrets
}

// GetSMTP returns a copy of the SMTP configuration (thread-safe).
func (c *Config) GetSMTP() SMTPConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.SMTP
}

// GetConfigDir returns the config directory path.
func (c *Config) GetConfigDir() string {
	return c.dir
}

// VerifyAdminPassword checks if the provided password matches the stored hash.
func (c *Config) VerifyAdminPassword(password string) bool {
	c.mu.RLock()
	hash := c.Secrets.AdminPasswordHash
	c.mu.RUnlock()

	if hash == "" {
		return false
	}

	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// WatchForChanges starts watching config files for changes and reloads them.
func (c *Config) WatchForChanges(logger *slog.Logger) error {
	c.logger = logger

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("creating config watcher: %w", err)
	}

	go func() {
		defer func() { _ = watcher.Close() }()
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) {
					logger.Info("config file changed, reloading", "file", event.Name)
					if err := c.reload(); err != nil {
						logger.Error("failed to reload config", "error", err)
					}
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				logger.Error("config watcher error", "error", err)
			}
		}
	}()

	return watcher.Add(c.dir)
}

func (c *Config) loadAll() error {
	if err := c.loadFile("site.yaml", &c.Site); err != nil {
		return fmt.Errorf("loading site.yaml: %w", err)
	}
	if err := c.loadFile("secrets.yaml", &c.Secrets); err != nil {
		// Secrets file is optional on first run
		if !os.IsNotExist(err) {
			return fmt.Errorf("loading secrets.yaml: %w", err)
		}
	}
	if err := c.loadFile("smtp.yaml", &c.SMTP); err != nil {
		// SMTP file is optional
		if !os.IsNotExist(err) {
			return fmt.Errorf("loading smtp.yaml: %w", err)
		}
	}
	return nil
}

// applyIndieWebDefaults sets IndieWeb/Fediverse features to enabled if not explicitly
// configured in site.yaml. We detect "never configured" by reading the raw YAML
// to check if the keys exist.
func (c *Config) applyIndieWebDefaults() {
	path := filepath.Join(c.dir, "site.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		// Site.yaml missing — set all to true
		c.Site.ActivityPubEnabled = true
		c.Site.WebmentionEnabled = true
		c.Site.IndieAuthEnabled = true
		c.Site.MicropubEnabled = true
		return
	}

	// Check if ANY of the IndieWeb keys are explicitly present in the YAML
	content := string(data)
	hasAP := strings.Contains(content, "activitypub_enabled:")
	hasWM := strings.Contains(content, "webmention_enabled:")
	hasIA := strings.Contains(content, "indieauth_enabled:")
	hasMP := strings.Contains(content, "micropub_enabled:")

	// If none are present, the user hasn't configured them — default all to true
	if !hasAP && !hasWM && !hasIA && !hasMP {
		c.Site.ActivityPubEnabled = true
		c.Site.WebmentionEnabled = true
		c.Site.IndieAuthEnabled = true
		c.Site.MicropubEnabled = true
	} else {
		// At least one is present — respect whatever was parsed (explicit false or true)
		// If individual keys are missing, default them to true
		if !hasAP {
			c.Site.ActivityPubEnabled = true
		}
		if !hasWM {
			c.Site.WebmentionEnabled = true
		}
		if !hasIA {
			c.Site.IndieAuthEnabled = true
		}
		if !hasMP {
			c.Site.MicropubEnabled = true
		}
	}
}

func (c *Config) loadFile(name string, target interface{}) error {
	path := filepath.Join(c.dir, name)
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := yaml.Unmarshal(data, target); err != nil {
		return fmt.Errorf("parsing %s: %w", name, err)
	}
	return nil
}

func (c *Config) reload() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.loadAll()
}

// UpdateSite updates the site configuration and persists it to site.yaml.
func (c *Config) UpdateSite(site SiteConfig) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.Site = site

	data, err := yaml.Marshal(&site)
	if err != nil {
		return fmt.Errorf("marshaling site config: %w", err)
	}
	path := filepath.Join(c.dir, "site.yaml")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("writing site.yaml: %w", err)
	}

	if c.logger != nil {
		c.logger.Info("site config updated and saved")
	}
	return nil
}

// ListThemes scans both the content directory and embedded FS for available themes.
func ListThemes(contentDir string, embeddedThemes embed.FS) ([]ThemeInfo, error) {
	seen := make(map[string]bool)
	var themes []ThemeInfo

	// First, scan content directory themes (user overrides)
	themesDir := filepath.Join(contentDir, "themes")
	entries, err := os.ReadDir(themesDir)
	if err == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			themeYaml := filepath.Join(themesDir, entry.Name(), "theme.yaml")
			data, err := os.ReadFile(themeYaml)
			if err != nil {
				continue // skip themes without theme.yaml
			}
			var info ThemeInfo
			if err := yaml.Unmarshal(data, &info); err != nil {
				continue
			}
			info.ID = entry.Name()
			themes = append(themes, info)
			seen[entry.Name()] = true
		}
	}

	// Then, scan embedded themes for ones not already overridden
	embeddedEntries, err := fs.ReadDir(embeddedThemes, "themes")
	if err == nil {
		for _, entry := range embeddedEntries {
			if !entry.IsDir() || seen[entry.Name()] {
				continue
			}
			data, err := fs.ReadFile(embeddedThemes, "themes/"+entry.Name()+"/theme.yaml")
			if err != nil {
				continue
			}
			var info ThemeInfo
			if err := yaml.Unmarshal(data, &info); err != nil {
				continue
			}
			info.ID = entry.Name()
			themes = append(themes, info)
		}
	}

	return themes, nil
}

// ThemeInfo holds metadata about an available theme.
type ThemeInfo struct {
	ID          string `json:"id" yaml:"-"`
	Name        string `json:"name" yaml:"name"`
	Description string `json:"description" yaml:"description"`
	Author      string `json:"author" yaml:"author"`
	Version     string `json:"version" yaml:"version"`
	Layout      string `json:"layout" yaml:"layout"`
}

func (c *Config) saveSecrets() error {
	data, err := yaml.Marshal(&c.Secrets)
	if err != nil {
		return fmt.Errorf("marshaling secrets: %w", err)
	}
	path := filepath.Join(c.dir, "secrets.yaml")
	return os.WriteFile(path, data, 0o600)
}

// UpdateSecrets updates the secrets configuration and persists it.
func (c *Config) UpdateSecrets(secrets SecretsConfig) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Secrets = secrets
	return c.saveSecrets()
}

// IsTOTPEnabled returns whether TOTP 2FA is configured.
func (c *Config) IsTOTPEnabled() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Secrets.TOTPSecret != ""
}

func generateSessionSecret() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generating random bytes: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}

// --- i18n Translations ---

// builtinTranslations provides translated UI strings for common blog elements.
var builtinTranslations = map[string]map[string]string{
	"en": {
		"home": "Home", "archive": "Archive", "tags": "Tags", "search": "Search",
		"read_more": "Read more", "min_read": "min read", "published_on": "Published on",
		"previous": "Previous", "next": "Next", "newer": "Newer", "older": "Older",
		"comments": "Comments", "post_comment": "Post Comment", "name": "Name", "email": "Email", "message": "Message",
		"subscribe": "Subscribe", "unsubscribe": "Unsubscribe", "your_email": "Your email",
		"search_placeholder": "Search posts...", "no_results": "No results found.",
		"powered_by": "Powered by", "copyright": "©", "all_rights_reserved": "All rights reserved.",
		"like": "Like", "dislike": "Dislike", "share": "Share",
		"related_posts": "Related Posts", "tag_page": "Posts tagged",
		"draft": "Draft", "featured": "Featured",
	},
	"es": {
		"home": "Inicio", "archive": "Archivo", "tags": "Etiquetas", "search": "Buscar",
		"read_more": "Leer más", "min_read": "min de lectura", "published_on": "Publicado el",
		"previous": "Anterior", "next": "Siguiente", "newer": "Más reciente", "older": "Más antiguo",
		"comments": "Comentarios", "post_comment": "Publicar comentario", "name": "Nombre", "email": "Correo", "message": "Mensaje",
		"subscribe": "Suscribirse", "unsubscribe": "Cancelar suscripción", "your_email": "Tu correo",
		"search_placeholder": "Buscar artículos...", "no_results": "No se encontraron resultados.",
		"powered_by": "Desarrollado con", "copyright": "©", "all_rights_reserved": "Todos los derechos reservados.",
		"like": "Me gusta", "dislike": "No me gusta", "share": "Compartir",
		"related_posts": "Artículos relacionados", "tag_page": "Artículos etiquetados",
		"draft": "Borrador", "featured": "Destacado",
	},
	"fr": {
		"home": "Accueil", "archive": "Archives", "tags": "Tags", "search": "Recherche",
		"read_more": "Lire la suite", "min_read": "min de lecture", "published_on": "Publié le",
		"previous": "Précédent", "next": "Suivant", "newer": "Plus récent", "older": "Plus ancien",
		"comments": "Commentaires", "post_comment": "Publier", "name": "Nom", "email": "Email", "message": "Message",
		"subscribe": "S'abonner", "unsubscribe": "Se désabonner", "your_email": "Votre email",
		"search_placeholder": "Rechercher...", "no_results": "Aucun résultat.",
		"powered_by": "Propulsé par", "copyright": "©", "all_rights_reserved": "Tous droits réservés.",
		"like": "J'aime", "dislike": "Je n'aime pas", "share": "Partager",
		"related_posts": "Articles connexes", "tag_page": "Articles tagués",
		"draft": "Brouillon", "featured": "À la une",
	},
	"de": {
		"home": "Startseite", "archive": "Archiv", "tags": "Tags", "search": "Suche",
		"read_more": "Weiterlesen", "min_read": "Min. Lesezeit", "published_on": "Veröffentlicht am",
		"previous": "Zurück", "next": "Weiter", "newer": "Neuer", "older": "Älter",
		"comments": "Kommentare", "post_comment": "Kommentieren", "name": "Name", "email": "E-Mail", "message": "Nachricht",
		"subscribe": "Abonnieren", "unsubscribe": "Abbestellen", "your_email": "Ihre E-Mail",
		"search_placeholder": "Beiträge suchen...", "no_results": "Keine Ergebnisse.",
		"powered_by": "Betrieben mit", "copyright": "©", "all_rights_reserved": "Alle Rechte vorbehalten.",
		"like": "Gefällt mir", "dislike": "Gefällt mir nicht", "share": "Teilen",
		"related_posts": "Ähnliche Beiträge", "tag_page": "Beiträge mit Tag",
		"draft": "Entwurf", "featured": "Empfohlen",
	},
	"ja": {
		"home": "ホーム", "archive": "アーカイブ", "tags": "タグ", "search": "検索",
		"read_more": "続きを読む", "min_read": "分で読了", "published_on": "公開日",
		"previous": "前へ", "next": "次へ", "newer": "新しい", "older": "古い",
		"comments": "コメント", "post_comment": "コメントする", "name": "名前", "email": "メール", "message": "メッセージ",
		"subscribe": "購読", "unsubscribe": "購読解除", "your_email": "メールアドレス",
		"search_placeholder": "記事を検索...", "no_results": "結果が見つかりませんでした。",
		"powered_by": "Powered by", "copyright": "©", "all_rights_reserved": "全著作権所有。",
		"like": "いいね", "dislike": "よくない", "share": "共有",
		"related_posts": "関連記事", "tag_page": "タグ付き記事",
		"draft": "下書き", "featured": "注目",
	},
	"zh": {
		"home": "首页", "archive": "归档", "tags": "标签", "search": "搜索",
		"read_more": "阅读更多", "min_read": "分钟阅读", "published_on": "发布于",
		"previous": "上一页", "next": "下一页", "newer": "更新", "older": "更早",
		"comments": "评论", "post_comment": "发表评论", "name": "姓名", "email": "邮箱", "message": "内容",
		"subscribe": "订阅", "unsubscribe": "取消订阅", "your_email": "你的邮箱",
		"search_placeholder": "搜索文章...", "no_results": "未找到结果。",
		"powered_by": "技术支持", "copyright": "©", "all_rights_reserved": "保留所有权利。",
		"like": "喜欢", "dislike": "不喜欢", "share": "分享",
		"related_posts": "相关文章", "tag_page": "标签文章",
		"draft": "草稿", "featured": "推荐",
	},
}

// Translate returns the localized string for a key, falling back to English.
func (c *Config) Translate(key string) string {
	locale := c.GetSite().Locale
	if locale == "" {
		locale = "en"
	}
	if t, ok := builtinTranslations[locale]; ok {
		if v, ok := t[key]; ok {
			return v
		}
	}
	// Fallback to English
	if t, ok := builtinTranslations["en"]; ok {
		if v, ok := t[key]; ok {
			return v
		}
	}
	return key
}
