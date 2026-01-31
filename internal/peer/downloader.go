package peer

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"sync"
	"time"

	"github.com/nchapman/lleme/internal/hf"
	"github.com/nchapman/lleme/internal/ui"
)

// logMu protects global log.Writer changes during peer discovery.
var logMu sync.Mutex

// DiscoverPeersSilent discovers peers with mDNS logging suppressed (fast mode).
// This is thread-safe and can be called concurrently.
func DiscoverPeersSilent() []*Peer {
	logMu.Lock()
	defer logMu.Unlock()

	origOutput := log.Writer()
	log.SetOutput(io.Discard)
	peers := DiscoverPeers()
	log.SetOutput(origOutput)

	return peers
}

// DiscoverPeersThoroughSilent discovers all peers with mDNS logging suppressed (thorough mode).
// Waits longer to find all available peers. Best for `peer list` command.
func DiscoverPeersThoroughSilent() []*Peer {
	logMu.Lock()
	defer logMu.Unlock()

	origOutput := log.Writer()
	log.SetOutput(io.Discard)
	peers := DiscoverPeersThorough()
	log.SetOutput(origOutput)

	return peers
}

// CreateDownloader returns a function that attempts to download files from peers.
// The returned function can be used as hf.PeerDownloadFunc for model pulls.
func CreateDownloader() hf.PeerDownloadFunc {
	// Discover peers once upfront, reuse for all files
	var peers []*Peer
	var peersOnce sync.Once

	return func(hash, destPath string, size int64, progress func(downloaded, total int64)) (bool, error) {
		// Discover peers on first call (with mDNS logging suppressed)
		peersOnce.Do(func() {
			peers = DiscoverPeersSilent()
		})

		if len(peers) == 0 {
			return false, nil
		}

		// Find a peer that has this file
		found := findPeerWithHash(peers, hash)
		if found == nil {
			return false, nil
		}

		// Download from peer
		fmt.Printf(" via peer %s\n", ui.Bold(found.peer.Host))

		if err := found.client.DownloadHash(hash, destPath, progress); err != nil {
			os.Remove(destPath)
			os.Remove(destPath + ".partial")
			return false, nil // Fall back to HuggingFace
		}

		// Quick size check (hash verification is done by caller)
		if info, err := os.Stat(destPath); err != nil || info.Size() != size {
			os.Remove(destPath)
			os.Remove(destPath + ".partial")
			return false, nil
		}

		return true, nil
	}
}

// peerMatch holds a peer that has a file and can serve it.
type peerMatch struct {
	peer   *Peer
	client *Client
	size   int64
}

// findPeerWithHash queries all peers in parallel and returns the first one that has the file.
func findPeerWithHash(peers []*Peer, hash string) *peerMatch {
	if len(peers) == 0 {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Buffer of 1 - we only need the first result
	resultCh := make(chan peerMatch, 1)

	for _, p := range peers {
		go func(p *Peer) {
			client := NewClient(p)
			if size, hasFile := client.HasHash(hash); hasFile {
				select {
				case resultCh <- peerMatch{peer: p, client: client, size: size}:
				default:
					// Another goroutine already sent a result
				}
			}
		}(p)
	}

	select {
	case match := <-resultCh:
		return &match
	case <-ctx.Done():
		return nil
	}
}
