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
	"github.com/nchapman/llemme/internal/logs"
	"github.com/nchapman/llemme/internal/proxy"
	"github.com/nchapman/llemme/internal/ui"
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
	Long: `Manage the llemme proxy server that routes requests to llama.cpp backends.

The proxy server:
  - Routes requests to the appropriate model backend
  - Automatically loads models on demand
  - Manages LRU eviction when max models is reached
  - Unloads idle models after the configured timeout

Examples:
  llemme server start          # Start in foreground
  llemme server start -d       # Start in background (detached)
  llemme server stop           # Stop the server
  llemme server restart        # Restart the server`,
}

var serverStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the proxy server",
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

		if existingState := proxy.GetRunningProxyState(); existingState != nil {
			fmt.Printf("%s Server already running on http://%s:%d (PID %d)\n",
				ui.ErrorMsg("Error:"), existingState.Host, existingState.Port, existingState.PID)
			fmt.Println("Use 'llemme server stop' to stop the existing server first")
			os.Exit(1)
		}

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

		if serverDetach {
			startServerDetached(proxyCfg)
			return
		}

		startServerForeground(proxyCfg, cfg)
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

		cfg, err := config.Load()
		if err != nil {
			fmt.Printf("%s Failed to load config: %v\n", ui.ErrorMsg("Error:"), err)
			os.Exit(1)
		}

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

		if serverDetach {
			startServerDetached(proxyCfg)
		} else {
			startServerForeground(proxyCfg, cfg)
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

func startServerForeground(proxyCfg *proxy.Config, appCfg *config.Config) {
	server := proxy.NewServer(proxyCfg, appCfg)

	if err := server.Start(); err != nil {
		fmt.Printf("%s Failed to start server: %v\n", ui.ErrorMsg("Error:"), err)
		os.Exit(1)
	}

	state := &proxy.ProxyState{
		PID:       os.Getpid(),
		Host:      proxyCfg.Host,
		Port:      proxyCfg.Port,
		StartedAt: time.Now(),
	}
	if err := proxy.SaveProxyState(state); err != nil {
		fmt.Printf("%s Failed to save server state: %v\n", ui.ErrorMsg("Warning:"), err)
	}

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

func startServerDetached(proxyCfg *proxy.Config) {
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

	logWriter, err := logs.NewRotatingWriter(logs.ProxyLogPath())
	if err != nil {
		fmt.Printf("%s Failed to open log file: %v\n", ui.ErrorMsg("Error:"), err)
		os.Exit(1)
	}
	cmd.Stdout = logWriter
	cmd.Stderr = logWriter

	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true,
	}

	if err := cmd.Start(); err != nil {
		logWriter.Close()
		fmt.Printf("%s Failed to start server in background: %v\n", ui.ErrorMsg("Error:"), err)
		os.Exit(1)
	}

	logWriter.Close()

	time.Sleep(500 * time.Millisecond)

	logPath := logs.ProxyLogPath()
	if state := proxy.GetRunningProxyState(); state != nil {
		fmt.Printf("Server started in background on http://%s:%d (PID %d)\n", state.Host, state.Port, state.PID)
		fmt.Printf("Logs: %s\n", ui.Muted(logPath))
	} else {
		fmt.Printf("%s Server may have failed to start. Check logs: %s\n", ui.ErrorMsg("Warning:"), logPath)
	}
}

// internalServeCmd is a hidden command used for background serving.
// Output goes to the log file, so errors are logged for debugging.
var internalServeCmd = &cobra.Command{
	Use:    "internal-serve",
	Hidden: true,
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.Load()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
			os.Exit(1)
		}

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

		server := proxy.NewServer(proxyCfg, cfg)

		if err := server.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to start server: %v\n", err)
			os.Exit(1)
		}

		state := &proxy.ProxyState{
			PID:       os.Getpid(),
			Host:      proxyCfg.Host,
			Port:      proxyCfg.Port,
			StartedAt: time.Now(),
		}
		if err := proxy.SaveProxyState(state); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to save state: %v\n", err)
		}

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
