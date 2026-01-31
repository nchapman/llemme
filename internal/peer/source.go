package peer

import (
	"sync"
	"time"

	"github.com/nchapman/lleme/internal/logs"
)

// Source manages finding and downloading files from peers by hash.
type Source struct {
	discovery *Discovery
}

// NewSource creates a new peer source for file downloads.
func NewSource(discovery *Discovery) *Source {
	return &Source{
		discovery: discovery,
	}
}

// PeerHashSource represents a peer that has a file with a specific hash.
type PeerHashSource struct {
	Peer   *Peer
	Hash   string
	Size   int64
	Client *Client
}

// FindHash searches all peers for a file with the given SHA256 hash.
// Returns nil if no peer has the file.
func (s *Source) FindHash(hash string) *PeerHashSource {
	if s.discovery == nil {
		return nil
	}

	peers := s.discovery.Peers()
	if len(peers) == 0 {
		return nil
	}

	hashDisplay := hash
	if len(hash) > 16 {
		hashDisplay = hash[:16] + "..."
	}
	logs.Debug("Checking peers for hash", "hash", hashDisplay, "peer_count", len(peers))

	// Query peers in parallel with timeout
	type result struct {
		peer   *Peer
		size   int64
		client *Client
	}

	resultCh := make(chan result, len(peers))
	var wg sync.WaitGroup

	for _, p := range peers {
		wg.Add(1)
		go func(peer *Peer) {
			defer wg.Done()

			client := NewClient(peer)
			size, ok := client.HasHash(hash)
			if ok {
				resultCh <- result{peer: peer, size: size, client: client}
			}
		}(p)
	}

	// Close resultCh when all goroutines complete (in background)
	go func() {
		wg.Wait()
		close(resultCh)
	}()

	// Collect results until timeout or all goroutines complete
	var best *PeerHashSource
	timeout := time.After(5 * time.Second)

collectLoop:
	for {
		select {
		case r, ok := <-resultCh:
			if !ok {
				break collectLoop
			}
			// Return the first peer that has the file
			if best == nil {
				best = &PeerHashSource{
					Peer:   r.peer,
					Hash:   hash,
					Size:   r.size,
					Client: r.client,
				}
			}
		case <-timeout:
			logs.Debug("Peer query timed out")
			break collectLoop
		}
	}

	if best != nil {
		logs.Debug("Found hash on peer", "peer", best.Peer.Host, "hash", hashDisplay)
	}

	return best
}

// FindHashAll searches all peers and returns all peers that have the file.
// Useful when you want to pick from multiple sources.
func (s *Source) FindHashAll(hash string) []*PeerHashSource {
	if s.discovery == nil {
		return nil
	}

	peers := s.discovery.Peers()
	if len(peers) == 0 {
		return nil
	}

	type result struct {
		peer   *Peer
		size   int64
		client *Client
	}

	resultCh := make(chan result, len(peers))
	var wg sync.WaitGroup

	for _, p := range peers {
		wg.Add(1)
		go func(peer *Peer) {
			defer wg.Done()
			client := NewClient(peer)
			size, ok := client.HasHash(hash)
			if ok {
				resultCh <- result{peer: peer, size: size, client: client}
			}
		}(p)
	}

	go func() {
		wg.Wait()
		close(resultCh)
	}()

	var sources []*PeerHashSource
	timeout := time.After(5 * time.Second)

collectLoop:
	for {
		select {
		case r, ok := <-resultCh:
			if !ok {
				break collectLoop
			}
			sources = append(sources, &PeerHashSource{
				Peer:   r.peer,
				Hash:   hash,
				Size:   r.size,
				Client: r.client,
			})
		case <-timeout:
			break collectLoop
		}
	}

	return sources
}
