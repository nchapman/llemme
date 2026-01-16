package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/nchapman/lemme/internal/proxy"
	"github.com/nchapman/lemme/internal/ui"
	"github.com/spf13/cobra"
)

var psCmd = &cobra.Command{
	Use:   "ps",
	Short: "Show proxy status and loaded models",
	Run: func(cmd *cobra.Command, args []string) {
		state := proxy.GetRunningProxyState()
		if state == nil {
			fmt.Println(ui.Muted("Proxy is not running"))
			fmt.Println()
			fmt.Println("Start it with: lemme serve")
			fmt.Println("Or use: lemme run <model> (will auto-start proxy)")
			return
		}

		// Get detailed status from proxy API
		proxyURL := fmt.Sprintf("http://%s:%d", state.Host, state.Port)
		status, err := getProxyStatus(proxyURL)
		if err != nil {
			// Fall back to basic info
			fmt.Printf("%s\n", ui.Bold("Proxy Status"))
			fmt.Println()
			fmt.Printf("  • Running on: %s\n", ui.Bold(proxyURL))
			fmt.Printf("  • PID: %d\n", state.PID)
			fmt.Printf("  • Started: %s\n", formatTimeSince(state.StartedAt))
			fmt.Println()
			fmt.Printf("%s Could not fetch detailed status: %v\n", ui.Muted("Note:"), err)
			return
		}

		// Pretty print status
		fmt.Printf("%s\n", ui.Bold("Proxy Status"))
		fmt.Println()
		fmt.Printf("  • Running on: %s (PID %d)\n", ui.Bold(proxyURL), state.PID)
		fmt.Printf("  • Uptime: %s\n", formatUptime(time.Duration(status.UptimeSeconds)*time.Second))
		fmt.Printf("  • Max models: %d\n", status.MaxModels)
		fmt.Printf("  • Idle timeout: %s\n", status.IdleTimeout)
		fmt.Println()

		if len(status.Models) == 0 {
			fmt.Println(ui.Muted("No models loaded"))
			fmt.Println()
			fmt.Println("Use 'lemme run <model>' to load a model")
			return
		}

		fmt.Printf("%s\n", ui.Bold("Loaded Models"))
		fmt.Println()

		header := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))

		// Calculate idle timeout in minutes for "unload in" display
		idleTimeoutMins := 10.0 // default
		if status.IdleTimeout != "" {
			if d, err := time.ParseDuration(status.IdleTimeout); err == nil {
				idleTimeoutMins = d.Minutes()
			}
		}

		fmt.Printf("  %-50s %5s  %-7s  %s\n",
			header.Render("MODEL"),
			header.Render("PORT"),
			header.Render("STATUS"),
			header.Render("UNLOADS IN"),
		)

		for _, m := range status.Models {
			unloadIn := formatUnloadTime(m.IdleMinutes, idleTimeoutMins)

			fmt.Printf("  %-50s %5d  %-7s  %s\n",
				truncateModel(m.ModelName, 50),
				m.Port,
				m.Status,
				unloadIn,
			)
		}

		fmt.Println()
		fmt.Printf("%s %d models loaded\n", ui.Bold("Total:"), len(status.Models))
	},
}

func getProxyStatus(proxyURL string) (*proxy.ProxyStatus, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(proxyURL + "/api/status")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var status proxy.ProxyStatus
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return nil, err
	}

	return &status, nil
}

func formatUptime(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%d seconds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%d minutes", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		hours := int(d.Hours())
		mins := int(d.Minutes()) % 60
		return fmt.Sprintf("%dh %dm", hours, mins)
	}
	return fmt.Sprintf("%.1f days", d.Hours()/24)
}

func formatUnloadTime(idleMinutes, timeoutMinutes float64) string {
	remaining := timeoutMinutes - idleMinutes
	if remaining <= 0 {
		return "soon"
	}
	if remaining < 1 {
		secs := int(remaining * 60)
		return fmt.Sprintf("%ds", secs)
	}
	if remaining < 60 {
		return fmt.Sprintf("%.0fm", remaining)
	}
	return fmt.Sprintf("%.1fh", remaining/60)
}

func truncateModel(name string, maxLen int) string {
	if len(name) <= maxLen {
		return name
	}
	return name[:maxLen-3] + "..."
}

func formatTimeSince(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	diff := time.Since(t)
	if diff < time.Minute {
		return "just now"
	}
	if diff < time.Hour {
		return fmt.Sprintf("%dm ago", int(diff.Minutes()))
	}
	if diff < 24*time.Hour {
		return fmt.Sprintf("%dh ago", int(diff.Hours()))
	}
	return t.Format("Jan 2 15:04")
}

func init() {
	rootCmd.AddCommand(psCmd)
}
