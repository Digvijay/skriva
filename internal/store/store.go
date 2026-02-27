// Package store manages the SQLite database for comments, page views,
// and reactions. It uses modernc.org/sqlite (pure Go, no CGO required).
package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// Store wraps the SQLite database connection and provides typed methods.
type Store struct {
	db     *sql.DB
	logger *slog.Logger
}

// Comment represents a blog comment.
type Comment struct {
	ID        int64     `json:"id"`
	PostSlug  string    `json:"post_slug"`
	Author    string    `json:"author"`
	Email     string    `json:"email,omitempty"` // Never exposed publicly
	Content   string    `json:"content"`
	Approved  bool      `json:"approved"`
	CreatedAt time.Time `json:"created_at"`
}

// PostStats holds aggregated statistics for a post.
type PostStats struct {
	Views    int64 `json:"views"`
	Likes    int64 `json:"likes"`
	Dislikes int64 `json:"dislikes"`
}

// DashboardStats holds overall blog statistics.
type DashboardStats struct {
	TotalViews    int64        `json:"total_views"`
	TotalPosts    int64        `json:"total_posts"`
	TotalComments int64        `json:"total_comments"`
	TopPosts      []TopPost    `json:"top_posts"`
	RecentViews   []DailyViews `json:"recent_views"`
}

// TopPost is a post with its view count for dashboard display.
type TopPost struct {
	Slug  string `json:"slug"`
	Title string `json:"title"`
	Views int64  `json:"views"`
}

// DailyViews holds view count for a single day.
type DailyViews struct {
	Date  string `json:"date"`
	Count int64  `json:"count"`
}

// New creates a new Store, opening or creating the SQLite database.
func New(configDir string, logger *slog.Logger) (*Store, error) {
	dbPath := filepath.Join(configDir, "blog.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening database %q: %w", dbPath, err)
	}

	// Enable WAL mode for better concurrent read performance
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("enabling WAL mode: %w", err)
	}

	// Enable foreign keys
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("enabling foreign keys: %w", err)
	}

	s := &Store{db: db, logger: logger}

	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("running migrations: %w", err)
	}

	logger.Info("database initialized", "path", dbPath)
	return s, nil
}

// Close closes the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// DB returns the underlying database connection for direct queries.
func (s *Store) DB() *sql.DB {
	return s.db
}

func (s *Store) migrate() error {
	// Create migration tracking table first (always safe to run)
	if _, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY,
		name TEXT NOT NULL,
		applied_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`); err != nil {
		return fmt.Errorf("creating schema_migrations table: %w", err)
	}

	// Run all pending migrations
	for _, m := range allMigrations {
		applied, err := s.isMigrationApplied(m.Version)
		if err != nil {
			return fmt.Errorf("checking migration %d: %w", m.Version, err)
		}
		if applied {
			continue
		}

		s.logger.Info("applying migration", "version", m.Version, "name", m.Name)
		for _, sql := range m.Up {
			if _, err := s.db.Exec(sql); err != nil {
				// Ignore "duplicate column" and "already exists" for idempotency
				errStr := err.Error()
				if strings.Contains(errStr, "duplicate column") || strings.Contains(errStr, "already exists") {
					continue
				}
				return fmt.Errorf("migration %d (%s) failed: %w\nSQL: %s", m.Version, m.Name, err, sql)
			}
		}

		if _, err := s.db.Exec("INSERT INTO schema_migrations (version, name) VALUES (?, ?)", m.Version, m.Name); err != nil {
			return fmt.Errorf("recording migration %d: %w", m.Version, err)
		}
	}

	return nil
}

func (s *Store) isMigrationApplied(version int) (bool, error) {
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM schema_migrations WHERE version = ?", version).Scan(&count)
	return count > 0, err
}

// RollbackMigration rolls back the last applied migration.
func (s *Store) RollbackMigration() error {
	var version int
	var name string
	err := s.db.QueryRow("SELECT version, name FROM schema_migrations ORDER BY version DESC LIMIT 1").Scan(&version, &name)
	if err != nil {
		return fmt.Errorf("no migrations to rollback")
	}

	// Find the migration definition
	var migration *Migration
	for i := range allMigrations {
		if allMigrations[i].Version == version {
			migration = &allMigrations[i]
			break
		}
	}
	if migration == nil {
		return fmt.Errorf("migration %d not found in code", version)
	}
	if len(migration.Down) == 0 {
		return fmt.Errorf("migration %d (%s) has no rollback SQL", version, name)
	}

	s.logger.Info("rolling back migration", "version", version, "name", name)
	for _, sql := range migration.Down {
		if _, err := s.db.Exec(sql); err != nil {
			return fmt.Errorf("rollback %d failed: %w\nSQL: %s", version, err, sql)
		}
	}

	if _, err := s.db.Exec("DELETE FROM schema_migrations WHERE version = ?", version); err != nil {
		return fmt.Errorf("removing migration record %d: %w", version, err)
	}

	s.logger.Info("migration rolled back", "version", version, "name", name)
	return nil
}

// MigrationStatus returns which migrations are applied.
func (s *Store) MigrationStatus() ([]MigrationInfo, error) {
	applied := make(map[int]string)
	rows, err := s.db.Query("SELECT version, applied_at FROM schema_migrations ORDER BY version ASC")
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var v int
			var at string
			_ = rows.Scan(&v, &at)
			applied[v] = at
		}
	}

	var infos []MigrationInfo
	for _, m := range allMigrations {
		info := MigrationInfo{
			Version: m.Version,
			Name:    m.Name,
			HasDown: len(m.Down) > 0,
		}
		if at, ok := applied[m.Version]; ok {
			info.Applied = true
			info.AppliedAt = at
		}
		infos = append(infos, info)
	}
	return infos, nil
}

// MigrationInfo holds status info about a single migration.
type MigrationInfo struct {
	Version   int    `json:"version"`
	Name      string `json:"name"`
	Applied   bool   `json:"applied"`
	AppliedAt string `json:"applied_at,omitempty"`
	HasDown   bool   `json:"has_rollback"`
}

// Migration defines a versioned schema change with up and down SQL.
type Migration struct {
	Version int
	Name    string
	Up      []string
	Down    []string // empty = irreversible
}

// allMigrations is the ordered list of all schema migrations.
var allMigrations = []Migration{
	{
		Version: 1, Name: "initial_schema",
		Up: []string{
			`CREATE TABLE IF NOT EXISTS comments (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				post_slug TEXT NOT NULL, author TEXT NOT NULL, email TEXT DEFAULT '',
				content TEXT NOT NULL, created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
				CHECK(length(author) > 0 AND length(author) <= 100),
				CHECK(length(content) > 0 AND length(content) <= 5000))`,
			`CREATE INDEX IF NOT EXISTS idx_comments_post_slug ON comments(post_slug)`,
			`CREATE TABLE IF NOT EXISTS page_views (
				id INTEGER PRIMARY KEY AUTOINCREMENT, post_slug TEXT NOT NULL,
				view_date DATE NOT NULL DEFAULT (date('now')), count INTEGER NOT NULL DEFAULT 1,
				UNIQUE(post_slug, view_date))`,
			`CREATE INDEX IF NOT EXISTS idx_page_views_slug ON page_views(post_slug)`,
			`CREATE TABLE IF NOT EXISTS reactions (
				id INTEGER PRIMARY KEY AUTOINCREMENT, post_slug TEXT NOT NULL,
				reaction_type TEXT NOT NULL CHECK(reaction_type IN ('like', 'dislike')),
				fingerprint TEXT NOT NULL, created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
				UNIQUE(post_slug, fingerprint))`,
			`CREATE INDEX IF NOT EXISTS idx_reactions_slug ON reactions(post_slug)`,
		},
		Down: []string{
			`DROP TABLE IF EXISTS reactions`, `DROP TABLE IF EXISTS page_views`, `DROP TABLE IF EXISTS comments`,
		},
	},
	{
		Version: 2, Name: "comment_moderation",
		Up:   []string{`ALTER TABLE comments ADD COLUMN approved INTEGER NOT NULL DEFAULT 1`},
		Down: []string{}, // SQLite doesn't support DROP COLUMN before 3.35
	},
	{
		Version: 3, Name: "passkeys",
		Up: []string{`CREATE TABLE IF NOT EXISTS passkeys (
			id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT NOT NULL DEFAULT 'Passkey',
			credential_id BLOB NOT NULL UNIQUE, public_key BLOB NOT NULL,
			attestation_type TEXT NOT NULL DEFAULT '', aaguid BLOB,
			sign_count INTEGER NOT NULL DEFAULT 0, transports TEXT NOT NULL DEFAULT '[]',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP)`},
		Down: []string{`DROP TABLE IF EXISTS passkeys`},
	},
	{
		Version: 4, Name: "unique_views",
		Up: []string{`CREATE TABLE IF NOT EXISTS unique_views (
			post_slug TEXT NOT NULL, view_date DATE NOT NULL DEFAULT (date('now')),
			visitor_hash TEXT NOT NULL, UNIQUE(post_slug, view_date, visitor_hash))`},
		Down: []string{`DROP TABLE IF EXISTS unique_views`},
	},
	{
		Version: 5, Name: "subscribers",
		Up: []string{`CREATE TABLE IF NOT EXISTS subscribers (
			id INTEGER PRIMARY KEY AUTOINCREMENT, email TEXT NOT NULL UNIQUE,
			confirmed INTEGER NOT NULL DEFAULT 0, token TEXT NOT NULL DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP)`},
		Down: []string{`DROP TABLE IF EXISTS subscribers`},
	},
	{
		Version: 6, Name: "newsletters",
		Up: []string{`CREATE TABLE IF NOT EXISTS newsletters (
			id INTEGER PRIMARY KEY AUTOINCREMENT, subject TEXT NOT NULL, body TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'draft' CHECK(status IN ('draft','scheduled','sending','sent','failed')),
			scheduled_at DATETIME, sent_at DATETIME, sent_count INTEGER NOT NULL DEFAULT 0,
			fail_count INTEGER NOT NULL DEFAULT 0, created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP)`},
		Down: []string{`DROP TABLE IF EXISTS newsletters`},
	},
	{
		Version: 7, Name: "post_revisions",
		Up: []string{
			`CREATE TABLE IF NOT EXISTS post_revisions (
				id INTEGER PRIMARY KEY AUTOINCREMENT, post_slug TEXT NOT NULL,
				title TEXT NOT NULL, content TEXT NOT NULL, metadata TEXT NOT NULL DEFAULT '{}',
				created_at DATETIME DEFAULT CURRENT_TIMESTAMP)`,
			`CREATE INDEX IF NOT EXISTS idx_post_revisions_slug ON post_revisions(post_slug)`,
		},
		Down: []string{`DROP TABLE IF EXISTS post_revisions`},
	},
	{
		Version: 8, Name: "newsletter_analytics",
		Up: []string{
			`CREATE TABLE IF NOT EXISTS newsletter_events (
				id INTEGER PRIMARY KEY AUTOINCREMENT, newsletter_id INTEGER NOT NULL,
				subscriber_id INTEGER NOT NULL DEFAULT 0,
				event_type TEXT NOT NULL CHECK(event_type IN ('open','click')),
				url TEXT NOT NULL DEFAULT '', created_at DATETIME DEFAULT CURRENT_TIMESTAMP)`,
			`CREATE INDEX IF NOT EXISTS idx_newsletter_events_nid ON newsletter_events(newsletter_id)`,
		},
		Down: []string{`DROP TABLE IF EXISTS newsletter_events`},
	},
	{
		Version: 9, Name: "newsletter_ab_testing",
		Up: []string{
			`ALTER TABLE newsletters ADD COLUMN variant_of INTEGER DEFAULT NULL`,
			`ALTER TABLE newsletters ADD COLUMN variant_label TEXT NOT NULL DEFAULT ''`,
		},
		Down: []string{}, // ALTER TABLE DROP COLUMN not safe on older SQLite
	},
	{
		Version: 10, Name: "draft_shares",
		Up: []string{`CREATE TABLE IF NOT EXISTS draft_shares (
			id INTEGER PRIMARY KEY AUTOINCREMENT, post_slug TEXT NOT NULL,
			token TEXT NOT NULL UNIQUE, expires_at DATETIME NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP)`},
		Down: []string{`DROP TABLE IF EXISTS draft_shares`},
	},
	{
		Version: 11, Name: "webmentions",
		Up: []string{
			`CREATE TABLE IF NOT EXISTS webmentions (
				id INTEGER PRIMARY KEY AUTOINCREMENT, source TEXT NOT NULL, target TEXT NOT NULL,
				post_slug TEXT NOT NULL DEFAULT '', author_name TEXT NOT NULL DEFAULT '',
				author_url TEXT NOT NULL DEFAULT '', author_avatar TEXT NOT NULL DEFAULT '',
				content TEXT NOT NULL DEFAULT '', mention_type TEXT NOT NULL DEFAULT 'mention',
				verified INTEGER NOT NULL DEFAULT 0, created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
				UNIQUE(source, target))`,
			`CREATE INDEX IF NOT EXISTS idx_webmentions_slug ON webmentions(post_slug)`,
		},
		Down: []string{`DROP TABLE IF EXISTS webmentions`},
	},
	{
		Version: 12, Name: "activitypub_followers",
		Up: []string{`CREATE TABLE IF NOT EXISTS activitypub_followers (
			id INTEGER PRIMARY KEY AUTOINCREMENT, actor_url TEXT NOT NULL UNIQUE,
			inbox_url TEXT NOT NULL, shared_inbox_url TEXT NOT NULL DEFAULT '',
			name TEXT NOT NULL DEFAULT '', created_at DATETIME DEFAULT CURRENT_TIMESTAMP)`},
		Down: []string{`DROP TABLE IF EXISTS activitypub_followers`},
	},
	{
		Version: 13, Name: "webhooks",
		Up: []string{`CREATE TABLE IF NOT EXISTS webhooks (
			id INTEGER PRIMARY KEY AUTOINCREMENT, event TEXT NOT NULL, url TEXT NOT NULL,
			secret TEXT NOT NULL DEFAULT '', active INTEGER NOT NULL DEFAULT 1,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP)`},
		Down: []string{`DROP TABLE IF EXISTS webhooks`},
	},
	{
		Version: 14, Name: "api_tokens",
		Up: []string{`CREATE TABLE IF NOT EXISTS api_tokens (
			id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT NOT NULL,
			token_hash TEXT NOT NULL UNIQUE, scopes TEXT NOT NULL DEFAULT 'read',
			last_used_at DATETIME, created_at DATETIME DEFAULT CURRENT_TIMESTAMP)`},
		Down: []string{`DROP TABLE IF EXISTS api_tokens`},
	},
	{
		Version: 15, Name: "newsletter_sends",
		Up: []string{
			`CREATE TABLE IF NOT EXISTS newsletter_sends (
				id INTEGER PRIMARY KEY AUTOINCREMENT, newsletter_id INTEGER NOT NULL,
				subscriber_id INTEGER NOT NULL,
				status TEXT NOT NULL DEFAULT 'pending' CHECK(status IN ('pending','sent','failed')),
				error_msg TEXT NOT NULL DEFAULT '', sent_at DATETIME,
				UNIQUE(newsletter_id, subscriber_id))`,
			`CREATE INDEX IF NOT EXISTS idx_newsletter_sends_nid ON newsletter_sends(newsletter_id)`,
		},
		Down: []string{`DROP TABLE IF EXISTS newsletter_sends`},
	},
	{
		Version: 16, Name: "audit_log",
		Up: []string{
			`CREATE TABLE IF NOT EXISTS audit_log (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				event TEXT NOT NULL,
				detail TEXT NOT NULL DEFAULT '',
				ip TEXT NOT NULL DEFAULT '',
				created_at DATETIME DEFAULT CURRENT_TIMESTAMP)`,
			`CREATE INDEX IF NOT EXISTS idx_audit_log_created ON audit_log(created_at)`,
			`CREATE INDEX IF NOT EXISTS idx_audit_log_event ON audit_log(event)`,
		},
		Down: []string{`DROP TABLE IF EXISTS audit_log`},
	},
}

// --- Comments ---

// AddComment inserts a new comment (unapproved by default).
func (s *Store) AddComment(ctx context.Context, c *Comment) error {
	result, err := s.db.ExecContext(ctx,
		"INSERT INTO comments (post_slug, author, email, content, approved) VALUES (?, ?, ?, ?, 0)",
		c.PostSlug, c.Author, c.Email, c.Content,
	)
	if err != nil {
		return fmt.Errorf("inserting comment: %w", err)
	}
	id, _ := result.LastInsertId()
	c.ID = id
	c.CreatedAt = time.Now()
	c.Approved = false
	return nil
}

// CommentsByPost returns approved comments for a given post slug.
func (s *Store) CommentsByPost(ctx context.Context, slug string) ([]Comment, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT id, post_slug, author, content, approved, created_at FROM comments WHERE post_slug = ? AND approved = 1 ORDER BY created_at ASC",
		slug,
	)
	if err != nil {
		return nil, fmt.Errorf("querying comments for %q: %w", slug, err)
	}
	defer rows.Close()

	var comments []Comment
	for rows.Next() {
		var c Comment
		if err := rows.Scan(&c.ID, &c.PostSlug, &c.Author, &c.Content, &c.Approved, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning comment: %w", err)
		}
		comments = append(comments, c)
	}
	return comments, rows.Err()
}

// AllComments returns all comments across all posts (for admin), including pending.
func (s *Store) AllComments(ctx context.Context) ([]Comment, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT id, post_slug, author, content, approved, created_at FROM comments ORDER BY approved ASC, created_at DESC",
	)
	if err != nil {
		return nil, fmt.Errorf("querying all comments: %w", err)
	}
	defer rows.Close()

	var comments []Comment
	for rows.Next() {
		var c Comment
		if err := rows.Scan(&c.ID, &c.PostSlug, &c.Author, &c.Content, &c.Approved, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning comment: %w", err)
		}
		comments = append(comments, c)
	}
	return comments, rows.Err()
}

// ApproveComment marks a comment as approved.
func (s *Store) ApproveComment(ctx context.Context, id int64) error {
	result, err := s.db.ExecContext(ctx, "UPDATE comments SET approved = 1 WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("approving comment %d: %w", id, err)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("comment %d not found", id)
	}
	return nil
}

// UnapproveComment marks a comment as pending (un-approve).
func (s *Store) UnapproveComment(ctx context.Context, id int64) error {
	result, err := s.db.ExecContext(ctx, "UPDATE comments SET approved = 0 WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("unapproving comment %d: %w", id, err)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("comment %d not found", id)
	}
	return nil
}

// PendingCommentCount returns the number of unapproved comments.
func (s *Store) PendingCommentCount(ctx context.Context) (int64, error) {
	var count int64
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM comments WHERE approved = 0").Scan(&count)
	return count, err
}

// DeleteComment removes a comment by ID.
func (s *Store) DeleteComment(ctx context.Context, id int64) error {
	result, err := s.db.ExecContext(ctx, "DELETE FROM comments WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("deleting comment %d: %w", id, err)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("comment %d not found", id)
	}
	return nil
}

// --- Page Views ---

// TrackView records a unique page view for a post, deduplicated by visitor per day.
// The visitorHash should be a hashed IP or fingerprint — not raw IP.
func (s *Store) TrackView(ctx context.Context, slug, visitorHash string) error {
	// Insert unique visitor — silently skip if already counted today
	result, err := s.db.ExecContext(ctx,
		`INSERT OR IGNORE INTO unique_views (post_slug, view_date, visitor_hash) VALUES (?, date('now'), ?)`,
		slug, visitorHash,
	)
	if err != nil {
		return fmt.Errorf("tracking unique view for %q: %w", slug, err)
	}

	// Only increment the counter if this was a new unique visit
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected > 0 {
		_, err = s.db.ExecContext(ctx,
			`INSERT INTO page_views (post_slug, view_date, count) VALUES (?, date('now'), 1)
			 ON CONFLICT(post_slug, view_date) DO UPDATE SET count = count + 1`,
			slug,
		)
		if err != nil {
			return fmt.Errorf("incrementing view count for %q: %w", slug, err)
		}
	}
	return nil
}

// ViewsByPost returns the total view count for a post.
func (s *Store) ViewsByPost(ctx context.Context, slug string) (int64, error) {
	var total int64
	err := s.db.QueryRowContext(ctx,
		"SELECT COALESCE(SUM(count), 0) FROM page_views WHERE post_slug = ?",
		slug,
	).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("querying views for %q: %w", slug, err)
	}
	return total, nil
}

// --- Reactions ---

// AddReaction adds a like or dislike. Returns false if the fingerprint already reacted.
func (s *Store) AddReaction(ctx context.Context, slug, reactionType, fingerprint string) (bool, error) {
	_, err := s.db.ExecContext(ctx,
		"INSERT OR IGNORE INTO reactions (post_slug, reaction_type, fingerprint) VALUES (?, ?, ?)",
		slug, reactionType, fingerprint,
	)
	if err != nil {
		return false, fmt.Errorf("adding reaction: %w", err)
	}

	// Check if it was actually inserted (not ignored due to unique constraint)
	var count int
	err = s.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM reactions WHERE post_slug = ? AND fingerprint = ?",
		slug, fingerprint,
	).Scan(&count)
	return count > 0, err
}

// ReactionsByPost returns the like and dislike counts for a post.
func (s *Store) ReactionsByPost(ctx context.Context, slug string) (likes, dislikes int64, err error) {
	err = s.db.QueryRowContext(ctx,
		"SELECT COALESCE(SUM(CASE WHEN reaction_type='like' THEN 1 ELSE 0 END), 0), COALESCE(SUM(CASE WHEN reaction_type='dislike' THEN 1 ELSE 0 END), 0) FROM reactions WHERE post_slug = ?",
		slug,
	).Scan(&likes, &dislikes)
	if err != nil {
		return 0, 0, fmt.Errorf("querying reactions for %q: %w", slug, err)
	}
	return likes, dislikes, nil
}

// --- Stats (Dashboard) ---

// PostStatsForSlug returns combined stats for a single post.
func (s *Store) PostStatsForSlug(ctx context.Context, slug string) (*PostStats, error) {
	views, err := s.ViewsByPost(ctx, slug)
	if err != nil {
		return nil, err
	}

	likes, dislikes, err := s.ReactionsByPost(ctx, slug)
	if err != nil {
		return nil, err
	}

	return &PostStats{
		Views:    views,
		Likes:    likes,
		Dislikes: dislikes,
	}, nil
}

// GetDashboardStats returns aggregated stats for the admin dashboard.
func (s *Store) GetDashboardStats(ctx context.Context) (*DashboardStats, error) {
	stats := &DashboardStats{}

	// Total views
	err := s.db.QueryRowContext(ctx, "SELECT COALESCE(SUM(count), 0) FROM page_views").Scan(&stats.TotalViews)
	if err != nil {
		return nil, fmt.Errorf("querying total views: %w", err)
	}

	// Total comments
	err = s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM comments").Scan(&stats.TotalComments)
	if err != nil {
		return nil, fmt.Errorf("querying total comments: %w", err)
	}

	// Top posts by views
	rows, err := s.db.QueryContext(ctx,
		"SELECT post_slug, SUM(count) as total FROM page_views GROUP BY post_slug ORDER BY total DESC LIMIT 10",
	)
	if err != nil {
		return nil, fmt.Errorf("querying top posts: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var tp TopPost
		if err := rows.Scan(&tp.Slug, &tp.Views); err != nil {
			return nil, fmt.Errorf("scanning top post: %w", err)
		}
		stats.TopPosts = append(stats.TopPosts, tp)
	}

	// Recent daily views (last 30 days)
	rows2, err := s.db.QueryContext(ctx,
		"SELECT view_date, SUM(count) FROM page_views WHERE view_date >= date('now', '-30 days') GROUP BY view_date ORDER BY view_date ASC",
	)
	if err != nil {
		return nil, fmt.Errorf("querying recent views: %w", err)
	}
	defer rows2.Close()

	for rows2.Next() {
		var dv DailyViews
		if err := rows2.Scan(&dv.Date, &dv.Count); err != nil {
			return nil, fmt.Errorf("scanning daily views: %w", err)
		}
		stats.RecentViews = append(stats.RecentViews, dv)
	}

	return stats, nil
}

// --- Passkeys (WebAuthn) ---

// Passkey represents a stored WebAuthn credential.
type Passkey struct {
	ID              int64     `json:"id"`
	Name            string    `json:"name"`
	CredentialID    []byte    `json:"-"`
	PublicKey       []byte    `json:"-"`
	AttestationType string    `json:"-"`
	AAGUID          []byte    `json:"-"`
	SignCount       uint32    `json:"-"`
	Transports      []string  `json:"-"`
	CreatedAt       time.Time `json:"created_at"`
}

// AddPasskey saves a new passkey credential.
func (s *Store) AddPasskey(ctx context.Context, p *Passkey) error {
	transportsJSON, _ := json.Marshal(p.Transports)
	result, err := s.db.ExecContext(ctx,
		"INSERT INTO passkeys (name, credential_id, public_key, attestation_type, aaguid, sign_count, transports) VALUES (?, ?, ?, ?, ?, ?, ?)",
		p.Name, p.CredentialID, p.PublicKey, p.AttestationType, p.AAGUID, p.SignCount, string(transportsJSON),
	)
	if err != nil {
		return fmt.Errorf("inserting passkey: %w", err)
	}
	id, _ := result.LastInsertId()
	p.ID = id
	p.CreatedAt = time.Now()
	return nil
}

// AllPasskeys returns all stored passkeys (for admin display and WebAuthn).
func (s *Store) AllPasskeys(ctx context.Context) ([]Passkey, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT id, name, credential_id, public_key, attestation_type, aaguid, sign_count, transports, created_at FROM passkeys ORDER BY created_at ASC",
	)
	if err != nil {
		return nil, fmt.Errorf("querying passkeys: %w", err)
	}
	defer rows.Close()

	var passkeys []Passkey
	for rows.Next() {
		var p Passkey
		var transportsJSON string
		if err := rows.Scan(&p.ID, &p.Name, &p.CredentialID, &p.PublicKey, &p.AttestationType, &p.AAGUID, &p.SignCount, &transportsJSON, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning passkey: %w", err)
		}
		_ = json.Unmarshal([]byte(transportsJSON), &p.Transports)
		passkeys = append(passkeys, p)
	}
	return passkeys, rows.Err()
}

// UpdatePasskeySignCount updates the sign count after a successful authentication.
func (s *Store) UpdatePasskeySignCount(ctx context.Context, credentialID []byte, signCount uint32) error {
	_, err := s.db.ExecContext(ctx, "UPDATE passkeys SET sign_count = ? WHERE credential_id = ?", signCount, credentialID)
	return err
}

// DeletePasskey removes a passkey by ID.
func (s *Store) DeletePasskey(ctx context.Context, id int64) error {
	result, err := s.db.ExecContext(ctx, "DELETE FROM passkeys WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("deleting passkey %d: %w", id, err)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("passkey %d not found", id)
	}
	return nil
}

// --- Newsletter Subscribers ---

// Subscriber represents a newsletter subscriber.
type Subscriber struct {
	ID        int64     `json:"id"`
	Email     string    `json:"email"`
	Confirmed bool      `json:"confirmed"`
	Token     string    `json:"-"`
	CreatedAt time.Time `json:"created_at"`
}

// AddSubscriber adds a new subscriber with a confirmation token.
func (s *Store) AddSubscriber(ctx context.Context, email, token string) error {
	_, err := s.db.ExecContext(ctx,
		"INSERT OR IGNORE INTO subscribers (email, token) VALUES (?, ?)",
		email, token,
	)
	return err
}

// ConfirmSubscriber marks a subscriber as confirmed by token.
func (s *Store) ConfirmSubscriber(ctx context.Context, token string) error {
	result, err := s.db.ExecContext(ctx,
		"UPDATE subscribers SET confirmed = 1 WHERE token = ?", token,
	)
	if err != nil {
		return err
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("invalid or expired token")
	}
	return nil
}

// RemoveSubscriber unsubscribes by token.
func (s *Store) RemoveSubscriber(ctx context.Context, token string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM subscribers WHERE token = ?", token)
	return err
}

// ConfirmedSubscribers returns all confirmed subscriber emails.
func (s *Store) ConfirmedSubscribers(ctx context.Context) ([]Subscriber, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT id, email, token, created_at FROM subscribers WHERE confirmed = 1 ORDER BY created_at ASC",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var subs []Subscriber
	for rows.Next() {
		var sub Subscriber
		if err := rows.Scan(&sub.ID, &sub.Email, &sub.Token, &sub.CreatedAt); err != nil {
			return nil, err
		}
		sub.Confirmed = true
		subs = append(subs, sub)
	}
	return subs, rows.Err()
}

// SubscriberCount returns the number of confirmed subscribers.
func (s *Store) SubscriberCount(ctx context.Context) (int64, error) {
	var count int64
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM subscribers WHERE confirmed = 1").Scan(&count)
	return count, err
}

// AllSubscribers returns all subscribers (confirmed and unconfirmed) for admin display.
func (s *Store) AllSubscribers(ctx context.Context) ([]Subscriber, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT id, email, confirmed, token, created_at FROM subscribers ORDER BY created_at DESC",
	)
	if err != nil {
		return nil, fmt.Errorf("querying all subscribers: %w", err)
	}
	defer rows.Close()
	var subs []Subscriber
	for rows.Next() {
		var sub Subscriber
		if err := rows.Scan(&sub.ID, &sub.Email, &sub.Confirmed, &sub.Token, &sub.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning subscriber: %w", err)
		}
		subs = append(subs, sub)
	}
	return subs, rows.Err()
}

// DeleteSubscriber removes a subscriber by ID.
func (s *Store) DeleteSubscriber(ctx context.Context, id int64) error {
	result, err := s.db.ExecContext(ctx, "DELETE FROM subscribers WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("deleting subscriber %d: %w", id, err)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("subscriber %d not found", id)
	}
	return nil
}

// --- Newsletters ---

// Newsletter represents a newsletter campaign.
type Newsletter struct {
	ID          int64      `json:"id"`
	Subject     string     `json:"subject"`
	Body        string     `json:"body"`
	Status      string     `json:"status"` // draft, scheduled, sending, sent, failed
	ScheduledAt *time.Time `json:"scheduled_at,omitempty"`
	SentAt      *time.Time `json:"sent_at,omitempty"`
	SentCount   int        `json:"sent_count"`
	FailCount   int        `json:"fail_count"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// CreateNewsletter inserts a new newsletter draft.
func (s *Store) CreateNewsletter(ctx context.Context, subject, body string) (*Newsletter, error) {
	result, err := s.db.ExecContext(ctx,
		"INSERT INTO newsletters (subject, body) VALUES (?, ?)",
		subject, body,
	)
	if err != nil {
		return nil, fmt.Errorf("inserting newsletter: %w", err)
	}
	id, _ := result.LastInsertId()
	now := time.Now()
	return &Newsletter{
		ID:        id,
		Subject:   subject,
		Body:      body,
		Status:    "draft",
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// GetNewsletter returns a single newsletter by ID.
func (s *Store) GetNewsletter(ctx context.Context, id int64) (*Newsletter, error) {
	n := &Newsletter{}
	var scheduledAt, sentAt sql.NullTime
	err := s.db.QueryRowContext(ctx,
		"SELECT id, subject, body, status, scheduled_at, sent_at, sent_count, fail_count, created_at, updated_at FROM newsletters WHERE id = ?",
		id,
	).Scan(&n.ID, &n.Subject, &n.Body, &n.Status, &scheduledAt, &sentAt, &n.SentCount, &n.FailCount, &n.CreatedAt, &n.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("querying newsletter %d: %w", id, err)
	}
	if scheduledAt.Valid {
		n.ScheduledAt = &scheduledAt.Time
	}
	if sentAt.Valid {
		n.SentAt = &sentAt.Time
	}
	return n, nil
}

// AllNewsletters returns all newsletters ordered by most recent first.
func (s *Store) AllNewsletters(ctx context.Context) ([]Newsletter, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT id, subject, body, status, scheduled_at, sent_at, sent_count, fail_count, created_at, updated_at FROM newsletters ORDER BY created_at DESC",
	)
	if err != nil {
		return nil, fmt.Errorf("querying newsletters: %w", err)
	}
	defer rows.Close()

	var newsletters []Newsletter
	for rows.Next() {
		var n Newsletter
		var scheduledAt, sentAt sql.NullTime
		if err := rows.Scan(&n.ID, &n.Subject, &n.Body, &n.Status, &scheduledAt, &sentAt, &n.SentCount, &n.FailCount, &n.CreatedAt, &n.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning newsletter: %w", err)
		}
		if scheduledAt.Valid {
			n.ScheduledAt = &scheduledAt.Time
		}
		if sentAt.Valid {
			n.SentAt = &sentAt.Time
		}
		newsletters = append(newsletters, n)
	}
	return newsletters, rows.Err()
}

// UpdateNewsletter updates a newsletter's subject and body (only drafts).
func (s *Store) UpdateNewsletter(ctx context.Context, id int64, subject, body string) error {
	result, err := s.db.ExecContext(ctx,
		"UPDATE newsletters SET subject = ?, body = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ? AND status IN ('draft', 'scheduled')",
		subject, body, id,
	)
	if err != nil {
		return fmt.Errorf("updating newsletter %d: %w", id, err)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("newsletter %d not found or already sent", id)
	}
	return nil
}

// ScheduleNewsletter sets a newsletter to be sent at a specific time.
func (s *Store) ScheduleNewsletter(ctx context.Context, id int64, scheduledAt time.Time) error {
	result, err := s.db.ExecContext(ctx,
		"UPDATE newsletters SET status = 'scheduled', scheduled_at = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ? AND status IN ('draft', 'scheduled')",
		scheduledAt, id,
	)
	if err != nil {
		return fmt.Errorf("scheduling newsletter %d: %w", id, err)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("newsletter %d not found or already sent", id)
	}
	return nil
}

// UnscheduleNewsletter moves a scheduled newsletter back to draft.
func (s *Store) UnscheduleNewsletter(ctx context.Context, id int64) error {
	result, err := s.db.ExecContext(ctx,
		"UPDATE newsletters SET status = 'draft', scheduled_at = NULL, updated_at = CURRENT_TIMESTAMP WHERE id = ? AND status = 'scheduled'",
		id,
	)
	if err != nil {
		return fmt.Errorf("unscheduling newsletter %d: %w", id, err)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("newsletter %d not found or not scheduled", id)
	}
	return nil
}

// MarkNewsletterSending marks a newsletter as currently being sent.
// Returns error if the newsletter is not in a sendable state (prevents double-send).
func (s *Store) MarkNewsletterSending(ctx context.Context, id int64) error {
	result, err := s.db.ExecContext(ctx,
		"UPDATE newsletters SET status = 'sending', updated_at = CURRENT_TIMESTAMP WHERE id = ? AND status IN ('draft', 'scheduled')",
		id,
	)
	if err != nil {
		return err
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("newsletter %d is not in a sendable state (already sending or sent)", id)
	}
	return nil
}

// MarkNewsletterSent marks a newsletter as sent with final counts.
func (s *Store) MarkNewsletterSent(ctx context.Context, id int64, sentCount, failCount int) error {
	_, err := s.db.ExecContext(ctx,
		"UPDATE newsletters SET status = 'sent', sent_at = CURRENT_TIMESTAMP, sent_count = ?, fail_count = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?",
		sentCount, failCount, id,
	)
	return err
}

// MarkNewsletterFailed marks a newsletter as failed.
func (s *Store) MarkNewsletterFailed(ctx context.Context, id int64, sentCount, failCount int) error {
	_, err := s.db.ExecContext(ctx,
		"UPDATE newsletters SET status = 'failed', sent_count = ?, fail_count = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?",
		sentCount, failCount, id,
	)
	return err
}

// --- Per-subscriber newsletter send tracking ---

// RecordNewsletterSend records the send status for a specific subscriber.
func (s *Store) RecordNewsletterSend(ctx context.Context, newsletterID, subscriberID int64, status, errorMsg string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO newsletter_sends (newsletter_id, subscriber_id, status, error_msg, sent_at)
		 VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
		 ON CONFLICT(newsletter_id, subscriber_id) DO UPDATE SET
		   status=excluded.status, error_msg=excluded.error_msg, sent_at=excluded.sent_at`,
		newsletterID, subscriberID, status, errorMsg,
	)
	return err
}

// FailedNewsletterRecipients returns subscribers who failed for a given newsletter.
func (s *Store) FailedNewsletterRecipients(ctx context.Context, newsletterID int64) ([]Subscriber, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT s.id, s.email, s.token, s.created_at
		 FROM newsletter_sends ns
		 JOIN subscribers s ON s.id = ns.subscriber_id
		 WHERE ns.newsletter_id = ? AND ns.status = 'failed' AND s.confirmed = 1
		 ORDER BY s.id ASC`,
		newsletterID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var subs []Subscriber
	for rows.Next() {
		var sub Subscriber
		if err := rows.Scan(&sub.ID, &sub.Email, &sub.Token, &sub.CreatedAt); err != nil {
			return nil, err
		}
		sub.Confirmed = true
		subs = append(subs, sub)
	}
	return subs, rows.Err()
}

// PendingNewsletterRecipients returns subscribers who haven't been sent a newsletter yet.
func (s *Store) PendingNewsletterRecipients(ctx context.Context, newsletterID int64) ([]Subscriber, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT s.id, s.email, s.token, s.created_at
		 FROM subscribers s
		 WHERE s.confirmed = 1
		   AND s.id NOT IN (SELECT subscriber_id FROM newsletter_sends WHERE newsletter_id = ? AND status = 'sent')
		 ORDER BY s.id ASC`,
		newsletterID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var subs []Subscriber
	for rows.Next() {
		var sub Subscriber
		if err := rows.Scan(&sub.ID, &sub.Email, &sub.Token, &sub.CreatedAt); err != nil {
			return nil, err
		}
		sub.Confirmed = true
		subs = append(subs, sub)
	}
	return subs, rows.Err()
}

// DueNewsletters returns scheduled newsletters that are due to be sent.
func (s *Store) DueNewsletters(ctx context.Context) ([]Newsletter, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT id, subject, body, status, scheduled_at, sent_at, sent_count, fail_count, created_at, updated_at FROM newsletters WHERE status = 'scheduled' AND scheduled_at <= CURRENT_TIMESTAMP",
	)
	if err != nil {
		return nil, fmt.Errorf("querying due newsletters: %w", err)
	}
	defer rows.Close()

	var newsletters []Newsletter
	for rows.Next() {
		var n Newsletter
		var scheduledAt, sentAt sql.NullTime
		if err := rows.Scan(&n.ID, &n.Subject, &n.Body, &n.Status, &scheduledAt, &sentAt, &n.SentCount, &n.FailCount, &n.CreatedAt, &n.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning due newsletter: %w", err)
		}
		if scheduledAt.Valid {
			n.ScheduledAt = &scheduledAt.Time
		}
		if sentAt.Valid {
			n.SentAt = &sentAt.Time
		}
		newsletters = append(newsletters, n)
	}
	return newsletters, rows.Err()
}

// DeleteNewsletter removes a newsletter by ID.
func (s *Store) DeleteNewsletter(ctx context.Context, id int64) error {
	result, err := s.db.ExecContext(ctx, "DELETE FROM newsletters WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("deleting newsletter %d: %w", id, err)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("newsletter %d not found", id)
	}
	return nil
}

// --- Post Revision History ---

// PostRevision represents a saved version of a post.
type PostRevision struct {
	ID        int64     `json:"id"`
	PostSlug  string    `json:"post_slug"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	Metadata  string    `json:"metadata"` // JSON: tags, description, image, draft, featured
	CreatedAt time.Time `json:"created_at"`
}

// SavePostRevision stores a snapshot of a post before it is modified.
func (s *Store) SavePostRevision(ctx context.Context, slug, title, content, metadata string) error {
	_, err := s.db.ExecContext(ctx,
		"INSERT INTO post_revisions (post_slug, title, content, metadata) VALUES (?, ?, ?, ?)",
		slug, title, content, metadata,
	)
	if err != nil {
		return fmt.Errorf("saving revision for %q: %w", slug, err)
	}
	return nil
}

// PostRevisions returns all revisions for a given post, newest first.
func (s *Store) PostRevisions(ctx context.Context, slug string) ([]PostRevision, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT id, post_slug, title, content, metadata, created_at FROM post_revisions WHERE post_slug = ? ORDER BY created_at DESC",
		slug,
	)
	if err != nil {
		return nil, fmt.Errorf("querying revisions for %q: %w", slug, err)
	}
	defer rows.Close()

	var revisions []PostRevision
	for rows.Next() {
		var r PostRevision
		if err := rows.Scan(&r.ID, &r.PostSlug, &r.Title, &r.Content, &r.Metadata, &r.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning revision: %w", err)
		}
		revisions = append(revisions, r)
	}
	return revisions, rows.Err()
}

// GetPostRevision returns a specific revision by ID.
func (s *Store) GetPostRevision(ctx context.Context, id int64) (*PostRevision, error) {
	var r PostRevision
	err := s.db.QueryRowContext(ctx,
		"SELECT id, post_slug, title, content, metadata, created_at FROM post_revisions WHERE id = ?", id,
	).Scan(&r.ID, &r.PostSlug, &r.Title, &r.Content, &r.Metadata, &r.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("querying revision %d: %w", id, err)
	}
	return &r, nil
}

// --- Newsletter Analytics ---

// RecordNewsletterEvent logs an open or click event.
func (s *Store) RecordNewsletterEvent(ctx context.Context, newsletterID, subscriberID int64, eventType, url string) error {
	_, err := s.db.ExecContext(ctx,
		"INSERT INTO newsletter_events (newsletter_id, subscriber_id, event_type, url) VALUES (?, ?, ?, ?)",
		newsletterID, subscriberID, eventType, url,
	)
	return err
}

// NewsletterAnalytics holds aggregated stats for a newsletter.
type NewsletterAnalytics struct {
	NewsletterID int64            `json:"newsletter_id"`
	TotalSent    int              `json:"total_sent"`
	UniqueOpens  int              `json:"unique_opens"`
	TotalOpens   int              `json:"total_opens"`
	UniqueClicks int              `json:"unique_clicks"`
	TotalClicks  int              `json:"total_clicks"`
	OpenRate     float64          `json:"open_rate"`
	ClickRate    float64          `json:"click_rate"`
	TopLinks     []LinkClickStats `json:"top_links"`
}

// LinkClickStats holds click stats for a single link.
type LinkClickStats struct {
	URL    string `json:"url"`
	Clicks int    `json:"clicks"`
}

// GetNewsletterAnalytics returns aggregated analytics for a newsletter.
func (s *Store) GetNewsletterAnalytics(ctx context.Context, newsletterID int64) (*NewsletterAnalytics, error) {
	a := &NewsletterAnalytics{NewsletterID: newsletterID}

	// Get sent count from newsletter
	err := s.db.QueryRowContext(ctx,
		"SELECT sent_count FROM newsletters WHERE id = ?", newsletterID,
	).Scan(&a.TotalSent)
	if err != nil {
		return nil, fmt.Errorf("querying newsletter sent count: %w", err)
	}

	// Unique opens (distinct subscribers who opened)
	s.db.QueryRowContext(ctx,
		"SELECT COUNT(DISTINCT subscriber_id) FROM newsletter_events WHERE newsletter_id = ? AND event_type = 'open'",
		newsletterID,
	).Scan(&a.UniqueOpens)

	// Total opens
	s.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM newsletter_events WHERE newsletter_id = ? AND event_type = 'open'",
		newsletterID,
	).Scan(&a.TotalOpens)

	// Unique clicks
	s.db.QueryRowContext(ctx,
		"SELECT COUNT(DISTINCT subscriber_id) FROM newsletter_events WHERE newsletter_id = ? AND event_type = 'click'",
		newsletterID,
	).Scan(&a.UniqueClicks)

	// Total clicks
	s.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM newsletter_events WHERE newsletter_id = ? AND event_type = 'click'",
		newsletterID,
	).Scan(&a.TotalClicks)

	// Rates
	if a.TotalSent > 0 {
		a.OpenRate = float64(a.UniqueOpens) / float64(a.TotalSent) * 100
		a.ClickRate = float64(a.UniqueClicks) / float64(a.TotalSent) * 100
	}

	// Top links
	rows, err := s.db.QueryContext(ctx,
		"SELECT url, COUNT(*) as clicks FROM newsletter_events WHERE newsletter_id = ? AND event_type = 'click' AND url != '' GROUP BY url ORDER BY clicks DESC LIMIT 10",
		newsletterID,
	)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var l LinkClickStats
			if err := rows.Scan(&l.URL, &l.Clicks); err == nil {
				a.TopLinks = append(a.TopLinks, l)
			}
		}
	}

	return a, nil
}

// --- A/B Testing ---

// CreateNewsletterVariant creates a variant of an existing newsletter.
func (s *Store) CreateNewsletterVariant(ctx context.Context, parentID int64, label, subject, body string) (*Newsletter, error) {
	result, err := s.db.ExecContext(ctx,
		"INSERT INTO newsletters (subject, body, variant_of, variant_label) VALUES (?, ?, ?, ?)",
		subject, body, parentID, label,
	)
	if err != nil {
		return nil, fmt.Errorf("creating variant: %w", err)
	}
	id, _ := result.LastInsertId()
	now := time.Now()
	return &Newsletter{
		ID:        id,
		Subject:   subject,
		Body:      body,
		Status:    "draft",
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// NewsletterVariants returns all variants of a newsletter (including the parent).
func (s *Store) NewsletterVariants(ctx context.Context, parentID int64) ([]Newsletter, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT id, subject, body, status, scheduled_at, sent_at, sent_count, fail_count, created_at, updated_at FROM newsletters WHERE id = ? OR variant_of = ? ORDER BY id ASC",
		parentID, parentID,
	)
	if err != nil {
		return nil, fmt.Errorf("querying variants: %w", err)
	}
	defer rows.Close()

	var variants []Newsletter
	for rows.Next() {
		var n Newsletter
		var scheduledAt, sentAt sql.NullTime
		if err := rows.Scan(&n.ID, &n.Subject, &n.Body, &n.Status, &scheduledAt, &sentAt, &n.SentCount, &n.FailCount, &n.CreatedAt, &n.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning variant: %w", err)
		}
		if scheduledAt.Valid {
			n.ScheduledAt = &scheduledAt.Time
		}
		if sentAt.Valid {
			n.SentAt = &sentAt.Time
		}
		variants = append(variants, n)
	}
	return variants, rows.Err()
}

// --- Draft Sharing ---

// DraftShare represents a secret preview link for a draft post.
type DraftShare struct {
	ID        int64     `json:"id"`
	PostSlug  string    `json:"post_slug"`
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

// CreateDraftShare generates a share link for a draft post.
func (s *Store) CreateDraftShare(ctx context.Context, slug, token string, expiresAt time.Time) (*DraftShare, error) {
	// Delete any existing share for this slug first
	_, _ = s.db.ExecContext(ctx, "DELETE FROM draft_shares WHERE post_slug = ?", slug)
	result, err := s.db.ExecContext(ctx,
		"INSERT INTO draft_shares (post_slug, token, expires_at) VALUES (?, ?, ?)",
		slug, token, expiresAt,
	)
	if err != nil {
		return nil, fmt.Errorf("creating draft share for %q: %w", slug, err)
	}
	id, _ := result.LastInsertId()
	return &DraftShare{ID: id, PostSlug: slug, Token: token, ExpiresAt: expiresAt, CreatedAt: time.Now()}, nil
}

// GetDraftShare looks up a share by token and checks expiry.
func (s *Store) GetDraftShare(ctx context.Context, token string) (*DraftShare, error) {
	var ds DraftShare
	err := s.db.QueryRowContext(ctx,
		"SELECT id, post_slug, token, expires_at, created_at FROM draft_shares WHERE token = ?", token,
	).Scan(&ds.ID, &ds.PostSlug, &ds.Token, &ds.ExpiresAt, &ds.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("draft share not found")
	}
	if time.Now().After(ds.ExpiresAt) {
		return nil, fmt.Errorf("share link expired")
	}
	return &ds, nil
}

// GetDraftShareBySlug returns the active share for a post (for admin display).
func (s *Store) GetDraftShareBySlug(ctx context.Context, slug string) (*DraftShare, error) {
	var ds DraftShare
	err := s.db.QueryRowContext(ctx,
		"SELECT id, post_slug, token, expires_at, created_at FROM draft_shares WHERE post_slug = ? AND expires_at > CURRENT_TIMESTAMP",
		slug,
	).Scan(&ds.ID, &ds.PostSlug, &ds.Token, &ds.ExpiresAt, &ds.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &ds, nil
}

// DeleteDraftShare revokes a draft share link.
func (s *Store) DeleteDraftShare(ctx context.Context, slug string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM draft_shares WHERE post_slug = ?", slug)
	return err
}

// --- Webmentions ---

// Webmention represents a received webmention.
type Webmention struct {
	ID           int64     `json:"id"`
	Source       string    `json:"source"`
	Target       string    `json:"target"`
	PostSlug     string    `json:"post_slug"`
	AuthorName   string    `json:"author_name"`
	AuthorURL    string    `json:"author_url"`
	AuthorAvatar string    `json:"author_avatar"`
	Content      string    `json:"content"`
	MentionType  string    `json:"mention_type"` // mention, reply, like, repost
	Verified     bool      `json:"verified"`
	CreatedAt    time.Time `json:"created_at"`
}

// SaveWebmention upserts a received webmention.
func (s *Store) SaveWebmention(ctx context.Context, wm *Webmention) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO webmentions (source, target, post_slug, author_name, author_url, author_avatar, content, mention_type, verified)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(source, target) DO UPDATE SET
		   author_name=excluded.author_name, author_url=excluded.author_url,
		   author_avatar=excluded.author_avatar, content=excluded.content,
		   mention_type=excluded.mention_type, verified=excluded.verified`,
		wm.Source, wm.Target, wm.PostSlug, wm.AuthorName, wm.AuthorURL, wm.AuthorAvatar, wm.Content, wm.MentionType, wm.Verified,
	)
	return err
}

// WebmentionsByPost returns verified webmentions for a post.
func (s *Store) WebmentionsByPost(ctx context.Context, slug string) ([]Webmention, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT id, source, target, post_slug, author_name, author_url, author_avatar, content, mention_type, verified, created_at FROM webmentions WHERE post_slug = ? AND verified = 1 ORDER BY created_at ASC",
		slug,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var wms []Webmention
	for rows.Next() {
		var wm Webmention
		if err := rows.Scan(&wm.ID, &wm.Source, &wm.Target, &wm.PostSlug, &wm.AuthorName, &wm.AuthorURL, &wm.AuthorAvatar, &wm.Content, &wm.MentionType, &wm.Verified, &wm.CreatedAt); err != nil {
			return nil, err
		}
		wms = append(wms, wm)
	}
	return wms, rows.Err()
}

// AllWebmentions returns all webmentions ordered by creation time (newest first).
func (s *Store) AllWebmentions(ctx context.Context) ([]Webmention, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT id, source, target, post_slug, author_name, author_url, author_avatar, content, mention_type, verified, created_at FROM webmentions ORDER BY created_at DESC",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var wms []Webmention
	for rows.Next() {
		var wm Webmention
		if err := rows.Scan(&wm.ID, &wm.Source, &wm.Target, &wm.PostSlug, &wm.AuthorName, &wm.AuthorURL, &wm.AuthorAvatar, &wm.Content, &wm.MentionType, &wm.Verified, &wm.CreatedAt); err != nil {
			return nil, err
		}
		wms = append(wms, wm)
	}
	return wms, rows.Err()
}

// DeleteWebmention deletes a webmention by ID.
func (s *Store) DeleteWebmention(ctx context.Context, id int64) error {
	result, err := s.db.ExecContext(ctx, "DELETE FROM webmentions WHERE id = ?", id)
	if err != nil {
		return err
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("webmention %d not found", id)
	}
	return nil
}

// WebmentionCount returns the number of verified webmentions.
func (s *Store) WebmentionCount(ctx context.Context) (int64, error) {
	var count int64
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM webmentions WHERE verified = 1").Scan(&count)
	return count, err
}

// RemoveActivityPubFollowerByID removes a follower by ID.
func (s *Store) RemoveActivityPubFollowerByID(ctx context.Context, id int64) error {
	result, err := s.db.ExecContext(ctx, "DELETE FROM activitypub_followers WHERE id = ?", id)
	if err != nil {
		return err
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("follower %d not found", id)
	}
	return nil
}

// --- ActivityPub Followers ---

// ActivityPubFollower represents a follower from the fediverse.
type ActivityPubFollower struct {
	ID             int64     `json:"id"`
	ActorURL       string    `json:"actor_url"`
	InboxURL       string    `json:"inbox_url"`
	SharedInboxURL string    `json:"shared_inbox_url"`
	Name           string    `json:"name"`
	CreatedAt      time.Time `json:"created_at"`
}

// AddActivityPubFollower adds a fediverse follower.
func (s *Store) AddActivityPubFollower(ctx context.Context, actorURL, inboxURL, sharedInboxURL, name string) error {
	_, err := s.db.ExecContext(ctx,
		"INSERT OR IGNORE INTO activitypub_followers (actor_url, inbox_url, shared_inbox_url, name) VALUES (?, ?, ?, ?)",
		actorURL, inboxURL, sharedInboxURL, name,
	)
	return err
}

// RemoveActivityPubFollower removes a fediverse follower.
func (s *Store) RemoveActivityPubFollower(ctx context.Context, actorURL string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM activitypub_followers WHERE actor_url = ?", actorURL)
	return err
}

// AllActivityPubFollowers returns all followers for outbox delivery.
func (s *Store) AllActivityPubFollowers(ctx context.Context) ([]ActivityPubFollower, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT id, actor_url, inbox_url, shared_inbox_url, name, created_at FROM activitypub_followers ORDER BY created_at ASC",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var followers []ActivityPubFollower
	for rows.Next() {
		var f ActivityPubFollower
		if err := rows.Scan(&f.ID, &f.ActorURL, &f.InboxURL, &f.SharedInboxURL, &f.Name, &f.CreatedAt); err != nil {
			return nil, err
		}
		followers = append(followers, f)
	}
	return followers, rows.Err()
}

// ActivityPubFollowerCount returns the number of followers.
func (s *Store) ActivityPubFollowerCount(ctx context.Context) (int64, error) {
	var count int64
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM activitypub_followers").Scan(&count)
	return count, err
}

// --- Webhooks ---

// Webhook represents a registered webhook endpoint.
type Webhook struct {
	ID        int64     `json:"id"`
	Event     string    `json:"event"`
	URL       string    `json:"url"`
	Secret    string    `json:"secret,omitempty"`
	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"created_at"`
}

// AddWebhook registers a new webhook.
func (s *Store) AddWebhook(ctx context.Context, event, url, secret string) (*Webhook, error) {
	result, err := s.db.ExecContext(ctx,
		"INSERT INTO webhooks (event, url, secret) VALUES (?, ?, ?)",
		event, url, secret,
	)
	if err != nil {
		return nil, fmt.Errorf("inserting webhook: %w", err)
	}
	id, _ := result.LastInsertId()
	return &Webhook{ID: id, Event: event, URL: url, Secret: secret, Active: true, CreatedAt: time.Now()}, nil
}

// AllWebhooks returns all webhooks.
func (s *Store) AllWebhooks(ctx context.Context) ([]Webhook, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT id, event, url, secret, active, created_at FROM webhooks ORDER BY created_at ASC",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var hooks []Webhook
	for rows.Next() {
		var h Webhook
		if err := rows.Scan(&h.ID, &h.Event, &h.URL, &h.Secret, &h.Active, &h.CreatedAt); err != nil {
			return nil, err
		}
		hooks = append(hooks, h)
	}
	return hooks, rows.Err()
}

// WebhooksByEvent returns active webhooks for a specific event.
func (s *Store) WebhooksByEvent(ctx context.Context, event string) ([]Webhook, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT id, event, url, secret, active, created_at FROM webhooks WHERE event = ? AND active = 1",
		event,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var hooks []Webhook
	for rows.Next() {
		var h Webhook
		if err := rows.Scan(&h.ID, &h.Event, &h.URL, &h.Secret, &h.Active, &h.CreatedAt); err != nil {
			return nil, err
		}
		hooks = append(hooks, h)
	}
	return hooks, rows.Err()
}

// DeleteWebhook removes a webhook by ID.
func (s *Store) DeleteWebhook(ctx context.Context, id int64) error {
	result, err := s.db.ExecContext(ctx, "DELETE FROM webhooks WHERE id = ?", id)
	if err != nil {
		return err
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("webhook %d not found", id)
	}
	return nil
}

// ToggleWebhook enables or disables a webhook.
func (s *Store) ToggleWebhook(ctx context.Context, id int64, active bool) error {
	activeInt := 0
	if active {
		activeInt = 1
	}
	_, err := s.db.ExecContext(ctx, "UPDATE webhooks SET active = ? WHERE id = ?", activeInt, id)
	return err
}

// --- API Tokens ---

// APIToken represents an API access token for headless CMS usage.
type APIToken struct {
	ID         int64      `json:"id"`
	Name       string     `json:"name"`
	TokenHash  string     `json:"-"`
	Scopes     string     `json:"scopes"` // comma-separated: read, write, admin
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

// AddAPIToken stores a new API token (hashed).
func (s *Store) AddAPIToken(ctx context.Context, name, tokenHash, scopes string) (*APIToken, error) {
	result, err := s.db.ExecContext(ctx,
		"INSERT INTO api_tokens (name, token_hash, scopes) VALUES (?, ?, ?)",
		name, tokenHash, scopes,
	)
	if err != nil {
		return nil, fmt.Errorf("inserting API token: %w", err)
	}
	id, _ := result.LastInsertId()
	return &APIToken{ID: id, Name: name, Scopes: scopes, CreatedAt: time.Now()}, nil
}

// ValidateAPIToken checks a token against stored hashes. Returns the token if valid.
func (s *Store) ValidateAPIToken(ctx context.Context, tokenHash string) (*APIToken, error) {
	var t APIToken
	var lastUsed sql.NullTime
	err := s.db.QueryRowContext(ctx,
		"SELECT id, name, token_hash, scopes, last_used_at, created_at FROM api_tokens WHERE token_hash = ?",
		tokenHash,
	).Scan(&t.ID, &t.Name, &t.TokenHash, &t.Scopes, &lastUsed, &t.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("invalid token")
	}
	if lastUsed.Valid {
		t.LastUsedAt = &lastUsed.Time
	}
	// Update last used
	_, _ = s.db.ExecContext(ctx, "UPDATE api_tokens SET last_used_at = CURRENT_TIMESTAMP WHERE id = ?", t.ID)
	return &t, nil
}

// AllAPITokens returns all tokens (for admin display).
func (s *Store) AllAPITokens(ctx context.Context) ([]APIToken, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT id, name, scopes, last_used_at, created_at FROM api_tokens ORDER BY created_at DESC",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tokens []APIToken
	for rows.Next() {
		var t APIToken
		var lastUsed sql.NullTime
		if err := rows.Scan(&t.ID, &t.Name, &t.Scopes, &lastUsed, &t.CreatedAt); err != nil {
			return nil, err
		}
		if lastUsed.Valid {
			t.LastUsedAt = &lastUsed.Time
		}
		tokens = append(tokens, t)
	}
	return tokens, rows.Err()
}

// DeleteAPIToken removes a token by ID.
func (s *Store) DeleteAPIToken(ctx context.Context, id int64) error {
	result, err := s.db.ExecContext(ctx, "DELETE FROM api_tokens WHERE id = ?", id)
	if err != nil {
		return err
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("token %d not found", id)
	}
	return nil
}

// --- Audit Log ---

// AuditEntry represents a single audit log event.
type AuditEntry struct {
	ID        int64     `json:"id"`
	Event     string    `json:"event"`
	Detail    string    `json:"detail"`
	IP        string    `json:"ip"`
	CreatedAt time.Time `json:"created_at"`
}

// LogAuditEvent records an auditable admin action.
func (s *Store) LogAuditEvent(ctx context.Context, event, detail, ip string) error {
	_, err := s.db.ExecContext(ctx,
		"INSERT INTO audit_log (event, detail, ip) VALUES (?, ?, ?)",
		event, detail, ip,
	)
	return err
}

// AuditLogs returns recent audit entries, newest first. Limit controls how many.
func (s *Store) AuditLogs(ctx context.Context, limit int) ([]AuditEntry, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	rows, err := s.db.QueryContext(ctx,
		"SELECT id, event, detail, ip, created_at FROM audit_log ORDER BY created_at DESC LIMIT ?", limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var entries []AuditEntry
	for rows.Next() {
		var e AuditEntry
		if err := rows.Scan(&e.ID, &e.Event, &e.Detail, &e.IP, &e.CreatedAt); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// CleanupAuditLog deletes entries older than the given duration,
// but never deletes entries newer than 24 hours.
func (s *Store) CleanupAuditLog(ctx context.Context, maxAge time.Duration) (int64, error) {
	cutoff := time.Now().Add(-maxAge)
	minCutoff := time.Now().Add(-24 * time.Hour)
	if cutoff.After(minCutoff) {
		cutoff = minCutoff
	}
	result, err := s.db.ExecContext(ctx,
		"DELETE FROM audit_log WHERE created_at < ?", cutoff,
	)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
