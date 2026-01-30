package cmd

import (
	"os"
	"testing"

	"github.com/nchapman/lleme/internal/proxy"
)

func TestStopServerNotRunning(t *testing.T) {
	// Use temp directory to isolate from real system state
	tmpDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	defer os.Setenv("HOME", oldHome)
	os.Setenv("HOME", tmpDir)

	// Use a port that's definitely not in use to avoid finding real servers
	oldPort := serverPort
	defer func() { serverPort = oldPort }()
	serverPort = 59999

	// With a temp HOME and unused port, no server should be found
	stopped, err := stopServer()
	if err != nil {
		t.Errorf("stopServer() error = %v, want nil", err)
	}
	if stopped {
		t.Error("stopServer() returned true when server was not running")
	}
}

func TestStopServerStaleState(t *testing.T) {
	// Create temp directory for state file
	tmpDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	defer os.Setenv("HOME", oldHome)
	os.Setenv("HOME", tmpDir)

	// Save state with a PID that doesn't exist
	state := &proxy.ProxyState{
		PID:  99999999, // Very unlikely to be a real process
		Host: "127.0.0.1",
		Port: 11313,
	}
	if err := proxy.SaveProxyState(state); err != nil {
		t.Fatalf("Failed to save state: %v", err)
	}

	// stopServer should handle this gracefully
	stopped, err := stopServer()

	// On Unix, FindProcess always succeeds, but Signal will fail
	// Either way, state should be cleared
	if proxy.GetRunningProxyState() != nil {
		t.Error("stopServer() should clear stale state")
	}

	// We expect either an error (signal failed) or stopped=true (process killed)
	// The important thing is it doesn't panic
	_ = stopped
	_ = err
}
