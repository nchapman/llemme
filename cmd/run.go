package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/nchapman/gollama/internal/config"
	"github.com/nchapman/gollama/internal/hf"
	"github.com/nchapman/gollama/internal/llama"
	"github.com/nchapman/gollama/internal/server"
	"github.com/nchapman/gollama/internal/ui"
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
	Use:   "run <user/repo>[:quant] [prompt]",
	Short: "Run inference with a model via server",
	Args:  cobra.MinimumNArgs(1),
	PreRun: func(cmd *cobra.Command, args []string) {
		if !llama.IsInstalled() {
			fmt.Printf("%s llama.cpp is not installed\n", ui.ErrorMsg("Error:"))
			fmt.Println("Run 'gollama update' to install it")
			os.Exit(1)
		}
	},
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.Load()
		if err != nil {
			fmt.Printf("%s Failed to load config: %v\n", ui.ErrorMsg("Error:"), err)
			os.Exit(1)
		}

		modelRef := args[0]
		user, repo, quant, err := parseModelRef(modelRef)
		if err != nil {
			fmt.Printf("%s %s\n", ui.ErrorMsg("Error:"), err)
			os.Exit(1)
		}

		modelPath := hf.GetModelFilePath(user, repo, quant)
		if quant == "" {
			modelDir := hf.GetModelPath(user, repo)
			if _, err := os.Stat(modelDir); os.IsNotExist(err) {
				fmt.Printf("%s Model not found: %s\n", ui.ErrorMsg("Error:"), modelRef)
				fmt.Println("Use 'gollama pull' to download models")
				os.Exit(1)
			}

			modelPath = findModelInDir(modelDir)
			if modelPath == "" {
				fmt.Printf("%s No GGUF files found in %s\n", ui.ErrorMsg("Error:"), modelRef)
				os.Exit(1)
			}

			fmt.Printf("%s No quantization specified, using: %s\n",
				ui.Warning("Warning:"), extractQuantFromPath(modelPath))
		} else {
			if _, err := os.Stat(modelPath); os.IsNotExist(err) {
				fmt.Printf("%s Model not found: %s\n", ui.ErrorMsg("Error:"), modelRef)
				fmt.Println("Use 'gollama pull' to download models")
				os.Exit(1)
			}
		}

		manager := server.NewManager(cfg)

		state, err := manager.Status()
		if err != nil {
			fmt.Printf("%s Failed to check server status: %v\n", ui.ErrorMsg("Error:"), err)
		}

		needRestart := false
		if state == nil {
			needRestart = true
		} else if state.Model != modelRef {
			needRestart = true
		}

		if needRestart {
			if state != nil && server.IsRunning(state) {
				fmt.Printf("Stopping server (model: %s)...\n", state.Model)
				if err := manager.Stop(); err != nil {
					fmt.Printf("%s Failed to stop server: %v\n", ui.ErrorMsg("Error:"), err)
					os.Exit(1)
				}
			}

			fmt.Printf("Starting server with model: %s\n", modelRef)
			if err := manager.Start(modelPath, modelRef); err != nil {
				fmt.Printf("%s Failed to start server: %v\n", ui.ErrorMsg("Error:"), err)
				os.Exit(1)
			}
		}

		api := server.NewAPIClient(cfg.Server.Host, cfg.Server.Port)

		if err := api.Health(); err != nil {
			fmt.Printf("%s Server health check failed: %v\n", ui.ErrorMsg("Error:"), err)
			os.Exit(1)
		}

		promptArg := ""
		if len(args) > 1 {
			promptArg = strings.Join(args[1:], " ")
		}

		if promptArg != "" {
			runOneShot(api, modelRef, promptArg, cfg, true)
		} else {
			runTUI(api, modelRef, cfg)
		}
	},
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

func findModelInDir(dir string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".gguf") {
			return dir + "/" + entry.Name()
		}
	}

	return ""
}

func extractQuantFromPath(path string) string {
	parts := strings.Split(path, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return path
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
