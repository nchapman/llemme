package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"syscall"
	"time"

	"github.com/nchapman/llemme/internal/proxy"
	"github.com/nchapman/llemme/internal/ui"
	"github.com/spf13/cobra"
)

var (
	stopAll   bool
	stopProxy bool
)

var stopCmd = &cobra.Command{
	Use:     "stop [model]",
	Short:   "Stop a model, all models, or the proxy server",
	GroupID: "server",
	Long: `Stop running models or the proxy server.

Examples:
  llemme stop llama              # Unload a specific model
  llemme stop --all              # Unload all models (keep proxy running)
  llemme stop --proxy            # Stop the proxy and all models`,
	Run: func(cmd *cobra.Command, args []string) {
		state := proxy.GetRunningProxyState()
		if state == nil {
			fmt.Println(ui.Muted("Proxy is not running"))
			return
		}

		proxyURL := fmt.Sprintf("http://%s:%d", state.Host, state.Port)

		if stopProxy {
			stopProxyServer(state)
			return
		}

		if stopAll {
			stopAllModels(proxyURL)
			return
		}

		if len(args) == 0 {
			// No args and no flags - show help
			fmt.Println("Usage: llemme stop [model] [flags]")
			fmt.Println()
			fmt.Println("Stop a specific model:")
			fmt.Println("  llemme stop llama")
			fmt.Println("  llemme stop bartowski/Llama-3.2-3B-Instruct-GGUF:Q4_K_M")
			fmt.Println()
			fmt.Println("Stop all models:")
			fmt.Println("  llemme stop --all")
			fmt.Println()
			fmt.Println("Stop the proxy server:")
			fmt.Println("  llemme stop --proxy")
			return
		}

		// Stop specific model
		modelName := args[0]
		stopModel(proxyURL, modelName)
	},
}

func stopModel(proxyURL, modelName string) {
	// We need to send a request to a management endpoint to unload the model
	// For now, we'll use the /api/stop endpoint
	client := &http.Client{Timeout: 30 * time.Second}

	reqBody, _ := json.Marshal(map[string]string{"model": modelName})
	resp, err := client.Post(proxyURL+"/api/stop", "application/json", bytes.NewReader(reqBody))
	if err != nil {
		fmt.Printf("%s Failed to stop model: %v\n", ui.ErrorMsg("Error:"), err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		fmt.Printf("%s Model '%s' is not loaded\n", ui.ErrorMsg("Error:"), modelName)
		os.Exit(1)
	}

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("%s Failed to stop model: HTTP %d\n", ui.ErrorMsg("Error:"), resp.StatusCode)
		os.Exit(1)
	}

	fmt.Printf("Unloaded %s\n", modelName)
}

func stopAllModels(proxyURL string) {
	client := &http.Client{Timeout: 30 * time.Second}

	resp, err := client.Post(proxyURL+"/api/stop-all", "application/json", nil)
	if err != nil {
		fmt.Printf("%s Failed to stop models: %v\n", ui.ErrorMsg("Error:"), err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	var result struct {
		Stopped int `json:"stopped"`
	}
	json.NewDecoder(resp.Body).Decode(&result)

	if result.Stopped == 0 {
		fmt.Println(ui.Muted("No models were loaded"))
	} else if result.Stopped == 1 {
		fmt.Println("Unloaded 1 model")
	} else {
		fmt.Printf("Unloaded %d models\n", result.Stopped)
	}
}

func stopProxyServer(state *proxy.ProxyState) {
	// First try to gracefully stop via signal
	process, err := os.FindProcess(state.PID)
	if err != nil {
		fmt.Printf("%s Could not find proxy process: %v\n", ui.ErrorMsg("Error:"), err)
		proxy.ClearProxyState()
		return
	}

	// Send SIGTERM for graceful shutdown
	if err := process.Signal(syscall.SIGTERM); err != nil {
		fmt.Printf("%s Could not signal proxy process: %v\n", ui.ErrorMsg("Error:"), err)
		proxy.ClearProxyState()
		return
	}

	// Wait for process to exit
	for range 40 { // 4 seconds max
		time.Sleep(100 * time.Millisecond)
		if err := process.Signal(syscall.Signal(0)); err != nil {
			// Process has exited
			proxy.ClearProxyState()
			fmt.Println("Proxy stopped")
			return
		}
	}

	// Force kill if still running
	process.Kill()
	proxy.ClearProxyState()
	fmt.Println("Proxy stopped")
}

func init() {
	rootCmd.AddCommand(stopCmd)

	stopCmd.Flags().BoolVarP(&stopAll, "all", "a", false, "Stop all loaded models")
	stopCmd.Flags().BoolVar(&stopProxy, "proxy", false, "Stop the proxy server")
}
