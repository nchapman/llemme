package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/nchapman/llemme/internal/config"
	"github.com/nchapman/llemme/internal/llama"
	"github.com/nchapman/llemme/internal/proxy"
	"github.com/nchapman/llemme/internal/ui"
	"github.com/spf13/cobra"
)

var (
	serveHost     string
	servePort     int
	serveMaxModel int
	detach        bool
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the llemme proxy server",
	Long: `Start the llemme proxy server that manages multiple llama.cpp backends.

The proxy server:
  - Routes requests to the appropriate model backend
  - Automatically loads models on demand
  - Manages LRU eviction when max models is reached
  - Unloads idle models after the configured timeout`,
	PreRun: func(cmd *cobra.Command, args []string) {
		if !llama.IsInstalled() {
			fmt.Println("llama.cpp is not installed.")
			fmt.Print("Install now? [Y/n] ")

			var response string
			fmt.Scanln(&response)
			response = strings.TrimSpace(strings.ToLower(response))

			if response != "" && response != "y" && response != "yes" {
				fmt.Println(ui.Muted("Cancelled"))
				os.Exit(0)
			}

			fmt.Println()
			if _, err := llama.InstallLatest(); err != nil {
				fmt.Printf("%s Failed to install llama.cpp: %v\n", ui.ErrorMsg("Error:"), err)
				os.Exit(1)
			}
			fmt.Println()
		}
	},
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.Load()
		if err != nil {
			fmt.Printf("%s Failed to load config: %v\n", ui.ErrorMsg("Error:"), err)
			os.Exit(1)
		}

		// Check if proxy is already running
		if existingState := proxy.GetRunningProxyState(); existingState != nil {
			fmt.Printf("%s Proxy already running on http://%s:%d (PID %d)\n",
				ui.ErrorMsg("Error:"), existingState.Host, existingState.Port, existingState.PID)
			fmt.Println("Use 'llemme stop --proxy' to stop the existing proxy first")
			os.Exit(1)
		}

		// Build proxy config from app config and CLI flags
		proxyCfg := proxy.ConfigFromAppConfig(
			cfg.Proxy.Host,
			cfg.Proxy.Port,
			cfg.Proxy.MaxModels,
			cfg.Proxy.IdleTimeoutMins,
			cfg.Proxy.BackendPortMin,
			cfg.Proxy.BackendPortMax,
			cfg.Proxy.StartupTimeoutS,
		)

		// CLI flags override config
		if serveHost != "" {
			proxyCfg.Host = serveHost
		}
		if servePort != 0 {
			proxyCfg.Port = servePort
		}
		if serveMaxModel != 0 {
			proxyCfg.MaxModels = serveMaxModel
		}

		if detach {
			// Start proxy in background
			startProxyDetached(proxyCfg, cfg)
			return
		}

		// Start proxy in foreground
		startProxyForeground(proxyCfg, cfg)
	},
}

func startProxyForeground(proxyCfg *proxy.Config, appCfg *config.Config) {
	server := proxy.NewServer(proxyCfg, appCfg)

	if err := server.Start(); err != nil {
		fmt.Printf("%s Failed to start proxy: %v\n", ui.ErrorMsg("Error:"), err)
		os.Exit(1)
	}

	// Save state
	state := &proxy.ProxyState{
		PID:       os.Getpid(),
		Host:      proxyCfg.Host,
		Port:      proxyCfg.Port,
		StartedAt: time.Now(),
	}
	if err := proxy.SaveProxyState(state); err != nil {
		fmt.Printf("%s Failed to save proxy state: %v\n", ui.ErrorMsg("Warning:"), err)
	}

	fmt.Printf("Proxy started on http://%s:%d\n", proxyCfg.Host, proxyCfg.Port)
	fmt.Println()
	fmt.Printf("  %-14s %d\n", "Max models", proxyCfg.MaxModels)
	fmt.Printf("  %-14s %v\n", "Idle timeout", proxyCfg.IdleTimeout)
	fmt.Printf("  %-14s %d-%d\n", "Backend ports", proxyCfg.BackendPortMin, proxyCfg.BackendPortMax)
	fmt.Println()
	fmt.Println(ui.Header("Endpoints"))
	fmt.Printf("  %-12s %s %s\n", "Chat", ui.Muted("POST"), "/v1/chat/completions")
	fmt.Printf("  %-12s %s %s\n", "Completions", ui.Muted("POST"), "/v1/completions")
	fmt.Printf("  %-12s %s %s\n", "Models", ui.Muted("GET"), "/v1/models")
	fmt.Printf("  %-12s %s %s\n", "Status", ui.Muted("GET"), "/api/status")
	fmt.Println()

	installed, _ := llama.GetInstalledVersion()
	if installed != nil {
		fmt.Println(ui.LlamaCppCredit(installed.TagName))
	}
	fmt.Println(ui.Muted("Press Ctrl+C to stop"))

	// Wait for interrupt
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	fmt.Println()
	fmt.Println("Shutting down...")

	if err := server.Stop(); err != nil {
		fmt.Printf("%s Failed to stop proxy cleanly: %v\n", ui.ErrorMsg("Warning:"), err)
	}

	proxy.ClearProxyState()
	fmt.Println("Proxy stopped")
}

func startProxyDetached(proxyCfg *proxy.Config, _ *config.Config) {
	// Re-run this binary with internal-serve command
	executable, err := os.Executable()
	if err != nil {
		fmt.Printf("%s Failed to get executable path: %v\n", ui.ErrorMsg("Error:"), err)
		os.Exit(1)
	}

	args := []string{
		"internal-serve",
		"--host", proxyCfg.Host,
		"--port", fmt.Sprintf("%d", proxyCfg.Port),
		"--max-models", fmt.Sprintf("%d", proxyCfg.MaxModels),
	}

	cmd := exec.Command(executable, args...)
	cmd.Env = os.Environ()

	// Redirect output to log file
	logFile := config.BinPath() + "/proxy.log"
	log, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		fmt.Printf("%s Failed to open log file: %v\n", ui.ErrorMsg("Error:"), err)
		os.Exit(1)
	}
	cmd.Stdout = log
	cmd.Stderr = log

	// Detach from parent process
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true,
	}

	if err := cmd.Start(); err != nil {
		fmt.Printf("%s Failed to start proxy in background: %v\n", ui.ErrorMsg("Error:"), err)
		os.Exit(1)
	}

	// Wait a moment for it to start
	time.Sleep(500 * time.Millisecond)

	// Check if it started successfully
	if state := proxy.GetRunningProxyState(); state != nil {
		fmt.Printf("Proxy started in background on http://%s:%d (PID %d)\n", state.Host, state.Port, state.PID)
		fmt.Printf("Logs: %s\n", ui.Muted(logFile))
	} else {
		fmt.Printf("%s Proxy may have failed to start. Check logs: %s\n", ui.ErrorMsg("Warning:"), logFile)
	}
}

// internalServeCmd is a hidden command used for background serving
var internalServeCmd = &cobra.Command{
	Use:    "internal-serve",
	Hidden: true,
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.Load()
		if err != nil {
			os.Exit(1)
		}

		proxyCfg := proxy.ConfigFromAppConfig(
			cfg.Proxy.Host,
			cfg.Proxy.Port,
			cfg.Proxy.MaxModels,
			cfg.Proxy.IdleTimeoutMins,
			cfg.Proxy.BackendPortMin,
			cfg.Proxy.BackendPortMax,
			cfg.Proxy.StartupTimeoutS,
		)

		if serveHost != "" {
			proxyCfg.Host = serveHost
		}
		if servePort != 0 {
			proxyCfg.Port = servePort
		}
		if serveMaxModel != 0 {
			proxyCfg.MaxModels = serveMaxModel
		}

		server := proxy.NewServer(proxyCfg, cfg)

		if err := server.Start(); err != nil {
			os.Exit(1)
		}

		state := &proxy.ProxyState{
			PID:       os.Getpid(),
			Host:      proxyCfg.Host,
			Port:      proxyCfg.Port,
			StartedAt: time.Now(),
		}
		proxy.SaveProxyState(state)

		// Wait for signal
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		server.Stop()
		proxy.ClearProxyState()
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)
	rootCmd.AddCommand(internalServeCmd)

	serveCmd.Flags().StringVar(&serveHost, "host", "", "Proxy host (default from config)")
	serveCmd.Flags().IntVar(&servePort, "port", 0, "Proxy port (default from config)")
	serveCmd.Flags().IntVar(&serveMaxModel, "max-models", 0, "Maximum concurrent models (default from config)")
	serveCmd.Flags().BoolVar(&detach, "detach", false, "Run proxy in background")

	internalServeCmd.Flags().StringVar(&serveHost, "host", "", "")
	internalServeCmd.Flags().IntVar(&servePort, "port", 0, "")
	internalServeCmd.Flags().IntVar(&serveMaxModel, "max-models", 0, "")
}
