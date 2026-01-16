package cmd

import (
	"bufio"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/nchapman/lemme/internal/config"
	"github.com/nchapman/lemme/internal/llama"
	"github.com/nchapman/lemme/internal/proxy"
	"github.com/nchapman/lemme/internal/server"
	"github.com/nchapman/lemme/internal/ui"
	"github.com/spf13/cobra"
)

var (
	ctxLen        int
	tokens        int
	temperature   float64
	topP          float64
	topK          int
	repeatPenalty float64
	gpuLayers     int
	threads       int
	systemPrompt  string
)

var runCmd = &cobra.Command{
	Use:   "run <model> [prompt]",
	Short: "Run inference with a model via the proxy server",
	Long: `Run inference with a model. The model can be specified using:
  - Full name: TheBloke/Llama-2-7B-GGUF:Q4_K_M
  - Partial name: llama (matches if unique)
  - Repo name: Llama-2-7B-GGUF

The proxy server will be auto-started if not running.
Models are loaded on-demand and unloaded after idle timeout.`,
	Args: cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.Load()
		if err != nil {
			fmt.Printf("%s Failed to load config: %v\n", ui.ErrorMsg("Error:"), err)
			os.Exit(1)
		}

		// Step 1: Ensure llama.cpp is installed
		if !llama.IsInstalled() {
			if err := ensureLlamaInstalled(); err != nil {
				fmt.Printf("%s %v\n", ui.ErrorMsg("Error:"), err)
				os.Exit(1)
			}
		}

		modelQuery := args[0]

		// Step 2: Validate model exists before starting proxy
		resolvedModel, err := validateModel(modelQuery)
		if err != nil {
			fmt.Printf("%s %v\n", ui.ErrorMsg("Error:"), err)
			os.Exit(1)
		}

		// Step 3: Ensure proxy is running
		proxyURL, err := ensureProxyRunning(cfg)
		if err != nil {
			fmt.Printf("%s Failed to start proxy: %v\n", ui.ErrorMsg("Error:"), err)
			os.Exit(1)
		}

		// Create API client pointing to proxy
		api := server.NewAPIClientFromURL(proxyURL)

		// Check health
		if err := api.Health(); err != nil {
			fmt.Printf("%s Proxy health check failed: %v\n", ui.ErrorMsg("Error:"), err)
			os.Exit(1)
		}

		promptArg := ""
		if len(args) > 1 {
			promptArg = strings.Join(args[1:], " ")
		}

		// Use the resolved full model name
		modelName := resolvedModel.FullName

		if promptArg != "" {
			runOneShot(api, modelName, promptArg, cfg, true)
		} else {
			runTUI(api, modelName, cfg)
		}
	},
}

// ensureLlamaInstalled prompts the user to install llama.cpp if not present
func ensureLlamaInstalled() error {
	fmt.Println("llama.cpp is not installed.")
	fmt.Print("Install now? [Y/n] ")

	var response string
	fmt.Scanln(&response)
	response = strings.TrimSpace(strings.ToLower(response))

	if response != "" && response != "y" && response != "yes" {
		return fmt.Errorf("llama.cpp is required to run models")
	}

	fmt.Println()
	_, err := llama.InstallLatest()
	if err != nil {
		return fmt.Errorf("failed to install llama.cpp: %w", err)
	}

	fmt.Println()
	return nil
}

// validateModel checks if a model exists before starting the proxy
func validateModel(query string) (*proxy.DownloadedModel, error) {
	resolver := proxy.NewModelResolver()
	result, err := resolver.Resolve(query)
	if err != nil {
		return nil, err
	}

	// No models downloaded at all
	models, _ := resolver.ListDownloadedModels()
	if len(models) == 0 {
		return nil, fmt.Errorf("no models downloaded yet\n\n  Use 'lemme search <query>' to find models\n  Use 'lemme pull <model>' to download one")
	}

	// Model resolved successfully
	if result.Model != nil {
		return result.Model, nil
	}

	// Ambiguous match
	if len(result.Matches) > 1 {
		var b strings.Builder
		b.WriteString(fmt.Sprintf("'%s' matches multiple models:\n\n", query))
		for _, m := range result.Matches {
			b.WriteString(fmt.Sprintf("  %s\n", m.FullName))
		}
		b.WriteString("\nSpecify the full model name to continue")
		return nil, fmt.Errorf("%s", b.String())
	}

	// No match - show suggestions
	var b strings.Builder
	b.WriteString(fmt.Sprintf("no model matches '%s'", query))
	if len(result.Suggestions) > 0 {
		b.WriteString("\n\nDid you mean:\n")
		for _, s := range result.Suggestions {
			b.WriteString(fmt.Sprintf("  %s\n", s.FullName))
		}
	} else {
		b.WriteString("\n\n  Use 'lemme list' to see downloaded models\n  Use 'lemme search <query>' to find models")
	}
	return nil, fmt.Errorf("%s", b.String())
}

// ensureProxyRunning starts the proxy if not already running and returns its URL
func ensureProxyRunning(cfg *config.Config) (string, error) {
	// Check if proxy is already running
	if state := proxy.GetRunningProxyState(); state != nil {
		return fmt.Sprintf("http://%s:%d", state.Host, state.Port), nil
	}

	// Need to start proxy
	fmt.Println(ui.Muted("Starting proxy..."))

	executable, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("failed to get executable path: %w", err)
	}

	// Use config values for proxy
	host := cfg.Proxy.Host
	if host == "" {
		host = "127.0.0.1"
	}
	port := cfg.Proxy.Port
	if port == 0 {
		port = 8080
	}

	args := []string{
		"internal-serve",
		"--host", host,
		"--port", fmt.Sprintf("%d", port),
	}

	cmd := exec.Command(executable, args...)
	cmd.Env = os.Environ()

	// Redirect output to log file
	logFile := config.BinPath() + "/proxy.log"
	log, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return "", fmt.Errorf("failed to open log file: %w", err)
	}
	cmd.Stdout = log
	cmd.Stderr = log

	// Detach from parent process
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true,
	}

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("failed to start proxy: %w", err)
	}

	// Wait for proxy to become ready
	proxyURL := fmt.Sprintf("http://%s:%d", host, port)
	client := &http.Client{Timeout: 2 * time.Second}

	for range 30 {
		time.Sleep(200 * time.Millisecond)
		resp, err := client.Get(proxyURL + "/health")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return proxyURL, nil
			}
		}
	}

	return "", fmt.Errorf("proxy did not become ready")
}

func runTUI(api *server.APIClient, model string, cfg *config.Config) {
	chat := initialChatModel(api, model, cfg.Temperature, cfg.TopP)
	p := tea.NewProgram(chat)

	if _, err := p.Run(); err != nil {
		fmt.Printf("%s Failed to run TUI: %v\n", ui.ErrorMsg("Error:"), err)
	}
}

func runInteractive(api *server.APIClient, model string, cfg *config.Config) {
	fmt.Printf("\n%s  %s\n", ui.Box(model), ui.Muted("Ctrl+D to exit, Ctrl+C to stop generation"))

	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Print("You: ")
		input, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println()
			break
		}

		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}

		if input == "/bye" {
			fmt.Println(ui.Muted("Goodbye!"))
			return
		}

		messages := []server.ChatMessage{
			{Role: "user", Content: input},
		}

		req := &server.ChatCompletionRequest{
			Model:       model,
			Messages:    messages,
			Stream:      true,
			Temperature: cfg.Temperature,
			TopP:        cfg.TopP,
			MaxTokens:   -1,
		}

		fmt.Print("AI: ")

		spinner := ui.NewSpinner("")
		spinner.Start("Thinking...")
		spinnerStarted := true

		time.Sleep(10 * time.Millisecond)

		if err := api.StreamChatCompletion(req, func(content string) {
			if spinnerStarted {
				spinner.Stop(true, "")
				spinnerStarted = false
			}
			fmt.Print(content)
		}); err != nil {
			if spinnerStarted {
				spinner.Stop(false, "")
			}
			fmt.Printf("\n%s %v\n", ui.ErrorMsg("Error:"), err)
			continue
		}

		fmt.Println()
		fmt.Println()
	}
}

func runOneShot(api *server.APIClient, model, prompt string, cfg *config.Config, exitAfter bool) {
	messages := []server.ChatMessage{
		{Role: "user", Content: prompt},
	}

	req := &server.ChatCompletionRequest{
		Model:       model,
		Messages:    messages,
		Stream:      true,
		Temperature: cfg.Temperature,
		TopP:        cfg.TopP,
		MaxTokens:   tokens,
	}

	if err := api.StreamChatCompletion(req, func(content string) {
		fmt.Print(content)
	}); err != nil {
		fmt.Printf("\n%s %v\n", ui.ErrorMsg("Error:"), err)
		os.Exit(1)
	}

	fmt.Println()

	if exitAfter {
		return
	}

	fmt.Println()
	runInteractive(api, model, cfg)
}

func isPipedInput() bool {
	stat, _ := os.Stdin.Stat()
	return (stat.Mode() & os.ModeCharDevice) == 0
}

func init() {
	rootCmd.AddCommand(runCmd)

	runCmd.Flags().IntVarP(&ctxLen, "ctx", "c", 0, "Context length")
	runCmd.Flags().IntVarP(&tokens, "predict", "n", 0, "Max tokens to generate")
	runCmd.Flags().Float64VarP(&temperature, "temp", "t", 0, "Temperature")
	runCmd.Flags().Float64Var(&topP, "top-p", 0, "Top-p sampling")
	runCmd.Flags().IntVar(&topK, "top-k", 0, "Top-k sampling")
	runCmd.Flags().Float64Var(&repeatPenalty, "repeat-penalty", 0, "Repeat penalty")
	runCmd.Flags().IntVar(&gpuLayers, "gpu-layers", 0, "Layers to offload to GPU (-1 = all)")
	runCmd.Flags().IntVar(&threads, "threads", 0, "CPU threads to use")
	runCmd.Flags().StringVar(&systemPrompt, "system", "", "System prompt")
}
