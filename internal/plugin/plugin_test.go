package plugin

import (
	"io"
	"log/slog"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestNewHookRegistry(t *testing.T) {
	r := NewHookRegistry(testLogger())
	if r == nil {
		t.Fatal("NewHookRegistry returned nil")
	}
	if r.HookCount() != 0 {
		t.Errorf("new registry hook count = %d, want 0", r.HookCount())
	}
	if r.RouteCount() != 0 {
		t.Errorf("new registry route count = %d, want 0", r.RouteCount())
	}
}

func TestOnAndFire(t *testing.T) {
	r := NewHookRegistry(testLogger())

	var called int32
	r.On(EventPostCreated, func(event string, data interface{}) {
		atomic.AddInt32(&called, 1)
		if event != EventPostCreated {
			t.Errorf("event = %q, want %q", event, EventPostCreated)
		}
		pe, ok := data.(PostEvent)
		if !ok {
			t.Errorf("data type = %T, want PostEvent", data)
		}
		if pe.Slug != "test-post" {
			t.Errorf("slug = %q, want test-post", pe.Slug)
		}
	})

	if r.HookCount() != 1 {
		t.Errorf("hook count = %d, want 1", r.HookCount())
	}

	r.Fire(EventPostCreated, PostEvent{Slug: "test-post", Title: "Test"})

	if atomic.LoadInt32(&called) != 1 {
		t.Errorf("hook called %d times, want 1", called)
	}
}

func TestFireMultipleHooks(t *testing.T) {
	r := NewHookRegistry(testLogger())

	var count int32
	r.On(EventPostUpdated, func(string, interface{}) { atomic.AddInt32(&count, 1) })
	r.On(EventPostUpdated, func(string, interface{}) { atomic.AddInt32(&count, 1) })
	r.On(EventPostUpdated, func(string, interface{}) { atomic.AddInt32(&count, 1) })

	if r.HookCount() != 3 {
		t.Errorf("hook count = %d, want 3", r.HookCount())
	}

	r.Fire(EventPostUpdated, nil)

	if atomic.LoadInt32(&count) != 3 {
		t.Errorf("hooks fired %d times, want 3", count)
	}
}

func TestFireNoHooks(t *testing.T) {
	r := NewHookRegistry(testLogger())
	// Should not panic
	r.Fire("nonexistent.event", nil)
}

func TestFirePanicRecovery(t *testing.T) {
	r := NewHookRegistry(testLogger())

	var secondCalled int32
	r.On(EventPostDeleted, func(string, interface{}) {
		panic("intentional test panic")
	})
	r.On(EventPostDeleted, func(string, interface{}) {
		atomic.AddInt32(&secondCalled, 1)
	})

	// Should not panic, and the second hook should still run
	r.Fire(EventPostDeleted, nil)

	if atomic.LoadInt32(&secondCalled) != 1 {
		t.Error("second hook should still run after first panics")
	}
}

func TestRegisterRoute(t *testing.T) {
	r := NewHookRegistry(testLogger())

	handler := func(w http.ResponseWriter, req *http.Request) {
		w.Write([]byte("hello"))
	}

	r.RegisterRoute("my-plugin", RouteFunc(handler))

	if r.RouteCount() != 1 {
		t.Errorf("route count = %d, want 1", r.RouteCount())
	}

	routes := r.Routes()
	if _, ok := routes["my-plugin"]; !ok {
		t.Error("registered route not found in Routes()")
	}
}

func TestRoutesCopy(t *testing.T) {
	r := NewHookRegistry(testLogger())
	r.RegisterRoute("a", func(http.ResponseWriter, *http.Request) {})

	routes := r.Routes()
	routes["injected"] = func(http.ResponseWriter, *http.Request) {}

	// Original should not be affected
	if r.RouteCount() != 1 {
		t.Errorf("route count changed after modifying copy: %d", r.RouteCount())
	}
}

func TestRegisterTemplateFunc(t *testing.T) {
	r := NewHookRegistry(testLogger())

	r.RegisterTemplateFunc("greet", func(name string) string {
		return "Hello, " + name + "!"
	})

	funcs := r.TemplateFuncs()
	if len(funcs) != 1 {
		t.Fatalf("template funcs count = %d, want 1", len(funcs))
	}
	if funcs[0].Name != "greet" {
		t.Errorf("func name = %q, want greet", funcs[0].Name)
	}
}

func TestConcurrentAccess(t *testing.T) {
	r := NewHookRegistry(testLogger())
	var wg sync.WaitGroup

	// Concurrent registrations + fires
	for i := 0; i < 100; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			r.On(EventPostCreated, func(string, interface{}) {})
		}()
		go func() {
			defer wg.Done()
			r.Fire(EventPostCreated, nil)
		}()
	}
	wg.Wait()

	if r.HookCount() < 100 {
		t.Errorf("expected at least 100 hooks, got %d", r.HookCount())
	}
}
