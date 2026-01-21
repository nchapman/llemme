package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/nchapman/lleme/internal/config"
	"github.com/nchapman/lleme/internal/llama"
	"github.com/nchapman/lleme/internal/logs"
	"github.com/nchapman/lleme/internal/proxy"
	"github.com/nchapman/lleme/internal/ui"
	"github.com/spf13/cobra"
)

var (
	serverHost      string
	serverPort      int
	serverMaxModels int
	serverDetach    bool
)

var serverCmd = &cobra.Command{
	Use:     "server",
	Short:   "Manage the proxy server",
	GroupID: "server",
	Long: `Manage the lleme proxy server that routes requests to llama.cpp backends.

The proxy server:
  - Routes requests to the appropriate model backend
  - Automatically loads models on demand
  - Manages LRU eviction when max models is reached
  - Unloads idle models after the configured timeout

Examples:
  lleme server start          # Start in foreground
  lleme server start -d       # Start in background (detached)
  lleme server stop           # Stop the server
  lleme server restart        # Restart the server`,
}

var serverStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the proxy server",
	PreRun: func(cmd *cobra.Command, args []string) {
		if !llama.IsInstalled() {
			fmt.Println("llama.cpp is not installed.")
			if !ui.PromptYesNo("Install now?", true) {
				fmt.Println(ui.Muted("Cancelled"))
				os.Exit(0)
			}

			fmt.Println()
			if _, err := llama.InstallLatest(func(msg string) { fmt.Println(msg) }); err != nil {
				ui.Fatal("Failed to install llama.cpp: %v", err)
			}
			fmt.Println()
		}
	},
	Run: func(cmd *cobra.Command, args []string) {
		// Check if already running
		if existingState := proxy.GetRunningProxyState(); existingState != nil {
			ui.PrintError("Server already running on http://%s:%d (PID %d)",
				existingState.Host, existingState.Port, existingState.PID)
			fmt.Println("Use 'lleme server stop' to stop the existing server first")
			os.Exit(1)
		}

		if serverDetach {
			// Detached mode: spawn daemon with CLI overrides, daemon handles everything else
			startServerDetached()
			return
		}

		// Foreground mode: we ARE the server
		startServerForeground()
	},
}

var serverStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the proxy server",
	Run: func(cmd *cobra.Command, args []string) {
		stopped, err := stopServer()
		if err != nil {
			fmt.Printf("%s %v\n", ui.ErrorMsg("Error:"), err)
			return
		}
		if !stopped {
			fmt.Println(ui.Muted("Server is not running"))
			return
		}
		fmt.Println("Server stopped")
	},
}

var serverRestartCmd = &cobra.Command{
	Use:   "restart",
	Short: "Restart the proxy server",
	Run: func(cmd *cobra.Command, args []string) {
		stopped, _ := stopServer()
		if stopped {
			fmt.Println("Stopped server")
		}

		fmt.Println("Starting server...")

		if serverDetach {
			startServerDetached()
		} else {
			startServerForeground()
		}
	},
}

// stopServer stops the running server and returns whether it was stopped.
func stopServer() (bool, error) {
	state := proxy.GetRunningProxyState()
	if state == nil {
		return false, nil
	}

	process, err := os.FindProcess(state.PID)
	if err != nil {
		proxy.ClearProxyState()
		return false, fmt.Errorf("could not find server process: %w", err)
	}

	if err := process.Signal(syscall.SIGTERM); err != nil {
		proxy.ClearProxyState()
		return false, fmt.Errorf("could not signal server process: %w", err)
	}

	for range 40 { // 4 seconds max
		time.Sleep(100 * time.Millisecond)
		if err := process.Signal(syscall.Signal(0)); err != nil {
			proxy.ClearProxyState()
			return true, nil
		}
	}

	process.Kill()
	proxy.ClearProxyState()
	return true, nil
}

func startServerForeground() {
	// Load config
	cfg, err := config.Load()
	if err != nil {
		ui.Fatal("Failed to load config: %v", err)
	}

	// Build proxy config from app config + CLI overrides
	proxyCfg := proxy.ConfigFromAppConfig(cfg.Server)
	if serverHost != "" {
		proxyCfg.Host = serverHost
	}
	if serverPort != 0 {
		proxyCfg.Port = serverPort
	}
	if serverMaxModels != 0 {
		proxyCfg.MaxModels = serverMaxModels
	}

	// Create and start server (handles orphan cleanup internally)
	server := proxy.NewServer(proxyCfg, cfg)
	if err := server.Start(); err != nil {
		ui.Fatal("Failed to start server: %v", err)
	}

	// Print startup info
	fmt.Printf("Server started on http://%s:%d\n", proxyCfg.Host, proxyCfg.Port)
	fmt.Println()
	fmt.Printf("  %-14s %d\n", "Max models", proxyCfg.MaxModels)
	fmt.Printf("  %-14s %v\n", "Idle timeout", proxyCfg.IdleTimeout)
	fmt.Printf("  %-14s %d-%d\n", "Backend ports", proxyCfg.BackendPortMin, proxyCfg.BackendPortMax)
	fmt.Println()
	fmt.Println(ui.Header("Endpoints"))
	fmt.Printf("  %-12s %s %s\n", "Chat", ui.Muted("POST"), "/v1/chat/completions")
	fmt.Printf("  %-12s %s %s\n", "Completions", ui.Muted("POST"), "/v1/completions")
	fmt.Printf("  %-12s %s %s\n", "Messages", ui.Muted("POST"), "/v1/messages")
	fmt.Printf("  %-12s %s %s\n", "Models", ui.Muted("GET"), "/v1/models")
	fmt.Printf("  %-12s %s %s\n", "Status", ui.Muted("GET"), "/api/status")
	fmt.Println()

	installed, _ := llama.GetInstalledVersion()
	if installed != nil {
		fmt.Println(ui.LlamaCppCredit(installed.TagName))
	}
	fmt.Println(ui.Muted("Press Ctrl+C to stop"))

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	fmt.Println()
	fmt.Println("Shutting down...")

	if err := server.Stop(); err != nil {
		fmt.Printf("%s Failed to stop server cleanly: %v\n", ui.ErrorMsg("Warning:"), err)
	}

	proxy.ClearProxyState()
	fmt.Println("Server stopped")
}

func startServerDetached() {
	executable, err := os.Executable()
	if err != nil {
		ui.Fatal("Failed to get executable path: %v", err)
	}

	// Build args: only pass CLI overrides that were explicitly set
	args := []string{"internal-serve"}
	if serverHost != "" {
		args = append(args, "--host", serverHost)
	}
	if serverPort != 0 {
		args = append(args, "--port", fmt.Sprintf("%d", serverPort))
	}
	if serverMaxModels != 0 {
		args = append(args, "--max-models", fmt.Sprintf("%d", serverMaxModels))
	}

	// Spawn daemon - it handles its own logging, config loading, etc.
	cmd := exec.Command(executable, args...)
	cmd.Env = os.Environ()
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true,
	}

	if err := cmd.Start(); err != nil {
		ui.Fatal("Failed to start server in background: %v", err)
	}

	// Poll for state file (up to 5 seconds)
	logPath := logs.ProxyLogPath()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if state := proxy.GetRunningProxyState(); state != nil {
			fmt.Printf("Server started in background on http://%s:%d (PID %d)\n", state.Host, state.Port, state.PID)
			fmt.Printf("Logs: %s\n", ui.Muted(logPath))
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	fmt.Printf("%s Server may have failed to start. Check logs: %s\n", ui.ErrorMsg("Warning:"), logPath)
}

// internalServeCmd is the daemon process for background serving.
// Fully self-contained: handles its own logging, config, lifecycle, and state.
var internalServeCmd = &cobra.Command{
	Use:    "internal-serve",
	Hidden: true,
	Run: func(cmd *cobra.Command, args []string) {
		// Set up logging - daemon owns its log file
		logFile, err := logs.NewRotatingWriter(logs.ProxyLogPath())
		if err != nil {
			os.Exit(1)
		}
		defer logFile.Close()
		logs.InitLogger(logFile, false)

		// Load config
		cfg, err := config.Load()
		if err != nil {
			logs.Warn("Failed to load config", "error", err)
			os.Exit(1)
		}

		// Build proxy config from app config + CLI overrides
		proxyCfg := proxy.ConfigFromAppConfig(cfg.Server)
		if serverHost != "" {
			proxyCfg.Host = serverHost
		}
		if serverPort != 0 {
			proxyCfg.Port = serverPort
		}
		if serverMaxModels != 0 {
			proxyCfg.MaxModels = serverMaxModels
		}

		// Create and start server (handles orphan cleanup and state persistence internally)
		server := proxy.NewServer(proxyCfg, cfg)
		if err := server.Start(); err != nil {
			logs.Warn("Failed to start server", "error", err)
			os.Exit(1)
		}

		// Wait for shutdown signal
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		server.Stop()
		proxy.ClearProxyState()
	},
}

func init() {
	rootCmd.AddCommand(serverCmd)
	rootCmd.AddCommand(internalServeCmd)

	serverCmd.AddCommand(serverStartCmd)
	serverCmd.AddCommand(serverStopCmd)
	serverCmd.AddCommand(serverRestartCmd)

	serverStartCmd.Flags().StringVarP(&serverHost, "host", "H", "", "Server host (default from config)")
	serverStartCmd.Flags().IntVarP(&serverPort, "port", "p", 0, "Server port (default from config)")
	serverStartCmd.Flags().IntVar(&serverMaxModels, "max-models", 0, "Maximum concurrent models (default from config)")
	serverStartCmd.Flags().BoolVarP(&serverDetach, "detach", "d", false, "Run server in background")

	serverRestartCmd.Flags().StringVarP(&serverHost, "host", "H", "", "Server host (default from config)")
	serverRestartCmd.Flags().IntVarP(&serverPort, "port", "p", 0, "Server port (default from config)")
	serverRestartCmd.Flags().IntVar(&serverMaxModels, "max-models", 0, "Maximum concurrent models (default from config)")
	serverRestartCmd.Flags().BoolVarP(&serverDetach, "detach", "d", false, "Run server in background")

	internalServeCmd.Flags().StringVar(&serverHost, "host", "", "")
	internalServeCmd.Flags().IntVar(&serverPort, "port", 0, "")
	internalServeCmd.Flags().IntVar(&serverMaxModels, "max-models", 0, "")
}
