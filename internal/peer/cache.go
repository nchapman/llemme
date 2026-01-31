package peer

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/nchapman/lleme/internal/config"
	"gopkg.in/yaml.v3"
)

const (
	// PeerTTL is how long to consider a cached peer valid
	PeerTTL = 15 * time.Minute

	// PollInterval is how often the server polls for new peers
	PollInterval = 2 * time.Minute
)

// CachedPeer represents a peer entry in the cache file
type CachedPeer struct {
	Host     string    `yaml:"host"`
	Port     int       `yaml:"port"`
	Version  string    `yaml:"version"`
	LastSeen time.Time `yaml:"lastSeen"`
}

// PeerCache manages the persistent peer cache file
type PeerCache struct {
	mu    sync.RWMutex
	peers map[string]*CachedPeer // key: "host:port"
}

// CacheFilePath returns the path to the peer cache file
func CacheFilePath() string {
	return filepath.Join(config.CachePath(), "peers.yaml")
}

// NewPeerCache creates a new peer cache
func NewPeerCache() *PeerCache {
	return &PeerCache{
		peers: make(map[string]*CachedPeer),
	}
}

// Load reads the cache from disk
func (c *PeerCache) Load() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	data, err := os.ReadFile(CacheFilePath())
	if err != nil {
		if os.IsNotExist(err) {
			c.peers = make(map[string]*CachedPeer)
			return nil
		}
		return err
	}

	return yaml.Unmarshal(data, &c.peers)
}

// Save writes the cache to disk
func (c *PeerCache) Save() error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	data, err := yaml.Marshal(c.peers)
	if err != nil {
		return err
	}

	// Ensure cache directory exists
	cachePath := CacheFilePath()
	if err := os.MkdirAll(filepath.Dir(cachePath), 0755); err != nil {
		return err
	}

	// Atomic write
	tmpPath := cachePath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmpPath, cachePath)
}

// Update adds or updates peers in the cache
func (c *PeerCache) Update(peers []*Peer) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for _, p := range peers {
		key := peerKey(p.Host, p.Port)
		c.peers[key] = &CachedPeer{
			Host:     p.Host,
			Port:     p.Port,
			Version:  p.Version,
			LastSeen: now,
		}
	}
}

// GetFresh returns all cached peers seen within the TTL
func (c *PeerCache) GetFresh() []*Peer {
	c.mu.RLock()
	defer c.mu.RUnlock()

	cutoff := time.Now().Add(-PeerTTL)
	var peers []*Peer

	for _, cp := range c.peers {
		if cp.LastSeen.After(cutoff) {
			peers = append(peers, &Peer{
				Host:         cp.Host,
				Port:         cp.Port,
				Version:      cp.Version,
				DiscoveredAt: cp.LastSeen,
			})
		}
	}

	return peers
}

// Cleanup removes stale entries older than TTL
func (c *PeerCache) Cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	cutoff := time.Now().Add(-PeerTTL)
	for key, cp := range c.peers {
		if cp.LastSeen.Before(cutoff) {
			delete(c.peers, key)
		}
	}
}

// Count returns the number of cached peers (including stale)
func (c *PeerCache) Count() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.peers)
}

// FreshCount returns the number of non-stale cached peers
func (c *PeerCache) FreshCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	cutoff := time.Now().Add(-PeerTTL)
	count := 0
	for _, cp := range c.peers {
		if cp.LastSeen.After(cutoff) {
			count++
		}
	}
	return count
}

func peerKey(host string, port int) string {
	return fmt.Sprintf("%s:%d", host, port)
}
