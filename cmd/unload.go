package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/nchapman/llemme/internal/proxy"
	"github.com/nchapman/llemme/internal/ui"
	"github.com/spf13/cobra"
)

var unloadAll bool

var unloadCmd = &cobra.Command{
	Use:     "unload [model]",
	Short:   "Unload a running model",
	GroupID: "model",
	Long: `Unload a model from the proxy server.

Examples:
  llemme unload llama                                    # Unload by name
  llemme unload bartowski/Llama-3.2-3B-Instruct-GGUF     # Unload by full name
  llemme unload --all                                    # Unload all models`,
	Run: func(cmd *cobra.Command, args []string) {
		state := proxy.GetRunningProxyState()
		if state == nil {
			fmt.Println(ui.Muted("Server is not running"))
			return
		}

		proxyURL := fmt.Sprintf("http://%s:%d", state.Host, state.Port)

		if unloadAll {
			unloadAllModels(proxyURL)
			return
		}

		if len(args) == 0 {
			fmt.Println("Usage: llemme unload [model] [flags]")
			fmt.Println()
			fmt.Println("Unload a specific model:")
			fmt.Println("  llemme unload llama")
			fmt.Println("  llemme unload bartowski/Llama-3.2-3B-Instruct-GGUF:Q4_K_M")
			fmt.Println()
			fmt.Println("Unload all models:")
			fmt.Println("  llemme unload --all")
			return
		}

		modelName := args[0]
		unloadModel(proxyURL, modelName)
	},
}

func unloadModel(proxyURL, modelName string) {
	client := &http.Client{Timeout: 30 * time.Second}

	reqBody, err := json.Marshal(map[string]string{"model": modelName})
	if err != nil {
		fmt.Printf("%s Failed to encode request: %v\n", ui.ErrorMsg("Error:"), err)
		os.Exit(1)
	}
	resp, err := client.Post(proxyURL+"/api/stop", "application/json", bytes.NewReader(reqBody))
	if err != nil {
		fmt.Printf("%s Failed to unload model: %v\n", ui.ErrorMsg("Error:"), err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		fmt.Printf("%s Model '%s' is not loaded\n", ui.ErrorMsg("Error:"), modelName)
		os.Exit(1)
	}

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("%s Failed to unload model: HTTP %d\n", ui.ErrorMsg("Error:"), resp.StatusCode)
		os.Exit(1)
	}

	fmt.Printf("Unloaded %s\n", modelName)
}

func unloadAllModels(proxyURL string) {
	client := &http.Client{Timeout: 30 * time.Second}

	resp, err := client.Post(proxyURL+"/api/stop-all", "application/json", nil)
	if err != nil {
		fmt.Printf("%s Failed to unload models: %v\n", ui.ErrorMsg("Error:"), err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("%s Failed to unload models: HTTP %d\n", ui.ErrorMsg("Error:"), resp.StatusCode)
		os.Exit(1)
	}

	var result struct {
		Stopped int `json:"stopped"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		fmt.Printf("%s Failed to parse response: %v\n", ui.ErrorMsg("Error:"), err)
		os.Exit(1)
	}

	switch result.Stopped {
	case 0:
		fmt.Println(ui.Muted("No models were loaded"))
	case 1:
		fmt.Println("Unloaded 1 model")
	default:
		fmt.Printf("Unloaded %d models\n", result.Stopped)
	}
}

func init() {
	rootCmd.AddCommand(unloadCmd)

	unloadCmd.Flags().BoolVarP(&unloadAll, "all", "a", false, "Unload all loaded models")
}
