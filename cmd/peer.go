package cmd

import (
	"fmt"

	"github.com/nchapman/lleme/internal/config"
	"github.com/nchapman/lleme/internal/peer"
	"github.com/nchapman/lleme/internal/ui"
	"github.com/spf13/cobra"
)

var peerCmd = &cobra.Command{
	Use:     "peer",
	Short:   "Peer-to-peer model sharing commands",
	GroupID: "model",
	Long:    `Commands for debugging and managing peer-to-peer model sharing.`,
}

var peerStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show peer sharing status",
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.Load()
		if err != nil {
			ui.Fatal("Failed to load config: %v", err)
		}

		if !cfg.Peer.Enabled {
			fmt.Println("Peer sharing: " + ui.Muted("disabled"))
			fmt.Println()
			fmt.Println(ui.Muted("Enable with: lleme config set peer.enabled true"))
			return
		}

		// Check if server is actually running by trying to connect
		serverRunning := peer.IsServerRunning(cfg.Peer.Port)

		if serverRunning {
			localIP := peer.GetLocalIP()
			fmt.Println("Peer sharing: " + ui.Keyword("active"))
			fmt.Printf("Address:      %s:%d\n", localIP, cfg.Peer.Port)

			idx := peer.NewPeerFileIndex()
			if err := idx.Load(); err == nil {
				fmt.Printf("Indexed:      %d files\n", idx.Count())
			}
		} else {
			fmt.Println("Peer sharing: " + ui.Muted("not running"))
			fmt.Printf("Port:         %d\n", cfg.Peer.Port)
			fmt.Println()
			fmt.Println(ui.Muted("Start with: lleme serve"))
		}
	},
}

var peerListCmd = &cobra.Command{
	Use:   "list",
	Short: "Discover and list peers on the network",
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.Load()
		if err != nil {
			ui.Fatal("Failed to load config: %v", err)
		}

		if !cfg.Peer.Enabled {
			fmt.Println(ui.Muted("Peer discovery is disabled."))
			fmt.Println(ui.Muted("Enable with 'peer.enabled: true' in ~/.lleme/config.yaml"))
			return
		}

		// Discover all peers with spinner (thorough mode for complete list)
		spinner := ui.NewSpinner()
		spinner.Start("Discovering peers...")

		peers := peer.DiscoverPeersThoroughSilent()

		if len(peers) == 0 {
			spinner.Stop(true, "")
			fmt.Println(ui.Muted("No peers found on the network."))
			return
		}

		spinner.Stop(true, "")

		table := ui.NewTable().
			AddColumn("HOST", 0, ui.AlignLeft).
			AddColumn("PORT", 5, ui.AlignRight).
			AddColumn("VERSION", 0, ui.AlignLeft)
		for _, p := range peers {
			table.AddRow(p.Host, fmt.Sprintf("%d", p.Port), p.Version)
		}
		fmt.Print(table.Render())

		fmt.Printf("\nFound %d peer(s)\n", len(peers))
	},
}

var peerIndexCmd = &cobra.Command{
	Use:   "index",
	Short: "Show or rebuild the peer file index",
	Long: `Show files in the peer file index that are available for sharing.

Use --rebuild to rebuild the index from downloaded model manifests.`,
	Run: func(cmd *cobra.Command, args []string) {
		rebuild, _ := cmd.Flags().GetBool("rebuild")

		if rebuild {
			fmt.Println("Rebuilding peer file index...")
			if err := peer.RebuildPeerFileIndex(); err != nil {
				ui.Fatal("Failed to rebuild index: %v", err)
			}
			fmt.Println("Index rebuilt successfully.")
			fmt.Println()
		}

		idx := peer.NewPeerFileIndex()
		if err := idx.Load(); err != nil {
			ui.Fatal("Failed to load index: %v", err)
		}

		entries := idx.Entries()
		if len(entries) == 0 {
			fmt.Println(ui.Muted("Peer file index is empty. Pull some models first."))
			fmt.Println(ui.Muted("Run 'lleme peer index --rebuild' if you have models."))
			return
		}

		table := ui.NewTable().
			AddColumn("HASH", 19, ui.AlignLeft).
			AddColumn("FILE", 0, ui.AlignLeft)
		for hash, path := range entries {
			// Truncate hash for display
			shortHash := hash
			if len(hash) > 16 {
				shortHash = hash[:16] + "..."
			}
			table.AddRow(shortHash, path)
		}
		fmt.Print(table.Render())

		fmt.Printf("\n%d file(s) indexed\n", len(entries))
	},
}

func init() {
	peerIndexCmd.Flags().Bool("rebuild", false, "Rebuild the hash index from manifests")

	peerCmd.AddCommand(peerStatusCmd)
	peerCmd.AddCommand(peerListCmd)
	peerCmd.AddCommand(peerIndexCmd)
	rootCmd.AddCommand(peerCmd)
}
