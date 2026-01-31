package peer

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func TestNewPeerCache(t *testing.T) {
	cache := NewPeerCache()
	if cache == nil {
		t.Fatal("NewPeerCache returned nil")
	}
	if cache.peers == nil {
		t.Error("peers map should be initialized")
	}
	if cache.Count() != 0 {
		t.Errorf("new cache should be empty, got %d", cache.Count())
	}
}

func TestPeerCacheUpdate(t *testing.T) {
	cache := NewPeerCache()

	peers := []*Peer{
		{Host: "192.168.1.100", Port: 11313, Version: "0.1.0"},
		{Host: "192.168.1.101", Port: 11313, Version: "0.2.0"},
	}

	cache.Update(peers)

	if cache.Count() != 2 {
		t.Errorf("expected 2 peers, got %d", cache.Count())
	}

	// Update with same host:port should overwrite
	updated := []*Peer{
		{Host: "192.168.1.100", Port: 11313, Version: "0.3.0"},
	}
	cache.Update(updated)

	if cache.Count() != 2 {
		t.Errorf("expected still 2 peers, got %d", cache.Count())
	}
}

func TestPeerCacheGetFresh(t *testing.T) {
	cache := NewPeerCache()

	// Add fresh peer
	cache.Update([]*Peer{
		{Host: "192.168.1.100", Port: 11313, Version: "0.1.0"},
	})

	fresh := cache.GetFresh()
	if len(fresh) != 1 {
		t.Errorf("expected 1 fresh peer, got %d", len(fresh))
	}

	// Manually inject stale peer (past TTL)
	cache.mu.Lock()
	cache.peers["192.168.1.200:11313"] = &CachedPeer{
		Host:     "192.168.1.200",
		Port:     11313,
		Version:  "0.1.0",
		LastSeen: time.Now().Add(-PeerTTL - time.Minute),
	}
	cache.mu.Unlock()

	// Total count should be 2
	if cache.Count() != 2 {
		t.Errorf("expected 2 total peers, got %d", cache.Count())
	}

	// Fresh count should be 1
	if cache.FreshCount() != 1 {
		t.Errorf("expected 1 fresh peer, got %d", cache.FreshCount())
	}

	// GetFresh should return only non-stale
	fresh = cache.GetFresh()
	if len(fresh) != 1 {
		t.Errorf("expected 1 fresh peer after adding stale, got %d", len(fresh))
	}
}

func TestPeerCacheCleanup(t *testing.T) {
	cache := NewPeerCache()

	// Add fresh and stale peers
	cache.mu.Lock()
	cache.peers["192.168.1.100:11313"] = &CachedPeer{
		Host:     "192.168.1.100",
		Port:     11313,
		Version:  "0.1.0",
		LastSeen: time.Now(),
	}
	cache.peers["192.168.1.200:11313"] = &CachedPeer{
		Host:     "192.168.1.200",
		Port:     11313,
		Version:  "0.1.0",
		LastSeen: time.Now().Add(-PeerTTL - time.Minute),
	}
	cache.mu.Unlock()

	if cache.Count() != 2 {
		t.Errorf("expected 2 peers before cleanup, got %d", cache.Count())
	}

	cache.Cleanup()

	if cache.Count() != 1 {
		t.Errorf("expected 1 peer after cleanup, got %d", cache.Count())
	}
}

func TestPeerCacheSaveLoad(t *testing.T) {
	// Use a temp directory
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "peers.json")

	// Create cache and add peers
	cache := NewPeerCache()
	cache.Update([]*Peer{
		{Host: "192.168.1.100", Port: 11313, Version: "0.1.0"},
	})

	// Save to temp file manually (since we can't override CacheFilePath)
	cache.mu.RLock()
	data, err := encodeCache(cache.peers)
	cache.mu.RUnlock()
	if err != nil {
		t.Fatalf("failed to encode: %v", err)
	}
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		t.Fatalf("failed to write: %v", err)
	}

	// Read back and verify
	data2, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("failed to read: %v", err)
	}

	cache2 := NewPeerCache()
	if err := decodeCache(data2, &cache2.peers); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}

	if cache2.Count() != 1 {
		t.Errorf("expected 1 peer after load, got %d", cache2.Count())
	}
}

func TestPeerKey(t *testing.T) {
	tests := []struct {
		host     string
		port     int
		expected string
	}{
		{"192.168.1.100", 11313, "192.168.1.100:11313"},
		{"localhost", 8080, "localhost:8080"},
		{"::1", 443, "::1:443"},
	}

	for _, tt := range tests {
		result := peerKey(tt.host, tt.port)
		if result != tt.expected {
			t.Errorf("peerKey(%s, %d) = %s, want %s", tt.host, tt.port, result, tt.expected)
		}
	}
}

// Helper functions for testing (avoid changing CacheFilePath)
func encodeCache(peers map[string]*CachedPeer) ([]byte, error) {
	return yaml.Marshal(peers)
}

func decodeCache(data []byte, peers *map[string]*CachedPeer) error {
	return yaml.Unmarshal(data, peers)
}
