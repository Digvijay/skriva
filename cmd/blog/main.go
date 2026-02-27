// Package main is the entry point for the Skriva blog engine.
// It supports two subcommands:
//   - serve: Start the HTTP server
//   - import ghost: Import posts from a Ghost JSON export
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/Digvijay/skriva/internal/config"
	"github.com/Digvijay/skriva/internal/content"
	"github.com/Digvijay/skriva/internal/importer"
	"github.com/Digvijay/skriva/internal/render"
	"github.com/Digvijay/skriva/internal/server"
	"github.com/Digvijay/skriva/internal/store"

	blog "github.com/Digvijay/skriva"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	cmd := "serve"
	if len(args) > 0 {
		cmd = args[0]
	}

	switch cmd {
	case "serve":
		return runServe()
	case "version":
		fmt.Printf("Skriva v%s\n", blog.Version)
		return nil
	case "import":
		if len(args) < 2 {
			return fmt.Errorf("usage: blog import <ghost|wordpress> --export <file> [--images <dir>]")
		}
		return runImport(args[1:])
	case "migrate":
		if len(args) < 2 {
			return fmt.Errorf("usage: blog migrate <status|rollback>")
		}
		return runMigrate(args[1:])
	default:
		return fmt.Errorf("unknown command %q; use 'serve', 'version', 'import', or 'migrate'", cmd)
	}
}

func runServe() error {
	// Setup structured logging
	logLevel := envOrDefault("BLOG_LOG_LEVEL", "info")
	logger := setupLogger(logLevel)

	// Load configuration
	configDir := envOrDefault("BLOG_CONFIG_DIR", "/data/config")
	contentDir := envOrDefault("BLOG_CONTENT_DIR", "/data/content")
	port := envOrDefault("BLOG_PORT", "8080")

	logger.Info("starting Skriva",
		"version", blog.Version,
		"config_dir", configDir,
		"content_dir", contentDir,
		"port", port,
	)

	cfg, err := config.Load(configDir)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Initialize SQLite store
	db, err := store.New(configDir, logger)
	if err != nil {
		return fmt.Errorf("opening store: %w", err)
	}
	defer func() { _ = db.Close() }()

	// Initialize content loader
	loader, err := content.NewLoader(contentDir, logger)
	if err != nil {
		return fmt.Errorf("loading content: %w", err)
	}

	// Initialize render engine
	engine, err := render.NewEngine(contentDir, cfg, logger, blog.EmbeddedThemes)
	if err != nil {
		return fmt.Errorf("initializing render engine: %w", err)
	}

	// Create and start server
	srv := server.New(cfg, loader, db, engine, logger, port, contentDir, configDir)

	// Graceful shutdown on SIGTERM/SIGINT
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	return srv.ListenAndServe(ctx)
}

func runImport(args []string) error {
	logger := setupLogger("info")

	if len(args) == 0 {
		return fmt.Errorf("usage: blog import <ghost|wordpress> --export <file> [--images <dir>]")
	}

	platform := args[0]

	// Parse flags
	var exportPath, imagesDir string
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--export":
			if i+1 < len(args) {
				exportPath = args[i+1]
				i++
			}
		case "--images":
			if i+1 < len(args) {
				imagesDir = args[i+1]
				i++
			}
		}
	}

	if exportPath == "" {
		return fmt.Errorf("--export flag is required: blog import %s --export <file>", platform)
	}

	contentDir := envOrDefault("BLOG_CONTENT_DIR", "/data/content")

	switch platform {
	case "ghost":
		logger.Info("starting Ghost import",
			"export", exportPath,
			"images", imagesDir,
			"output", contentDir,
		)

		imp := importer.NewGhostImporter(exportPath, imagesDir, contentDir, logger)
		result, err := imp.Run()
		if err != nil {
			return fmt.Errorf("import failed: %w", err)
		}

		logger.Info("import complete",
			"found", result.TotalFound,
			"imported", result.Imported,
			"skipped", result.Skipped,
			"images", result.ImagesCopied,
			"errors", len(result.Errors),
		)

		for _, e := range result.Errors {
			logger.Warn("import error", "detail", e)
		}

	case "wordpress":
		logger.Info("starting WordPress import",
			"export", exportPath,
			"output", contentDir,
		)

		if err := importer.ImportWordPress(exportPath, contentDir, logger); err != nil {
			return fmt.Errorf("WordPress import failed: %w", err)
		}

	default:
		return fmt.Errorf("unknown import platform %q; use 'ghost' or 'wordpress'", platform)
	}

	return nil
}

func setupLogger(level string) *slog.Logger {
	var logLevel slog.Level
	switch level {
	case "debug":
		logLevel = slog.LevelDebug
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	})
	return slog.New(handler)
}

func envOrDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func runMigrate(args []string) error {
	logger := setupLogger("info")
	configDir := envOrDefault("BLOG_CONFIG_DIR", "/data/config")

	db, err := store.New(configDir, logger)
	if err != nil {
		return fmt.Errorf("opening store: %w", err)
	}
	defer func() { _ = db.Close() }()

	switch args[0] {
	case "status":
		infos, err := db.MigrationStatus()
		if err != nil {
			return fmt.Errorf("getting migration status: %w", err)
		}
		fmt.Printf("%-8s %-30s %-10s %s\n", "VERSION", "NAME", "STATUS", "APPLIED AT")
		fmt.Println(strings.Repeat("-", 80))
		for _, info := range infos {
			status := "pending"
			appliedAt := ""
			if info.Applied {
				status = "applied"
				appliedAt = info.AppliedAt
			}
			rollback := ""
			if info.HasDown {
				rollback = " (rollback available)"
			}
			fmt.Printf("%-8d %-30s %-10s %s%s\n", info.Version, info.Name, status, appliedAt, rollback)
		}
		return nil

	case "rollback":
		return db.RollbackMigration()

	default:
		return fmt.Errorf("unknown migrate subcommand %q; use 'status' or 'rollback'", args[0])
	}
}
