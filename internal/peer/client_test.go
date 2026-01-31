package peer

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
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

func TestVerifyDownloadEmptyHash(t *testing.T) {
	// Create a temp file
	tmpFile, err := os.CreateTemp("", "test-*.gguf")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.WriteString("test content")
	tmpFile.Close()

	// Empty hash should return true (skip verification)
	ok, err := VerifyDownload(tmpFile.Name(), "")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("expected true for empty hash")
	}
}

func TestVerifyDownloadCorrectHash(t *testing.T) {
	// Create a temp file with known content
	tmpFile, err := os.CreateTemp("", "test-*.gguf")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	content := []byte("test content for hashing")
	tmpFile.Write(content)
	tmpFile.Close()

	// Calculate expected hash
	hash := sha256.Sum256(content)
	expectedHash := hex.EncodeToString(hash[:])

	ok, err := VerifyDownload(tmpFile.Name(), expectedHash)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("expected true for correct hash")
	}
}

func TestVerifyDownloadIncorrectHash(t *testing.T) {
	// Create a temp file
	tmpFile, err := os.CreateTemp("", "test-*.gguf")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.WriteString("test content")
	tmpFile.Close()

	// Wrong hash
	wrongHash := "0000000000000000000000000000000000000000000000000000000000000000"

	ok, err := VerifyDownload(tmpFile.Name(), wrongHash)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if ok {
		t.Error("expected false for incorrect hash")
	}
}

func TestVerifyDownloadCaseInsensitive(t *testing.T) {
	// Create a temp file with known content
	tmpFile, err := os.CreateTemp("", "test-*.gguf")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	content := []byte("test content for case test")
	tmpFile.Write(content)
	tmpFile.Close()

	// Calculate expected hash
	hash := sha256.Sum256(content)
	lowerHash := hex.EncodeToString(hash[:])

	// Convert to uppercase
	upperHash := ""
	for _, c := range lowerHash {
		if c >= 'a' && c <= 'f' {
			upperHash += string(c - 32) // Convert to uppercase
		} else {
			upperHash += string(c)
		}
	}

	// Should work with uppercase hash
	ok, err := VerifyDownload(tmpFile.Name(), upperHash)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("expected true for uppercase hash (case-insensitive comparison)")
	}
}

func TestVerifyDownloadFileNotFound(t *testing.T) {
	_, err := VerifyDownload("/nonexistent/file.gguf", "somehash")
	if err == nil {
		t.Error("expected error for non-existent file")
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
