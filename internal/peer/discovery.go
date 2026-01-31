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
	port     int
	version  string // lleme version to advertise
	stopChan chan struct{}
	stopOnce sync.Once
	enabled  bool
	share    bool
}

// NewDiscovery creates a new peer discovery manager
func NewDiscovery(port int, version string, enabled, share bool) *Discovery {
	return &Discovery{
		peers:    make(map[string]*Peer),
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

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-d.stopChan:
			return
		case <-ticker.C:
			d.discover()
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

			// Skip our own instance
			if entry.Port == d.port && isLocalIP(entry.AddrV4) {
				continue
			}

			host := entry.AddrV4.String()
			if host == "" || host == "<nil>" {
				continue
			}

			key := fmt.Sprintf("%s:%d", host, entry.Port)

			// Parse TXT records
			version := ""
			for _, txt := range entry.InfoFields {
				if v, ok := strings.CutPrefix(txt, "version="); ok {
					version = v
				}
			}

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

	if len(newPeers) > 0 {
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

// QuickDiscover performs a one-time mDNS query and sends discovered peers to the channel.
// The channel is closed when discovery completes.
func QuickDiscover(results chan<- *Peer) {
	defer close(results)

	entriesCh := make(chan *mdns.ServiceEntry, 10)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go func() {
		params := mdns.DefaultParams(ServiceType)
		params.DisableIPv6 = true
		params.Entries = entriesCh
		params.Timeout = 2 * time.Second
		mdns.Query(params)
	}()

	seen := make(map[string]bool)

	for {
		select {
		case entry, ok := <-entriesCh:
			if !ok {
				return
			}
			if entry == nil {
				continue
			}

			host := entry.AddrV4.String()
			if host == "" || host == "<nil>" {
				continue
			}

			// Skip local instances
			if isLocalIP(entry.AddrV4) {
				continue
			}

			key := fmt.Sprintf("%s:%d", host, entry.Port)
			if seen[key] {
				continue
			}
			seen[key] = true

			// Parse TXT records
			version := ""
			for _, txt := range entry.InfoFields {
				if v, ok := strings.CutPrefix(txt, "version="); ok {
					version = v
				}
			}

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
