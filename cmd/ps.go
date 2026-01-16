package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/nchapman/gollama/internal/server"
	"github.com/nchapman/gollama/internal/ui"
	"github.com/spf13/cobra"
)

var psCmd = &cobra.Command{
	Use:   "ps",
	Short: "Show server status and loaded models",
	Run: func(cmd *cobra.Command, args []string) {
		manager := server.NewManager(nil)
		state, err := manager.Status()
		if err != nil {
			fmt.Printf("%s Failed to check server status: %v\n", ui.ErrorMsg("Error:"), err)
			os.Exit(1)
		}

		if state == nil {
			fmt.Println(ui.Muted("Server is not running"))
			return
		}

		fmt.Printf("%s\n", ui.Bold("Server Status"))
		fmt.Println()

		startedAt, _ := time.Parse(time.RFC3339, state.StartedAt)
		uptime := time.Since(startedAt)

		fmt.Printf("  • Running on: %s\n", ui.Bold(server.GetServerURL(state)))
		fmt.Printf("  • Model: %s\n", ui.Value(state.Model))
		fmt.Printf("  • Uptime: %s\n", formatUptime(uptime))

		if state.ModelPath != "" {
			if info, err := os.Stat(state.ModelPath); err == nil {
				size := info.Size()
				fmt.Printf("  • Size: %s\n", formatBytes(size))
			}
		}
	},
}

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop llama.cpp server",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		manager := server.NewManager(nil)

		state, err := manager.Status()
		if err != nil {
			fmt.Printf("%s Failed to check server status: %v\n", ui.ErrorMsg("Error:"), err)
			os.Exit(1)
		}

		if state == nil {
			fmt.Println(ui.Muted("Server is not running"))
			return
		}

		if err := manager.Stop(); err != nil {
			fmt.Printf("%s Failed to stop server: %v\n", ui.ErrorMsg("Error:"), err)
			os.Exit(1)
		}

		fmt.Println(ui.Success("✓ Server stopped"))
	},
}

func formatUptime(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%d seconds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%d minutes", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%.1f hours", d.Hours())
	}
	return fmt.Sprintf("%.1f days", d.Hours()/24)
}

func init() {
	rootCmd.AddCommand(psCmd)
	rootCmd.AddCommand(stopCmd)
}
