package peer

import (
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	peer := &Peer{
		Name:    "test-peer",
		Host:    "192.168.1.100",
		Port:    11314,
		Version: "0.1.0",
	}

	client := NewClient(peer)
	if client == nil {
		t.Fatal("NewClient returned nil")
	}
	if client.peer != peer {
		t.Error("client.peer should match input peer")
	}
	if client.httpClient == nil {
		t.Error("httpClient should be initialized")
	}
}

func TestPeerStructFields(t *testing.T) {
	now := time.Now()
	p := &Peer{
		Name:         "test-host",
		Host:         "192.168.1.100",
		Port:         11314,
		Version:      "0.2.0",
		DiscoveredAt: now,
	}

	if p.Name != "test-host" {
		t.Errorf("expected name test-host, got %s", p.Name)
	}
	if p.Host != "192.168.1.100" {
		t.Errorf("expected host 192.168.1.100, got %s", p.Host)
	}
	if p.Port != 11314 {
		t.Errorf("expected port 11314, got %d", p.Port)
	}
	if p.Version != "0.2.0" {
		t.Errorf("expected version 0.2.0, got %s", p.Version)
	}
	if !p.DiscoveredAt.Equal(now) {
		t.Error("DiscoveredAt should match")
	}
}
