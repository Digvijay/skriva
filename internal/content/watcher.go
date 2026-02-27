// Package content - watcher provides fsnotify-based content hot-reload.
package content

import (
	"io/fs"
	"log/slog"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Watcher monitors the content directory for changes and triggers reloads.
type Watcher struct {
	loader  *Loader
	logger  *slog.Logger
	watcher *fsnotify.Watcher
	done    chan struct{}
}

// NewWatcher creates a file watcher for the content directory.
func NewWatcher(loader *Loader, logger *slog.Logger) (*Watcher, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	cw := &Watcher{
		loader:  loader,
		logger:  logger,
		watcher: w,
		done:    make(chan struct{}),
	}

	go cw.run()

	// Watch content directory recursively
	if err := cw.addRecursive(loader.ContentDir()); err != nil {
		_ = w.Close()
		return nil, err
	}

	return cw, nil
}

// Close stops the watcher.
func (cw *Watcher) Close() error {
	close(cw.done)
	return cw.watcher.Close()
}

func (cw *Watcher) run() {
	// Debounce: wait for a burst of events to settle before reloading.
	var timer *time.Timer
	debounce := 500 * time.Millisecond

	for {
		select {
		case <-cw.done:
			return
		case event, ok := <-cw.watcher.Events:
			if !ok {
				return
			}

			// Only care about write, create, remove, rename events on .md files or media
			if !isRelevantEvent(event) {
				continue
			}

			cw.logger.Debug("content file changed", "event", event.Op.String(), "path", event.Name)

			// If a new directory was created, watch it too
			if event.Has(fsnotify.Create) {
				_ = cw.addRecursive(event.Name)
			}

			// Debounce: reset timer on each event
			if timer != nil {
				timer.Stop()
			}
			timer = time.AfterFunc(debounce, func() {
				cw.logger.Info("reloading content after file change")
				if err := cw.loader.Reload(); err != nil {
					cw.logger.Error("failed to reload content", "error", err)
				}
			})

		case err, ok := <-cw.watcher.Errors:
			if !ok {
				return
			}
			cw.logger.Error("content watcher error", "error", err)
		}
	}
}

func (cw *Watcher) addRecursive(dir string) error {
	// SECURITY: Use WalkDir (not Walk) to avoid following directory symlinks.
	// filepath.Walk follows symlinks, which could watch directories outside
	// the content tree and exhaust inotify watches.
	return filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if watchErr := cw.watcher.Add(path); watchErr != nil {
				cw.logger.Warn("failed to watch directory", "path", path, "error", watchErr)
			}
		}
		return nil
	})
}

func isRelevantEvent(event fsnotify.Event) bool {
	if event.Has(fsnotify.Chmod) {
		return false
	}

	ext := filepath.Ext(event.Name)
	switch ext {
	case ".md", ".yaml", ".yml", ".html", ".css":
		return true
	}

	// Also trigger on directory changes (new post directories)
	if event.Has(fsnotify.Create) || event.Has(fsnotify.Remove) || event.Has(fsnotify.Rename) {
		return true
	}

	return false
}
