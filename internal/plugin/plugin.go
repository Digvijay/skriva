// Package plugin provides a hook-based plugin system for Skriva.
// Plugins register callbacks for lifecycle events (post-create, post-update,
// post-delete, comment-create, pre-render, custom-route) via a central registry.
//
// Plugins are Go source files in {contentDir}/plugins/ that are loaded at startup.
// Each plugin must implement a Register(registry *HookRegistry) function.
//
// For security, plugins run in-process — they have full access to the blog's
// data and should only be installed from trusted sources.
package plugin

import (
	"log/slog"
	"net/http"
	"sync"
)

// Event types for plugin hooks.
const (
	EventPostCreated    = "post.created"
	EventPostUpdated    = "post.updated"
	EventPostDeleted    = "post.deleted"
	EventCommentCreated = "comment.created"
	EventPreRender      = "pre.render"
)

// PostEvent contains data passed to post lifecycle hooks.
type PostEvent struct {
	Slug  string
	Title string
	Tags  []string
	Draft bool
	// NOTE: ContentDir intentionally excluded — plugins must not know filesystem paths
}

// CommentEvent contains data passed to comment hooks.
type CommentEvent struct {
	PostSlug string
	Author   string
	Content  string
}

// RenderEvent contains data passed to pre-render hooks.
// The plugin can modify the HTML before it's sent to the client.
type RenderEvent struct {
	TemplateName string
	HTML         string
}

// HookFunc is a callback for lifecycle events.
type HookFunc func(event string, data interface{})

// RouteFunc is a handler for custom plugin routes.
type RouteFunc func(w http.ResponseWriter, r *http.Request)

// TemplateFuncEntry is a custom template function registered by a plugin.
type TemplateFuncEntry struct {
	Name string
	Func interface{}
}

// HookRegistry manages plugin hooks, custom routes, and template functions.
type HookRegistry struct {
	mu            sync.RWMutex
	hooks         map[string][]HookFunc
	routes        map[string]RouteFunc // path → handler
	templateFuncs []TemplateFuncEntry
	logger        *slog.Logger
}

// NewHookRegistry creates a new empty plugin registry.
func NewHookRegistry(logger *slog.Logger) *HookRegistry {
	return &HookRegistry{
		hooks:  make(map[string][]HookFunc),
		routes: make(map[string]RouteFunc),
		logger: logger,
	}
}

// On registers a callback for a specific event.
func (r *HookRegistry) On(event string, fn HookFunc) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.hooks[event] = append(r.hooks[event], fn)
	r.logger.Info("plugin hook registered", "event", event)
}

// Fire triggers all callbacks for a given event.
func (r *HookRegistry) Fire(event string, data interface{}) {
	r.mu.RLock()
	hooks := r.hooks[event]
	r.mu.RUnlock()

	for _, fn := range hooks {
		func() {
			defer func() {
				if err := recover(); err != nil {
					r.logger.Error("plugin hook panic", "event", event, "error", err)
				}
			}()
			fn(event, data)
		}()
	}
}

// RegisterRoute adds a custom HTTP route handled by a plugin.
// Routes are mounted under /plugins/{path}.
func (r *HookRegistry) RegisterRoute(path string, handler RouteFunc) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.routes[path] = handler
	r.logger.Info("plugin route registered", "path", "/plugins/"+path)
}

// Routes returns all registered plugin routes.
func (r *HookRegistry) Routes() map[string]RouteFunc {
	r.mu.RLock()
	defer r.mu.RUnlock()
	// Return a copy
	routes := make(map[string]RouteFunc, len(r.routes))
	for k, v := range r.routes {
		routes[k] = v
	}
	return routes
}

// RegisterTemplateFunc adds a custom template function available to themes.
func (r *HookRegistry) RegisterTemplateFunc(name string, fn interface{}) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.templateFuncs = append(r.templateFuncs, TemplateFuncEntry{Name: name, Func: fn})
	r.logger.Info("plugin template function registered", "name", name)
}

// TemplateFuncs returns all registered custom template functions.
func (r *HookRegistry) TemplateFuncs() []TemplateFuncEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]TemplateFuncEntry, len(r.templateFuncs))
	copy(result, r.templateFuncs)
	return result
}

// HookCount returns the total number of registered hooks.
func (r *HookRegistry) HookCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	count := 0
	for _, hooks := range r.hooks {
		count += len(hooks)
	}
	return count
}

// RouteCount returns the number of registered routes.
func (r *HookRegistry) RouteCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.routes)
}
