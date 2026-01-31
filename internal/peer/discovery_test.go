package peer

import (
	"net"
	"testing"
	"time"
)

func TestNewDiscovery(t *testing.T) {
	d := NewDiscovery(11313, "0.1.0", true, true)

	if d == nil {
		t.Fatal("NewDiscovery returned nil")
	}

	if d.port != 11313 {
		t.Errorf("expected port 11313, got %d", d.port)
	}

	if d.version != "0.1.0" {
		t.Errorf("expected version 0.1.0, got %s", d.version)
	}

	if !d.enabled {
		t.Error("expected enabled to be true")
	}

	if !d.share {
		t.Error("expected share to be true")
	}
}

func TestDiscoveryDisabled(t *testing.T) {
	d := NewDiscovery(11313, "0.1.0", false, false)

	// Start should return nil and not actually start anything
	if err := d.Start(); err != nil {
		t.Errorf("Start returned error for disabled discovery: %v", err)
	}

	// Peers should be empty
	if len(d.Peers()) != 0 {
		t.Errorf("expected 0 peers, got %d", len(d.Peers()))
	}

	// PeerCount should be 0
	if d.PeerCount() != 0 {
		t.Errorf("expected peer count 0, got %d", d.PeerCount())
	}

	// Stop should not panic
	d.Stop()
}

func TestPeerStruct(t *testing.T) {
	p := &Peer{
		Name:         "test-host",
		Host:         "192.168.1.100",
		Port:         11313,
		Version:      "0.1.0",
		DiscoveredAt: time.Now(),
	}

	if p.Name != "test-host" {
		t.Errorf("expected name test-host, got %s", p.Name)
	}

	if p.Host != "192.168.1.100" {
		t.Errorf("expected host 192.168.1.100, got %s", p.Host)
	}

	if p.Port != 11313 {
		t.Errorf("expected port 11313, got %d", p.Port)
	}
}

func TestGetLocalIP(t *testing.T) {
	ipStr := GetLocalIP()

	if ipStr == "" {
		t.Fatal("GetLocalIP returned empty string")
	}

	// Should be a valid IPv4 address
	ip := net.ParseIP(ipStr)
	if ip == nil || ip.To4() == nil {
		t.Errorf("expected valid IPv4 address, got %q", ipStr)
	}
}

func TestIsLocalIP(t *testing.T) {
	tests := []struct {
		name     string
		ip       net.IP
		expected bool
	}{
		{
			name:     "nil IP",
			ip:       nil,
			expected: false,
		},
		{
			name:     "loopback IPv4",
			ip:       net.IPv4(127, 0, 0, 1),
			expected: true,
		},
		{
			name:     "external IP",
			ip:       net.IPv4(8, 8, 8, 8),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isLocalIP(tt.ip)
			if result != tt.expected {
				t.Errorf("isLocalIP(%v) = %v, want %v", tt.ip, result, tt.expected)
			}
		})
	}
}

func TestServiceTypeConstant(t *testing.T) {
	if ServiceType != "_lleme._tcp" {
		t.Errorf("expected service type _lleme._tcp, got %s", ServiceType)
	}
}

func TestDomainConstant(t *testing.T) {
	if Domain != "local." {
		t.Errorf("expected domain local., got %s", Domain)
	}
}

func TestDiscoverPeers(t *testing.T) {
	// This test exercises the DiscoverPeers function
	// It may or may not find peers depending on the network environment
	peers := DiscoverPeers()

	// Should complete without error (nil slice is valid if no peers)
	// If peers are found, verify they have required fields
	for _, p := range peers {
		if p.Host == "" {
			t.Error("peer should have a host")
		}
		if p.Port == 0 {
			t.Error("peer should have a port")
		}
		// Version may be empty if peer doesn't advertise it
	}

	t.Logf("Found %d peers", len(peers))
}

func TestPeersCopyBehavior(t *testing.T) {
	d := NewDiscovery(11313, "0.1.0", true, false)

	// Manually inject a peer for testing
	d.mu.Lock()
	d.peers["192.168.1.100:11313"] = &Peer{
		Host: "192.168.1.100",
		Port: 11313,
	}
	d.mu.Unlock()

	// Get peers
	peers := d.Peers()
	if len(peers) != 1 {
		t.Fatalf("expected 1 peer, got %d", len(peers))
	}

	// Modify the returned peer
	peers[0].Host = "modified"

	// Original should be unchanged
	d.mu.RLock()
	original := d.peers["192.168.1.100:11313"]
	d.mu.RUnlock()

	if original.Host != "192.168.1.100" {
		t.Errorf("original peer was modified, expected 192.168.1.100, got %s", original.Host)
	}
}

func TestProbeStaticPeerInvalidAddress(t *testing.T) {
	tests := []struct {
		name string
		addr string
	}{
		{"no port", "192.168.1.100"},
		{"empty", ""},
		{"just colon", ":"},
		{"invalid port", "192.168.1.100:abc"},
		{"negative port", "192.168.1.100:-1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := probeStaticPeer(tt.addr)
			if result != nil {
				t.Errorf("probeStaticPeer(%q) should return nil for invalid address", tt.addr)
			}
		})
	}
}

func TestProbeStaticPeerLocalIP(t *testing.T) {
	// Local IPs should be skipped
	result := probeStaticPeer("127.0.0.1:11314")
	if result != nil {
		t.Error("probeStaticPeer should return nil for loopback address")
	}
}

func TestProbeStaticPeerUnreachable(t *testing.T) {
	// Use a non-routable IP that will timeout quickly
	result := probeStaticPeer("192.0.2.1:11314") // TEST-NET-1, should be unreachable
	if result != nil {
		t.Error("probeStaticPeer should return nil for unreachable peer")
	}
}

func TestGetStaticPeersParallelEmpty(t *testing.T) {
	// When no static peers are configured, should return nil
	// This test relies on the test environment not having static_peers configured
	peers := getStaticPeersParallel()
	// Just verify it doesn't panic - result depends on config
	t.Logf("getStaticPeersParallel returned %d peers", len(peers))
}

func TestDiscoverPeersTimings(t *testing.T) {
	// Test that discovery uses fast timeouts
	start := time.Now()
	peers := DiscoverPeers()
	elapsed := time.Since(start)

	t.Logf("Discovery found %d peers in %v", len(peers), elapsed)

	// With tiered timeouts (300ms + 800ms + 2s), if peers are found quickly
	// it should return in under 500ms. If no peers, max is ~3.2s.
	if len(peers) > 0 && elapsed > 1*time.Second {
		t.Logf("Warning: discovery with peers took longer than expected: %v", elapsed)
	}
}

func TestDiscoveryModes(t *testing.T) {
	// Test fast mode (should return quickly when peer found)
	start := time.Now()
	fastPeers := discoverWithMode(ModeFast)
	fastElapsed := time.Since(start)

	// Test thorough mode (should wait full timeout)
	start = time.Now()
	thoroughPeers := discoverWithMode(ModeThorough)
	thoroughElapsed := time.Since(start)

	t.Logf("Fast mode: %d peers in %v", len(fastPeers), fastElapsed)
	t.Logf("Thorough mode: %d peers in %v", len(thoroughPeers), thoroughElapsed)

	// If peers are found, fast mode should be quicker than thorough mode
	if len(fastPeers) > 0 && len(thoroughPeers) > 0 {
		if fastElapsed >= thoroughElapsed {
			t.Logf("Note: Fast mode (%v) was not faster than thorough mode (%v)", fastElapsed, thoroughElapsed)
		}
	}

	// Thorough mode should find at least as many peers as fast mode
	if len(thoroughPeers) < len(fastPeers) {
		t.Errorf("Thorough mode found fewer peers (%d) than fast mode (%d)", len(thoroughPeers), len(fastPeers))
	}
}

func BenchmarkDiscoverPeers(b *testing.B) {
	for i := 0; i < b.N; i++ {
		DiscoverPeers()
	}
}

func BenchmarkDiscoverFastMode(b *testing.B) {
	for i := 0; i < b.N; i++ {
		discoverWithMode(ModeFast)
	}
}

func BenchmarkDiscoverThoroughMode(b *testing.B) {
	for i := 0; i < b.N; i++ {
		discoverWithMode(ModeThorough)
	}
}
