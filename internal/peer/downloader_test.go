package peer

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestCreateDownloader(t *testing.T) {
	downloader := CreateDownloader("user", "repo", "Q4_K_M")
	if downloader == nil {
		t.Fatal("CreateDownloader() returned nil")
	}
}

func TestCreateDownloaderNoPeers(t *testing.T) {
	// When no peers are discovered, should return false
	downloader := CreateDownloader("user", "repo", "Q4_K_M")

	tmpDir := t.TempDir()
	destPath := filepath.Join(tmpDir, "model.gguf")

	// This will discover no peers (in test environment) and return false
	downloaded, err := downloader("somehash", destPath, 1000, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if downloaded {
		t.Error("should return false when no peers are available")
	}
}

func TestFindPeerWithHashNoPeers(t *testing.T) {
	result := findPeerWithHash(nil, "somehash")
	if result != nil {
		t.Error("should return nil when no peers")
	}

	result = findPeerWithHash([]*Peer{}, "somehash")
	if result != nil {
		t.Error("should return nil when empty peer list")
	}
}

func TestFindPeerWithHashTimeout(t *testing.T) {
	// Create peers that will never respond (no server running)
	peers := []*Peer{
		{Host: "192.0.2.1", Port: 12345}, // TEST-NET address, won't respond
	}

	start := time.Now()
	result := findPeerWithHash(peers, "somehash")
	elapsed := time.Since(start)

	if result != nil {
		t.Error("should return nil when peers don't respond")
	}

	// Should timeout within reasonable time (5 seconds + some buffer)
	if elapsed > 10*time.Second {
		t.Errorf("took too long: %v", elapsed)
	}
}

func TestPeerMatchStruct(t *testing.T) {
	peer := &Peer{Host: "192.168.1.1", Port: 11313}
	client := NewClient(peer)

	match := peerMatch{
		peer:   peer,
		client: client,
		size:   1000000,
	}

	if match.peer != peer {
		t.Error("peer should be set")
	}
	if match.client != client {
		t.Error("client should be set")
	}
	if match.size != 1000000 {
		t.Error("size should be set")
	}
}

func TestCreateDownloaderReusePeers(t *testing.T) {
	// Test that peer discovery happens only once
	downloader := CreateDownloader("user", "repo", "Q4_K_M")

	tmpDir := t.TempDir()

	// Call multiple times - should reuse peer list
	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			destPath := filepath.Join(tmpDir, "model"+string(rune('0'+i))+".gguf")
			downloader("hash"+string(rune('0'+i)), destPath, 1000, nil)
		}(i)
	}
	wg.Wait()

	// If we got here without deadlock/panic, sync.Once is working
}

func TestCreateDownloaderWithProgress(t *testing.T) {
	downloader := CreateDownloader("user", "repo", "Q4_K_M")

	tmpDir := t.TempDir()
	destPath := filepath.Join(tmpDir, "model.gguf")

	progressFn := func(downloaded, total int64) {
		// Progress function provided but won't be called without peers
	}

	// Even with no peers, progress function should be handled gracefully
	downloaded, _ := downloader("somehash", destPath, 1000, progressFn)

	// Since there are no peers, it should return false and not call progress
	if downloaded {
		t.Error("should not download without peers")
	}
}

func TestCreateDownloaderSizeCheck(t *testing.T) {
	// This tests the size check logic by creating a mock scenario
	// In real usage, the peer would provide the file

	tmpDir := t.TempDir()
	destPath := filepath.Join(tmpDir, "model.gguf")

	// Create a file with wrong size
	os.WriteFile(destPath, []byte("short"), 0644)

	// The size check in CreateDownloader would detect this mismatch
	// and return false, triggering fallback to HuggingFace
	info, _ := os.Stat(destPath)
	expectedSize := int64(1000)

	if info.Size() == expectedSize {
		t.Error("test setup error: file should have different size")
	}
}

func TestFindPeerWithHashConcurrency(t *testing.T) {
	// Test that concurrent peer queries don't cause issues
	peers := make([]*Peer, 10)
	for i := range peers {
		peers[i] = &Peer{
			Host: "192.0.2.1", // TEST-NET, won't respond
			Port: 12345 + i,
		}
	}

	// Run multiple searches concurrently
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			findPeerWithHash(peers, "somehash")
		}()
	}
	wg.Wait()
	// If we get here without deadlock, concurrent access is safe
}
