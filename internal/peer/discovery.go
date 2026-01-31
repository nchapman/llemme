package peer

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/grandcat/zeroconf"
	"github.com/nchapman/lleme/internal/config"
	"github.com/nchapman/lleme/internal/logs"
)

const (
	ServiceType = "_lleme._tcp"
	Domain      = "local."

	// Discovery timeouts - use tiered approach for faster response
	FastTimeout        = 300 * time.Millisecond // First attempt - catches most local peers
	MediumTimeout      = 800 * time.Millisecond // Second attempt - catches slower responders
	MaxTimeout         = 2 * time.Second        // Final attempt - maximum wait
	ThoroughTimeout    = 3 * time.Second        // Background polling - find all peers
	RetryDelay         = 100 * time.Millisecond // Delay between retries
	StaticProbeTimeout = 2 * time.Second        // Timeout for probing static peers
)

// DiscoveryMode controls how peer discovery behaves
type DiscoveryMode int

const (
	// ModeFast returns quickly when any peer is found (for CLI, pulls)
	ModeFast DiscoveryMode = iota
	// ModeThorough waits longer to find all available peers (for background polling)
	ModeThorough
)

// Peer represents a discovered lleme instance on the network
type Peer struct {
	Name         string // Instance name (hostname)
	Host         string // IP address or hostname
	Port         int    // HTTP port
	Version      string // lleme version
	DiscoveredAt time.Time
}

// Discovery manages mDNS service registration and peer discovery
type Discovery struct {
	mu       sync.RWMutex
	server   *zeroconf.Server
	peers    map[string]*Peer // key: "host:port"
	cache    *PeerCache       // persistent peer cache
	port     int
	version  string // lleme version to advertise
	stopChan chan struct{}
	stopOnce sync.Once
	enabled  bool
}

// NewDiscovery creates a new peer discovery manager
func NewDiscovery(port int, version string, enabled bool) *Discovery {
	cache := NewPeerCache()
	if err := cache.Load(); err != nil {
		logs.Debug("Failed to load peer cache", "error", err)
	}

	return &Discovery{
		peers:    make(map[string]*Peer),
		cache:    cache,
		port:     port,
		version:  version,
		stopChan: make(chan struct{}),
		enabled:  enabled,
	}
}

// Start begins mDNS registration and peer discovery
func (d *Discovery) Start() error {
	if !d.enabled {
		logs.Debug("Peer discovery disabled")
		return nil
	}

	// Register our service via mDNS
	if err := d.register(); err != nil {
		logs.Warn("Failed to register mDNS service", "error", err)
		// Continue anyway - we can still discover peers
	}

	// Start peer discovery in background
	go d.discoverLoop()

	return nil
}

// Stop shuts down mDNS registration and discovery
func (d *Discovery) Stop() {
	d.stopOnce.Do(func() {
		close(d.stopChan)
	})

	d.mu.Lock()
	defer d.mu.Unlock()

	if d.server != nil {
		d.server.Shutdown()
		d.server = nil
	}
}

// register advertises this instance via mDNS using zeroconf
func (d *Discovery) register() error {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "lleme"
	}

	// Build TXT records with metadata
	txt := []string{
		fmt.Sprintf("version=%s", d.version),
	}

	// Register the service - zeroconf handles the rest
	server, err := zeroconf.Register(
		hostname,    // Instance name
		ServiceType, // Service type
		Domain,      // Domain
		d.port,      // Port
		txt,         // TXT records
		nil,         // Use all interfaces
	)
	if err != nil {
		return fmt.Errorf("failed to register mDNS service: %w", err)
	}

	d.mu.Lock()
	d.server = server
	d.mu.Unlock()

	logs.Debug("mDNS service registered", "hostname", hostname, "port", d.port)
	return nil
}

// discoverLoop periodically scans for peers
func (d *Discovery) discoverLoop() {
	// Initial discovery
	d.discover()

	ticker := time.NewTicker(PollInterval)
	defer ticker.Stop()

	pollCount := 0
	for {
		select {
		case <-d.stopChan:
			return
		case <-ticker.C:
			d.discover()
			pollCount++
			// Cleanup stale cache entries every ~20 minutes
			if pollCount%10 == 0 {
				d.cache.Cleanup()
				if err := d.cache.Save(); err != nil {
					logs.Debug("Failed to save peer cache after cleanup", "error", err)
				}
			}
		}
	}
}

// discover performs a single mDNS query for peers using zeroconf.
// Uses thorough mode to find all available peers for the background loop.
func (d *Discovery) discover() {
	// Use thorough discovery to find all peers
	peers := discoverWithMode(ModeThorough)

	// Convert to map for updating
	newPeers := make(map[string]*Peer, len(peers))
	for _, p := range peers {
		// Skip our own instance
		if ip := net.ParseIP(p.Host); ip != nil && isLocalIP(ip) && p.Port == d.port {
			continue
		}
		newPeers[peerKey(p.Host, p.Port)] = p
	}

	// Update peer list
	d.mu.Lock()
	d.peers = newPeers
	d.mu.Unlock()

	// Update and save cache
	if len(newPeers) > 0 {
		peerList := make([]*Peer, 0, len(newPeers))
		for _, p := range newPeers {
			peerList = append(peerList, p)
		}
		d.cache.Update(peerList)
		if err := d.cache.Save(); err != nil {
			logs.Debug("Failed to save peer cache", "error", err)
		}
		logs.Debug("Discovered peers", "count", len(newPeers))
	}
}

// GetLocalIP returns the preferred outbound local IP address as a string.
func GetLocalIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "127.0.0.1"
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String()
}

// IsServerRunning checks if a peer server is listening on the given port.
func IsServerRunning(port int) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 500*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// isLocalIP checks if an IP belongs to this machine
func isLocalIP(ip net.IP) bool {
	if ip == nil {
		return false
	}

	// Check loopback
	if ip.IsLoopback() {
		return true
	}

	// Check all local interfaces
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return false
	}

	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok {
			if ipnet.IP.Equal(ip) {
				return true
			}
		}
	}

	return false
}

// DiscoverPeers returns peers using fast mode (returns quickly when any peer is found).
// Best for pulls and operations where finding one peer quickly is preferred.
// Fresh discovery results take precedence over cached entries.
func DiscoverPeers() []*Peer {
	return discoverPeersWithMode(ModeFast)
}

// DiscoverPeersThorough returns peers using thorough mode (waits to find all peers).
// Best for `peer list` and operations where a complete peer list is preferred.
// Fresh discovery results take precedence over cached entries.
func DiscoverPeersThorough() []*Peer {
	return discoverPeersWithMode(ModeThorough)
}

// discoverPeersWithMode returns peers from cache merged with fresh mDNS discovery and static peers.
// Fresh discovery results take precedence over cached entries.
func discoverPeersWithMode(mode DiscoveryMode) []*Peer {
	// Load cache first for fast fallback
	cache := NewPeerCache()
	cache.Load() // Ignore errors, cache may not exist
	cachedPeers := cache.GetFresh()

	// Try fresh discovery with the requested mode
	freshPeers := discoverWithMode(mode)

	// Update cache with fresh results
	if len(freshPeers) > 0 {
		cache.Update(freshPeers)
		cache.Save() // Ignore errors
	}

	// Merge: fresh peers first, then cached peers not already present
	seen := make(map[string]bool)
	var result []*Peer

	for _, p := range freshPeers {
		key := peerKey(p.Host, p.Port)
		seen[key] = true
		result = append(result, p)
	}

	for _, p := range cachedPeers {
		key := peerKey(p.Host, p.Port)
		if !seen[key] {
			seen[key] = true
			result = append(result, p)
		}
	}

	// Add static peers from config (probed in parallel)
	for _, p := range getStaticPeersParallel() {
		key := peerKey(p.Host, p.Port)
		if !seen[key] {
			seen[key] = true
			result = append(result, p)
		}
	}

	return result
}

// discoverWithMode performs discovery based on the specified mode.
// ModeFast: tiered timeouts with early return when any peer is found
// ModeThorough: longer timeout to find all available peers
func discoverWithMode(mode DiscoveryMode) []*Peer {
	if mode == ModeThorough {
		// For background polling, use longer timeout to find all peers
		peers := discoverWithTimeout(ThoroughTimeout)
		logs.Debug("Thorough discovery completed", "count", len(peers))
		return peers
	}

	// Fast mode: tiered timeouts with early return
	timeouts := []time.Duration{FastTimeout, MediumTimeout, MaxTimeout}

	for i, timeout := range timeouts {
		peers := discoverWithTimeout(timeout)
		if len(peers) > 0 {
			logs.Debug("Fast discovery found peers", "count", len(peers), "timeout", timeout, "attempt", i+1)
			return peers
		}
		if i < len(timeouts)-1 {
			time.Sleep(RetryDelay)
		}
	}

	return nil
}

// discoverWithTimeout performs mDNS discovery with specified timeout.
// Collects all peers discovered within the timeout period.
func discoverWithTimeout(timeout time.Duration) []*Peer {
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		logs.Debug("Failed to create mDNS resolver", "error", err)
		return nil
	}

	entries := make(chan *zeroconf.ServiceEntry, 10)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	go func() {
		if err := resolver.Browse(ctx, ServiceType, Domain, entries); err != nil {
			logs.Debug("mDNS browse failed", "error", err)
		}
	}()

	var peers []*Peer
	seen := make(map[string]bool)

	for entry := range entries {
		if entry == nil {
			continue
		}

		// Get IPv4 address, skip local
		var host string
		for _, ip := range entry.AddrIPv4 {
			if ip != nil && !isLocalIP(ip) {
				host = ip.String()
				break
			}
		}

		if host == "" {
			continue
		}

		key := peerKey(host, entry.Port)
		if seen[key] {
			continue
		}
		seen[key] = true

		// Parse TXT records for version
		version := ""
		for _, txt := range entry.Text {
			if v, ok := strings.CutPrefix(txt, "version="); ok {
				version = v
			}
		}

		peers = append(peers, &Peer{
			Name:         entry.Instance,
			Host:         host,
			Port:         entry.Port,
			Version:      version,
			DiscoveredAt: time.Now(),
		})

		logs.Debug("Discovered peer", "host", host, "port", entry.Port, "version", version)
	}

	return peers
}

// probeStaticPeer checks if a static peer address is a valid lleme instance.
// Returns a Peer if valid, nil otherwise.
func probeStaticPeer(addr string) *Peer {
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		logs.Debug("Invalid static peer address", "addr", addr, "error", err)
		return nil
	}

	port, err := strconv.Atoi(portStr)
	if err != nil || port < 1 || port > 65535 {
		logs.Debug("Invalid static peer port", "addr", addr, "port", portStr)
		return nil
	}

	// Skip if this is our own address
	if ip := net.ParseIP(host); ip != nil && isLocalIP(ip) {
		return nil
	}

	// Probe the peer with a HEAD request to check if it's alive
	client := &http.Client{Timeout: StaticProbeTimeout}
	url := fmt.Sprintf("http://%s/api/peer/sha256/0000000000000000000000000000000000000000000000000000000000000000", addr)
	resp, err := client.Head(url)
	if err != nil {
		logs.Debug("Static peer not reachable", "addr", addr, "error", err)
		return nil
	}
	resp.Body.Close()

	// 400 (invalid hash) or 404 (not found) both indicate a working peer server
	if resp.StatusCode != http.StatusBadRequest && resp.StatusCode != http.StatusNotFound {
		logs.Debug("Static peer returned unexpected status", "addr", addr, "status", resp.StatusCode)
		return nil
	}

	logs.Debug("Static peer verified", "addr", addr)
	return &Peer{
		Name:         host,
		Host:         host,
		Port:         port,
		Version:      "unknown",
		DiscoveredAt: time.Now(),
	}
}

// getStaticPeersParallel loads and probes static peers from config in parallel.
func getStaticPeersParallel() []*Peer {
	cfg, err := config.Load()
	if err != nil || len(cfg.Peer.StaticPeers) == 0 {
		return nil
	}

	results := make(chan *Peer, len(cfg.Peer.StaticPeers))

	var wg sync.WaitGroup
	for _, addr := range cfg.Peer.StaticPeers {
		wg.Add(1)
		go func(addr string) {
			defer wg.Done()
			results <- probeStaticPeer(addr)
		}(addr)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	var peers []*Peer
	for p := range results {
		if p != nil {
			peers = append(peers, p)
		}
	}
	return peers
}
