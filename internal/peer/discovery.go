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
	share    bool
}

// NewDiscovery creates a new peer discovery manager
func NewDiscovery(port int, version string, enabled, share bool) *Discovery {
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
		share:    share,
	}
}

// Start begins mDNS registration and peer discovery
func (d *Discovery) Start() error {
	if !d.enabled {
		logs.Debug("Peer discovery disabled")
		return nil
	}

	// Register our service if sharing is enabled
	if d.share {
		if err := d.register(); err != nil {
			logs.Warn("Failed to register mDNS service", "error", err)
			// Continue anyway - we can still discover peers
		}
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

// discover performs a single mDNS query for peers using zeroconf
func (d *Discovery) discover() {
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		logs.Debug("Failed to create mDNS resolver", "error", err)
		return
	}

	entries := make(chan *zeroconf.ServiceEntry, 10)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	go func() {
		if err := resolver.Browse(ctx, ServiceType, Domain, entries); err != nil {
			logs.Debug("mDNS browse failed", "error", err)
		}
	}()

	// Collect discovered peers
	newPeers := make(map[string]*Peer)

	for entry := range entries {
		if entry == nil {
			continue
		}

		// Parse TXT records to get version
		version := ""
		for _, txt := range entry.Text {
			if v, ok := strings.CutPrefix(txt, "version="); ok {
				version = v
			}
		}

		// Get IPv4 address
		var host string
		for _, ip := range entry.AddrIPv4 {
			if ip != nil && !isLocalIP(ip) {
				host = ip.String()
				break
			}
		}

		// Skip if no valid IPv4 or is local
		if host == "" {
			continue
		}

		// Skip our own instance (host is already verified non-local, just check port)
		if entry.Port == d.port && len(entry.AddrIPv4) > 0 && isLocalIP(entry.AddrIPv4[0]) {
			continue
		}

		key := fmt.Sprintf("%s:%d", host, entry.Port)

		newPeers[key] = &Peer{
			Name:         entry.Instance,
			Host:         host,
			Port:         entry.Port,
			Version:      version,
			DiscoveredAt: time.Now(),
		}
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

// Peers returns a copy of all discovered peers
func (d *Discovery) Peers() []*Peer {
	d.mu.RLock()
	defer d.mu.RUnlock()

	peers := make([]*Peer, 0, len(d.peers))
	for _, p := range d.peers {
		// Make a copy
		peerCopy := *p
		peers = append(peers, &peerCopy)
	}
	return peers
}

// PeerCount returns the number of discovered peers
func (d *Discovery) PeerCount() int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return len(d.peers)
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

// DiscoverPeers returns peers from cache merged with fresh mDNS discovery and static peers.
// Fresh discovery results take precedence over cached entries.
func DiscoverPeers() []*Peer {
	// Load cache
	cache := NewPeerCache()
	cache.Load() // Ignore errors, cache may not exist

	// Try fresh discovery (with retries for mDNS flakiness)
	var freshPeers []*Peer
	for attempt := range 3 {
		freshPeers = discoverPeersOnce()
		if len(freshPeers) > 0 {
			break
		}
		if attempt < 2 {
			time.Sleep(200 * time.Millisecond)
		}
	}

	// Update cache with fresh results
	if len(freshPeers) > 0 {
		cache.Update(freshPeers)
		cache.Save() // Ignore errors
	}

	// Merge: start with fresh peers, add non-stale cached peers not already present
	seen := make(map[string]bool)
	var result []*Peer

	for _, p := range freshPeers {
		key := peerKey(p.Host, p.Port)
		seen[key] = true
		result = append(result, p)
	}

	for _, p := range cache.GetFresh() {
		key := peerKey(p.Host, p.Port)
		if !seen[key] {
			seen[key] = true
			result = append(result, p)
		}
	}

	// Add static peers from config (useful when mDNS doesn't work)
	for _, p := range getStaticPeers() {
		key := peerKey(p.Host, p.Port)
		if !seen[key] {
			seen[key] = true
			result = append(result, p)
		}
	}

	return result
}

func discoverPeersOnce() []*Peer {
	var peers []*Peer
	ch := make(chan *Peer, 10)

	done := make(chan struct{})
	go func() {
		for p := range ch {
			peers = append(peers, p)
		}
		close(done)
	}()

	QuickDiscover(ch)
	<-done
	return peers
}

// QuickDiscover performs a one-time mDNS query and sends discovered peers to the channel.
// The channel is closed when discovery completes.
func QuickDiscover(results chan<- *Peer) {
	defer close(results)

	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		logs.Debug("Failed to create mDNS resolver", "error", err)
		return
	}

	entries := make(chan *zeroconf.ServiceEntry, 10)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	go func() {
		if err := resolver.Browse(ctx, ServiceType, Domain, entries); err != nil {
			logs.Debug("mDNS browse failed", "error", err)
		}
	}()

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

		// Parse TXT records for version
		version := ""
		for _, txt := range entry.Text {
			if v, ok := strings.CutPrefix(txt, "version="); ok {
				version = v
			}
		}

		logs.Debug("Discovered lleme peer", "host", host, "port", entry.Port, "version", version)

		key := fmt.Sprintf("%s:%d", host, entry.Port)
		if seen[key] {
			continue
		}
		seen[key] = true

		results <- &Peer{
			Name:         entry.Instance,
			Host:         host,
			Port:         entry.Port,
			Version:      version,
			DiscoveredAt: time.Now(),
		}
	}
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
	if err != nil {
		logs.Debug("Invalid static peer port", "addr", addr, "error", err)
		return nil
	}

	// Skip if this is our own address
	if ip := net.ParseIP(host); ip != nil && isLocalIP(ip) {
		return nil
	}

	// Probe the peer with a HEAD request to check if it's alive
	client := &http.Client{Timeout: 2 * time.Second}
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

// getStaticPeers loads and probes static peers from config.
func getStaticPeers() []*Peer {
	cfg, err := config.Load()
	if err != nil || len(cfg.Peer.StaticPeers) == 0 {
		return nil
	}

	var peers []*Peer
	for _, addr := range cfg.Peer.StaticPeers {
		if p := probeStaticPeer(addr); p != nil {
			peers = append(peers, p)
		}
	}
	return peers
}
