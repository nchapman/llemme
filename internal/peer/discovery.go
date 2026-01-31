package peer

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/mdns"
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
	server   *mdns.Server
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

// register advertises this instance via mDNS
func (d *Discovery) register() error {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "lleme"
	}

	// Get local IP address
	localIP := getLocalIP()

	// Build TXT records with metadata (no model count for privacy)
	txt := []string{
		fmt.Sprintf("version=%s", d.version),
	}

	service, err := mdns.NewMDNSService(
		hostname,          // Instance name
		ServiceType,       // Service type
		Domain,            // Domain
		"",                // Host (empty = use hostname)
		d.port,            // Port
		[]net.IP{localIP}, // IPs
		txt,               // TXT records
	)
	if err != nil {
		return fmt.Errorf("failed to create mDNS service: %w", err)
	}

	server, err := mdns.NewServer(&mdns.Config{Zone: service})
	if err != nil {
		return fmt.Errorf("failed to start mDNS server: %w", err)
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

// discover performs a single mDNS query for peers
func (d *Discovery) discover() {
	entriesCh := make(chan *mdns.ServiceEntry, 10)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	go func() {
		params := mdns.DefaultParams(ServiceType)
		params.DisableIPv6 = true
		params.Entries = entriesCh
		params.Timeout = 3 * time.Second
		if err := mdns.Query(params); err != nil {
			logs.Debug("mDNS query failed", "error", err)
		}
	}()

	// Collect discovered peers
	newPeers := make(map[string]*Peer)

	for {
		select {
		case entry, ok := <-entriesCh:
			if !ok {
				goto done
			}
			if entry == nil {
				continue
			}

			// Parse TXT records first to validate this is a lleme service
			version := ""
			for _, txt := range entry.InfoFields {
				if v, ok := strings.CutPrefix(txt, "version="); ok {
					version = v
				}
			}

			// Skip non-lleme services (must have version TXT record)
			if version == "" {
				continue
			}

			// Skip entries without IPv4 address
			if entry.AddrV4 == nil {
				continue
			}

			// Skip our own instance
			if entry.Port == d.port && isLocalIP(entry.AddrV4) {
				continue
			}

			host := entry.AddrV4.String()

			key := fmt.Sprintf("%s:%d", host, entry.Port)

			newPeers[key] = &Peer{
				Name:         entry.Name,
				Host:         host,
				Port:         entry.Port,
				Version:      version,
				DiscoveredAt: time.Now(),
			}

		case <-ctx.Done():
			goto done
		}
	}

done:
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

// getLocalIP returns the preferred outbound local IP address
func getLocalIP() net.IP {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return net.IPv4(127, 0, 0, 1)
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP
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

// DiscoverPeers returns peers from cache merged with fresh mDNS discovery.
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

	entriesCh := make(chan *mdns.ServiceEntry, 10)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	go func() {
		params := mdns.DefaultParams(ServiceType)
		params.DisableIPv6 = true
		params.Entries = entriesCh
		params.Timeout = 3 * time.Second
		mdns.Query(params)
	}()

	seen := make(map[string]bool)

	for {
		select {
		case entry, ok := <-entriesCh:
			if !ok {
				return
			}
			if entry == nil || entry.AddrV4 == nil {
				continue
			}

			host := entry.AddrV4.String()

			// Skip local instances
			if isLocalIP(entry.AddrV4) {
				continue
			}

			// Parse TXT records first to validate this is a lleme service
			version := ""
			for _, txt := range entry.InfoFields {
				if v, ok := strings.CutPrefix(txt, "version="); ok {
					version = v
				}
			}

			// Skip non-lleme services (must have version TXT record)
			if version == "" {
				continue
			}

			logs.Debug("Discovered lleme peer", "host", host, "port", entry.Port, "version", version)

			key := fmt.Sprintf("%s:%d", host, entry.Port)
			if seen[key] {
				continue
			}
			seen[key] = true

			results <- &Peer{
				Name:         entry.Name,
				Host:         host,
				Port:         entry.Port,
				Version:      version,
				DiscoveredAt: time.Now(),
			}

		case <-ctx.Done():
			return
		}
	}
}
