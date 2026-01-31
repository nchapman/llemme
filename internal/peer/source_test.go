package peer

import (
	"testing"
)

func TestNewSource(t *testing.T) {
	// Test with nil discovery
	s := NewSource(nil)
	if s == nil {
		t.Fatal("NewSource returned nil")
	}
	if s.discovery != nil {
		t.Error("discovery should be nil when passed nil")
	}

	// Test with actual discovery
	d := NewDiscovery(11313, "0.1.0", true, false)
	s = NewSource(d)
	if s.discovery != d {
		t.Error("discovery should match input")
	}
}

func TestFindHashNilDiscovery(t *testing.T) {
	s := NewSource(nil)

	result := s.FindHash("somehash")
	if result != nil {
		t.Error("expected nil when discovery is nil")
	}
}

func TestFindHashNoPeers(t *testing.T) {
	d := NewDiscovery(11313, "0.1.0", false, false) // disabled, so no peers
	s := NewSource(d)

	result := s.FindHash("somehash")
	if result != nil {
		t.Error("expected nil when no peers discovered")
	}
}

func TestFindHashAllNilDiscovery(t *testing.T) {
	s := NewSource(nil)

	results := s.FindHashAll("somehash")
	if results != nil {
		t.Error("expected nil when discovery is nil")
	}
}

func TestFindHashAllNoPeers(t *testing.T) {
	d := NewDiscovery(11313, "0.1.0", false, false)
	s := NewSource(d)

	results := s.FindHashAll("somehash")
	if results != nil {
		t.Error("expected nil when no peers discovered")
	}
}

func TestPeerHashSourceStruct(t *testing.T) {
	peer := &Peer{
		Name: "test",
		Host: "192.168.1.100",
		Port: 11314,
	}
	client := NewClient(peer)

	phs := &PeerHashSource{
		Peer:   peer,
		Hash:   "abc123",
		Size:   1024,
		Client: client,
	}

	if phs.Peer != peer {
		t.Error("Peer mismatch")
	}
	if phs.Hash != "abc123" {
		t.Errorf("expected hash abc123, got %s", phs.Hash)
	}
	if phs.Size != 1024 {
		t.Errorf("expected size 1024, got %d", phs.Size)
	}
	if phs.Client != client {
		t.Error("Client mismatch")
	}
}
