package cmd

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/nchapman/llemme/internal/config"
	"github.com/nchapman/llemme/internal/hf"
	"github.com/nchapman/llemme/internal/llama"
	"github.com/nchapman/llemme/internal/proxy"
	"github.com/nchapman/llemme/internal/server"
	"github.com/nchapman/llemme/internal/tui/chat"
	"github.com/nchapman/llemme/internal/ui"
	"github.com/spf13/cobra"
)

var (
	tokens        int
	temperature   float64
	topP          float64
	topK          int
	minP          float64
	repeatPenalty float64
	systemPrompt  string

	// Server options (require model reload)
	ctxSize   int
	gpuLayers int
	threads   int
)

var runCmd = &cobra.Command{
	Use:     "run <model|persona> [prompt]",
	Short:   "Run inference with a model or persona",
	GroupID: "model",
	Long: `Run inference with a model or persona. The first argument can be:

Models:
  - Full name: bartowski/Llama-3.2-3B-Instruct-GGUF:Q4_K_M
  - Partial name: llama (matches if unique)
  - Repo name: Llama-2-7B-GGUF

Personas:
  - Name of a saved persona (see 'llemme persona list')
  - Personas provide saved system prompts and options

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
		promptStartIdx := 1 // Where prompt args begin (shifts if persona has no model)

		// Check if this is a persona (personas take precedence over model names)
		var activePersona *config.Persona
		if config.PersonaExists(modelQuery) {
			persona, err := config.LoadPersona(modelQuery)
			if err != nil {
				fmt.Printf("%s Failed to load persona: %v\n", ui.ErrorMsg("Error:"), err)
				os.Exit(1)
			}
			activePersona = persona

			// Get model from persona or second argument
			if persona.Model != "" {
				modelQuery = persona.Model
			} else if len(args) > 1 {
				modelQuery = args[1]
				promptStartIdx = 2 // Prompt starts after persona and model args
			} else {
				fmt.Printf("%s Persona '%s' has no model. Specify one:\n", ui.ErrorMsg("Error:"), args[0])
				fmt.Printf("  llemme run %s <model> [prompt]\n", args[0])
				os.Exit(1)
			}

			// Apply persona system prompt if not overridden by flag
			if systemPrompt == "" && persona.System != "" {
				systemPrompt = persona.System
			}
		}

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

		// Use the resolved full model name
		modelName := resolvedModel.FullName

		// Build server options from persona and CLI flags
		// Priority: CLI flags > persona options > config defaults
		ctxSizeSet := cmd.Flags().Changed("ctx-size")
		gpuLayersSet := cmd.Flags().Changed("gpu-layers")
		threadsSet := cmd.Flags().Changed("threads")

		// Get persona server options (if any)
		var personaOpts map[string]any
		if activePersona != nil {
			personaOpts = activePersona.GetServerOptions()
		}

		// Only call /api/run if we have options to pass
		if ctxSizeSet || gpuLayersSet || threadsSet || personaOpts != nil {
			opts := &server.RunOptions{
				Options: personaOpts, // Base options from persona
			}
			// CLI flags override persona options
			if ctxSizeSet {
				opts.CtxSize = server.IntPtr(ctxSize)
			}
			if gpuLayersSet {
				opts.GpuLayers = server.IntPtr(gpuLayers)
			}
			if threadsSet {
				opts.Threads = server.IntPtr(threads)
			}
			if err := api.Run(modelName, opts); err != nil {
				fmt.Printf("%s Failed to load model: %v\n", ui.ErrorMsg("Error:"), err)
				os.Exit(1)
			}
		}

		promptArg := ""
		if len(args) > promptStartIdx {
			promptArg = strings.Join(args[promptStartIdx:], " ")
		}

		// Check if input is piped (non-interactive)
		stat, _ := os.Stdin.Stat()
		isPiped := (stat.Mode() & os.ModeCharDevice) == 0

		// Use classic mode for piped input or one-shot prompts
		if isPiped || promptArg != "" {
			// Use classic stdin/stdout chat session
			session := NewChatSession(api, modelName, cfg, activePersona)
			session.SetInitialServerOptions(ctxSize, gpuLayers, threads, ctxSizeSet, gpuLayersSet, threadsSet)
			if temperature != 0 {
				session.options.temp = temperature
			}
			if topP != 0 {
				session.options.topP = topP
			}
			if topK != 0 {
				session.options.topK = topK
			}
			if minP != 0 {
				session.options.minP = minP
			}
			if repeatPenalty != 0 {
				session.options.repeatPenalty = repeatPenalty
			}
			session.Run(promptArg)
			return
		}

		// Launch TUI for interactive mode
		m := chat.New(api, modelName, cfg, activePersona)
		m.SetInitialServerOptions(ctxSize, gpuLayers, threads, ctxSizeSet, gpuLayersSet, threadsSet)
		m.SetSamplingOptions(temperature, topP, minP, repeatPenalty, topK)

		p := tea.NewProgram(m, tea.WithAltScreen())
		m.SetProgram(p)

		if _, err := p.Run(); err != nil {
			fmt.Printf("%s TUI error: %v\n", ui.ErrorMsg("Error:"), err)
			os.Exit(1)
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
		b.WriteString("\n\n  Use 'llemme list' to see downloaded models\n  Use 'llemme search <query>' to find models")
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
			return nil, fmt.Errorf("model '%s/%s' not found on Hugging Face\n\n  Use 'llemme search <query>' to find models", user, repo)
		}
		return nil, fmt.Errorf("failed to check model: %w", err)
	}

	// Check for gated models
	if modelInfo.Gated && cfg.HuggingFace.Token == "" && os.Getenv("HF_TOKEN") == "" {
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

	progressBar := ui.NewProgressBar()
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
	host := cfg.Server.Host
	if host == "" {
		host = "127.0.0.1"
	}
	port := cfg.Server.Port
	if port == 0 {
		port = 11313
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

func init() {
	rootCmd.AddCommand(runCmd)

	// Sampling options (apply per-request)
	runCmd.Flags().Float64VarP(&temperature, "temp", "t", 0, "Temperature")
	runCmd.Flags().Float64Var(&topP, "top-p", 0, "Top-p sampling")
	runCmd.Flags().IntVar(&topK, "top-k", 0, "Top-k sampling")
	runCmd.Flags().Float64Var(&minP, "min-p", 0, "Min-p sampling")
	runCmd.Flags().Float64Var(&repeatPenalty, "repeat-penalty", 0, "Repeat penalty")
	runCmd.Flags().IntVarP(&tokens, "predict", "n", 0, "Max tokens to generate")
	runCmd.Flags().StringVarP(&systemPrompt, "system", "s", "", "System prompt")

	// Server options (affect model loading)
	runCmd.Flags().IntVar(&ctxSize, "ctx-size", 0, "Context size (0 = model default)")
	runCmd.Flags().IntVar(&gpuLayers, "gpu-layers", 0, "GPU layers to offload (0 = auto)")
	runCmd.Flags().IntVar(&threads, "threads", 0, "CPU threads (0 = auto)")
}
