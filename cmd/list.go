package cmd

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nchapman/llemme/internal/config"
	"github.com/nchapman/llemme/internal/hf"
	"github.com/nchapman/llemme/internal/ui"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List downloaded models",
	Run: func(cmd *cobra.Command, args []string) {
		modelsDir := config.ModelsPath()

		var models []ModelInfo
		var totalSize int64

		err := filepath.WalkDir(modelsDir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			if d.IsDir() {
				return nil
			}

			if filepath.Ext(d.Name()) != ".gguf" {
				return nil
			}

			relPath, err := filepath.Rel(modelsDir, path)
			if err != nil {
				return err
			}

			parts := strings.Split(relPath, string(filepath.Separator))
			if len(parts) < 3 {
				return nil
			}

			user := parts[0]
			repo := parts[1]
			quant := strings.TrimSuffix(d.Name(), ".gguf")

			info, err := d.Info()
			if err != nil {
				return err
			}

			lastUsed := hf.GetLastUsed(user, repo, quant)
			if lastUsed.IsZero() {
				lastUsed = info.ModTime() // Fall back to download time
			}

			models = append(models, ModelInfo{
				User:     user,
				Repo:     repo,
				Quant:    quant,
				Size:     info.Size(),
				LastUsed: lastUsed,
			})

			totalSize += info.Size()

			return nil
		})

		if err != nil {
			fmt.Printf("%s Failed to list models: %v\n", ui.ErrorMsg("Error:"), err)
			os.Exit(1)
		}

		if len(models) == 0 {
			fmt.Println(ui.Muted("No models downloaded yet"))
			fmt.Println()
			fmt.Println("Use 'llemme pull <user/repo>' to download a model")
			return
		}

		table := ui.NewTable().
			Indent(0).
			AddColumn("MODEL", 40, ui.AlignLeft).
			AddColumn("QUANT", 10, ui.AlignLeft).
			AddColumn("SIZE", 10, ui.AlignRight).
			AddColumn("LAST USED", 12, ui.AlignLeft)

		for _, m := range models {
			modelRef := fmt.Sprintf("%s/%s", m.User, m.Repo)
			table.AddRow(modelRef, m.Quant, ui.FormatBytes(m.Size), formatTime(m.LastUsed))
		}

		fmt.Print(table.Render())
		fmt.Println()
		fmt.Printf("%d models, %s total\n", len(models), ui.FormatBytes(totalSize))
	},
}

type ModelInfo struct {
	User     string
	Repo     string
	Quant    string
	Size     int64
	LastUsed time.Time
}

func formatTime(t time.Time) string {
	now := time.Now()
	diff := now.Sub(t)

	switch {
	case diff < time.Hour:
		return "Just now"
	case diff < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(diff.Hours()))
	case diff < 30*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(diff.Hours()/24))
	default:
		return t.Format("Jan 2006")
	}
}

func init() {
	rootCmd.AddCommand(listCmd)
}
