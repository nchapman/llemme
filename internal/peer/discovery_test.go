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
	ip := getLocalIP()

	if ip == nil {
		t.Fatal("getLocalIP returned nil")
	}

	// Should be a valid IPv4 address
	if ip.To4() == nil {
		t.Errorf("expected IPv4 address, got %v", ip)
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
		// Version must be non-empty (our filter requirement)
		if p.Version == "" {
			t.Error("peer should have version (we filter on this)")
		}
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
