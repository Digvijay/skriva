package render

import (
	"fmt"
	"testing"
)

func BenchmarkCacheHit(b *testing.B) {
	c := NewPageCache(1000)
	// Pre-populate with a page so every Get is a cache hit.
	c.Set("page-1", "<html>cached page</html>")

	b.ResetTimer()
	for b.Loop() {
		c.Get("page-1")
	}
}

func BenchmarkCacheMiss(b *testing.B) {
	c := NewPageCache(1000)
	// Cache is empty — every Get is a miss.
	b.ResetTimer()
	for b.Loop() {
		c.Get("page-miss")
	}
}

func BenchmarkCacheSet(b *testing.B) {
	c := NewPageCache(1000)
	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		c.Set(fmt.Sprintf("key-%d", i), "<html>page</html>")
	}
}

func BenchmarkCacheEviction(b *testing.B) {
	// Small cache to force evictions on every Set.
	c := NewPageCache(10)
	for i := 0; i < 10; i++ {
		c.Set(fmt.Sprintf("seed-%d", i), "<html>seed</html>")
	}

	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		c.Set(fmt.Sprintf("evict-%d", i), "<html>new page</html>")
	}
}
