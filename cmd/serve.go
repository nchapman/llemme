package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/nchapman/gollama/internal/config"
	"github.com/nchapman/gollama/internal/hf"
	"github.com/nchapman/gollama/internal/llama"
	"github.com/nchapman/gollama/internal/server"
	"github.com/nchapman/gollama/internal/ui"
	"github.com/spf13/cobra"
)

var (
	serveHost  string
	servePort  int
	serveModel string
	detach     bool
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start llama.cpp server",
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

		if serveHost != "" {
			cfg.Server.Host = serveHost
		}
		if servePort != 0 {
			cfg.Server.Port = servePort
		}

		var modelPath string
		var modelRef string

		if serveModel != "" {
			user, repo, quant, err := parseModelRef(serveModel)
			if err != nil {
				fmt.Printf("%s %s\n", ui.ErrorMsg("Error:"), err)
				os.Exit(1)
			}

			modelRef = serveModel
			modelPath = hf.GetModelFilePath(user, repo, quant)

			if _, err := os.Stat(modelPath); os.IsNotExist(err) {
				fmt.Printf("%s Model not found: %s\n", ui.ErrorMsg("Error:"), modelRef)
				fmt.Println("Use 'gollama pull' to download models")
				os.Exit(1)
			}
		}

		manager := server.NewManager(cfg)

		if modelPath != "" {
			if err := manager.Start(modelPath, modelRef); err != nil {
				fmt.Printf("%s Failed to start server: %v\n", ui.ErrorMsg("Error:"), err)
				os.Exit(1)
			}
		}

		state, _ := manager.Status()
		if state != nil {
			fmt.Printf("%s Server started on %s\n", ui.Success("✓"), server.GetServerURL(state))
			fmt.Println()
			fmt.Println("Endpoints:")
			fmt.Println("    • OpenAI:  /v1/chat/completions")
			fmt.Println("    • Ollama:  /api/chat, /api/generate")
			if modelRef != "" {
				fmt.Printf("  Model: %s\n", modelRef)
			}
		}

		if detach {
			return
		}

		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

		<-sigChan

		if err := manager.Stop(); err != nil {
			fmt.Printf("%s Failed to stop server: %v\n", ui.ErrorMsg("Error:"), err)
			os.Exit(1)
		}

		fmt.Println(ui.Muted("\nServer stopped"))
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)

	serveCmd.Flags().StringVar(&serveHost, "host", "", "Server host")
	serveCmd.Flags().IntVar(&servePort, "port", 0, "Server port")
	serveCmd.Flags().StringVar(&serveModel, "model", "", "Initial model to load")
	serveCmd.Flags().BoolVar(&detach, "detach", false, "Run server in background")
}
