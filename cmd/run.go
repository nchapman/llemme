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

	"github.com/nchapman/lemme/internal/config"
	"github.com/nchapman/lemme/internal/hf"
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

		// Step 2: Validate model exists (or offer to pull)
		resolvedModel, err := validateModel(modelQuery, cfg)
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

		runChat(api, modelName, promptArg, cfg)
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

// validateModel checks if a model exists, offering to pull it if not found
func validateModel(query string, cfg *config.Config) (*proxy.DownloadedModel, error) {
	resolver := proxy.NewModelResolver()
	result, err := resolver.Resolve(query)
	if err != nil {
		return nil, err
	}

	// Model resolved successfully
	if result.Model != nil {
		return result.Model, nil
	}

	// Ambiguous match - user needs to be more specific
	if len(result.Matches) > 1 {
		var b strings.Builder
		b.WriteString(fmt.Sprintf("'%s' matches multiple models:\n\n", query))
		for _, m := range result.Matches {
			b.WriteString(fmt.Sprintf("  %s\n", m.FullName))
		}
		b.WriteString("\nSpecify the full model name to continue")
		return nil, fmt.Errorf("%s", b.String())
	}

	// Model not found locally - check if it looks like a HuggingFace ref
	user, repo, quant, parseErr := parseModelRef(query)
	if parseErr != nil {
		// Not a valid model ref format, show suggestions
		return nil, modelNotFoundError(query, result.Suggestions)
	}

	// Try to pull from HuggingFace
	pulledModel, err := offerToPull(cfg, user, repo, quant)
	if err != nil {
		return nil, err
	}

	return pulledModel, nil
}

// modelNotFoundError returns a helpful error for models that aren't found
func modelNotFoundError(query string, suggestions []proxy.DownloadedModel) error {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("no model matches '%s'", query))
	if len(suggestions) > 0 {
		b.WriteString("\n\nDid you mean:\n")
		for _, s := range suggestions {
			b.WriteString(fmt.Sprintf("  %s\n", s.FullName))
		}
	} else {
		b.WriteString("\n\n  Use 'lemme list' to see downloaded models\n  Use 'lemme search <query>' to find models")
	}
	return fmt.Errorf("%s", b.String())
}

// offerToPull checks HuggingFace and offers to download a model
func offerToPull(cfg *config.Config, user, repo, quant string) (*proxy.DownloadedModel, error) {
	client := hf.NewClient(cfg)

	// Check if model exists on HuggingFace
	modelInfo, err := client.GetModel(user, repo)
	if err != nil {
		if strings.Contains(err.Error(), "404") {
			return nil, fmt.Errorf("model '%s/%s' not found on Hugging Face\n\n  Use 'lemme search <query>' to find models", user, repo)
		}
		return nil, fmt.Errorf("failed to check model: %w", err)
	}

	// Check for gated models
	if modelInfo.Gated && cfg.HFToken == "" && os.Getenv("HF_TOKEN") == "" {
		return nil, fmt.Errorf("model '%s/%s' requires authentication\n\n  Get a token at https://huggingface.co/settings/tokens\n  Then set: export HF_TOKEN=hf_xxxxx", user, repo)
	}

	// Get available quantizations
	files, err := client.ListFiles(user, repo, "main")
	if err != nil {
		return nil, fmt.Errorf("failed to list model files: %w", err)
	}

	quants := hf.ExtractQuantizations(files)
	if len(quants) == 0 {
		return nil, fmt.Errorf("'%s/%s' contains no GGUF files\n\n  Try: %s-GGUF/%s", user, repo, user, repo)
	}

	// Select quantization
	if quant == "" {
		quant = hf.GetBestQuantization(quants)
	} else {
		if _, found := hf.FindQuantization(quants, quant); !found {
			var b strings.Builder
			b.WriteString(fmt.Sprintf("quantization '%s' not found\n\nAvailable:\n", quant))
			for _, q := range hf.SortQuantizations(quants) {
				b.WriteString(fmt.Sprintf("  %s (%s)\n", q.Name, ui.FormatBytes(q.Size)))
			}
			return nil, fmt.Errorf("%s", b.String())
		}
	}

	selectedQuant, _ := hf.FindQuantization(quants, quant)

	// Prompt user to download
	fmt.Printf("Model not downloaded. Pull %s/%s:%s (%s)? [Y/n] ", user, repo, quant, ui.FormatBytes(selectedQuant.Size))
	var response string
	fmt.Scanln(&response)
	response = strings.TrimSpace(strings.ToLower(response))

	if response != "" && response != "y" && response != "yes" {
		return nil, fmt.Errorf("model required to continue")
	}

	// Download the model
	fmt.Println()
	modelDir := hf.GetModelPath(user, repo)
	modelPath := hf.GetModelFilePath(user, repo, quant)

	if err := os.MkdirAll(modelDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create model directory: %w", err)
	}

	progressBar := ui.NewProgressBar("", selectedQuant.Size)
	progressBar.Start("", selectedQuant.Size)

	downloaderWithProgress := hf.NewDownloaderWithProgress(client, func(downloaded, total int64, speed float64, eta time.Duration) {
		progressBar.Update(downloaded)
	})

	_, err = downloaderWithProgress.DownloadModel(user, repo, "main", selectedQuant.File, modelPath)
	if err != nil {
		progressBar.Stop()
		return nil, fmt.Errorf("failed to download: %w", err)
	}

	progressBar.Finish(fmt.Sprintf("Downloaded %s/%s:%s", user, repo, quant))
	fmt.Println()

	// Return the downloaded model info
	return &proxy.DownloadedModel{
		User:      user,
		Repo:      repo,
		Quant:     quant,
		FullName:  fmt.Sprintf("%s/%s:%s", user, repo, quant),
		ModelPath: modelPath,
	}, nil
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

func runChat(api *server.APIClient, model, initialPrompt string, cfg *config.Config) {
	// Check if input is piped (non-interactive)
	stat, _ := os.Stdin.Stat()
	isPiped := (stat.Mode() & os.ModeCharDevice) == 0

	reader := bufio.NewReader(os.Stdin)
	messages := []server.ChatMessage{}

	// Add system prompt (use default if not specified)
	sysPrompt := systemPrompt
	if sysPrompt == "" {
		sysPrompt = "You are a helpful assistant. Respond directly and conversationally to the user."
	}
	messages = append(messages, server.ChatMessage{Role: "system", Content: sysPrompt})

	if !isPiped {
		fmt.Printf("\n%s  %s\n\n", ui.Box(model), ui.Muted("Ctrl+D to exit"))
	}

	// Handle initial prompt if provided
	if initialPrompt != "" {
		if !isPiped {
			fmt.Printf("%s %s\n\n", ui.Muted("You:"), initialPrompt)
		}
		messages = append(messages, server.ChatMessage{Role: "user", Content: initialPrompt})
		response := streamResponse(api, model, messages, cfg, !isPiped)
		if response != "" {
			messages = append(messages, server.ChatMessage{Role: "assistant", Content: response})
		}
		if isPiped {
			return // Exit after one-shot when piped
		}
		fmt.Println()
	}

	// Interactive loop (skip if piped with no initial prompt)
	if isPiped {
		return
	}

	for {
		fmt.Print(ui.Muted("You: "))
		input, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println()
			break
		}

		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}

		if input == "/bye" || input == "/exit" || input == "/quit" {
			fmt.Println(ui.Muted("Goodbye!"))
			return
		}

		fmt.Println()
		messages = append(messages, server.ChatMessage{Role: "user", Content: input})
		response := streamResponse(api, model, messages, cfg, true)
		if response != "" {
			messages = append(messages, server.ChatMessage{Role: "assistant", Content: response})
		}
		fmt.Println()
	}
}

func streamResponse(api *server.APIClient, model string, messages []server.ChatMessage, cfg *config.Config, showSpinner bool) string {
	req := &server.ChatCompletionRequest{
		Model:       model,
		Messages:    messages,
		Stream:      true,
		Temperature: cfg.Temperature,
		TopP:        cfg.TopP,
		MaxTokens:   tokens,
	}

	var spinner *ui.Spinner
	spinnerRunning := false
	if showSpinner {
		spinner = ui.NewSpinner("")
		spinner.Start("Thinking...")
		spinnerRunning = true
	}

	var fullResponse strings.Builder

	err := api.StreamChatCompletion(req, func(content string) {
		if spinnerRunning {
			spinner.Stop(true, "")
			spinnerRunning = false
		}
		fullResponse.WriteString(content)
		fmt.Print(content)
	})

	if spinnerRunning {
		spinner.Stop(false, "")
	}

	if err != nil {
		fmt.Printf("\n%s %v\n", ui.ErrorMsg("Error:"), err)
		return ""
	}

	fmt.Println()
	return fullResponse.String()
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
