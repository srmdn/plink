package server

import (
	"fmt"
	"testing"
)

func TestStoreOGImageCapsCacheFIFO(t *testing.T) {
	s := &Server{}
	for i := 0; i < ogCacheLimit+5; i++ {
		s.storeOGImage(fmt.Sprintf("key-%d", i), []byte{byte(i)})
	}
	if len(s.ogCache) != ogCacheLimit || len(s.ogCacheOrder) != ogCacheLimit {
		t.Fatalf("cache size = %d (order %d), want %d", len(s.ogCache), len(s.ogCacheOrder), ogCacheLimit)
	}
	for i := 0; i < 5; i++ {
		if _, ok := s.ogCache[fmt.Sprintf("key-%d", i)]; ok {
			t.Fatalf("key-%d should have been evicted", i)
		}
	}
	if body, ok := s.ogCache[fmt.Sprintf("key-%d", ogCacheLimit+4)]; !ok || len(body) != 1 {
		t.Fatal("newest key missing from cache")
	}

	// Refreshing an existing key must not grow the cache or change its order.
	s.storeOGImage("key-10", []byte("updated"))
	if body := s.ogCache["key-10"]; string(body) != "updated" || len(s.ogCache) != ogCacheLimit {
		t.Fatal("existing key was not refreshed in place")
	}
	s.storeOGImage("key-new", []byte("new"))
	if _, ok := s.ogCache["key-10"]; !ok {
		t.Fatal("refreshed key was evicted before older entries")
	}
	if _, ok := s.ogCache["key-5"]; ok {
		t.Fatal("oldest key survived after a new insertion")
	}
	if len(s.ogCache) != ogCacheLimit {
		t.Fatalf("cache size after refresh = %d, want %d", len(s.ogCache), ogCacheLimit)
	}
}
