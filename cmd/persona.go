package cmd

import (
	"fmt"

	"github.com/nchapman/lleme/internal/config"
	"github.com/nchapman/lleme/internal/ui"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	personaModel  string
	personaSystem string
	personaFrom   string
	personaForce  bool
)

var personaCmd = &cobra.Command{
	Use:     "persona",
	Short:   "Manage personas (saved model configurations)",
	GroupID: "persona",
	Long: `Manage personas - saved model configurations with system prompts and options.

A persona is a YAML file that specifies:
  - model: The base model to use
  - system: A system prompt
  - options: llama.cpp options (temp, top-p, etc.)

Examples:
  lleme persona list                    # List all personas
  lleme persona show coding-assistant   # Show persona details
  lleme persona create my-assistant     # Create new persona
  lleme persona edit my-assistant       # Edit in $EDITOR
  lleme persona rm my-assistant         # Delete persona

Run a persona:
  lleme run coding-assistant "help me refactor this"`,
}

var personaListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all personas",
	Run: func(cmd *cobra.Command, args []string) {
		personas, err := config.ListPersonas()
		if err != nil {
			ui.Fatal("%v", err)
		}

		if len(personas) == 0 {
			fmt.Println(ui.Muted("No personas configured yet"))
			fmt.Println()
			fmt.Println("Create one with: lleme persona create <name>")
			return
		}

		fmt.Println(ui.Header("Personas"))
		fmt.Println()

		table := ui.NewTable().
			AddColumn("NAME", 0, ui.AlignLeft).
			AddColumn("MODEL", 0, ui.AlignLeft)

		for _, p := range personas {
			model := ui.Muted("(not set)")
			if p.HasModel {
				model = p.Model
			}
			table.AddRow(p.Name, model)
		}

		fmt.Print(table.Render())
		fmt.Println()
		fmt.Printf("%d persona(s)\n", len(personas))
	},
}

var personaShowCmd = &cobra.Command{
	Use:   "show <name>",
	Short: "Show persona details",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]

		persona, err := config.LoadPersona(name)
		if err != nil {
			ui.Fatal("%v", err)
		}

		fmt.Printf("%s\n\n", ui.Header("Persona: "+name))

		if persona.Model != "" {
			fmt.Printf("%s %s\n", ui.Bold("Model:"), persona.Model)
		} else {
			fmt.Printf("%s %s\n", ui.Bold("Model:"), ui.Muted("(not set - specify at runtime)"))
		}

		if persona.System != "" {
			fmt.Printf("\n%s\n%s\n", ui.Bold("System prompt:"), persona.System)
		}

		if len(persona.Options) > 0 {
			fmt.Printf("\n%s\n", ui.Bold("Options:"))
			data, _ := yaml.Marshal(persona.Options)
			fmt.Print(string(data))
		}

		fmt.Printf("\n%s %s\n", ui.Muted("Path:"), config.PersonaPath(name))
	},
}

var personaCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a new persona",
	Long: `Create a new persona configuration.

Examples:
  lleme persona create my-assistant                           # Create and edit
  lleme persona create coder -m bartowski/Qwen2.5-Coder-GGUF  # With model
  lleme persona create writer --from coder                    # Copy existing`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]

		if err := config.ValidatePersonaName(name); err != nil {
			ui.Fatal("%v", err)
		}

		if config.PersonaExists(name) && !personaForce {
			ui.Fatal("Persona '%s' already exists. Use --force to overwrite.", name)
		}

		var persona *config.Persona

		if personaFrom != "" {
			// Copy from existing persona
			existing, err := config.LoadPersona(personaFrom)
			if err != nil {
				ui.Fatal("%v", err)
			}
			persona = existing
		} else {
			persona = &config.Persona{}
		}

		// Apply flags
		if personaModel != "" {
			persona.Model = personaModel
		}
		if personaSystem != "" {
			persona.System = personaSystem
		}

		if err := config.SavePersonaTemplate(name, persona); err != nil {
			ui.Fatal("%v", err)
		}

		fmt.Printf("%s Created persona '%s'\n", ui.Success("✓"), name)
		fmt.Printf("  %s\n\n", ui.Muted(config.PersonaPath(name)))

		// Open in editor
		openPersonaInEditor(name)
	},
}

var personaEditCmd = &cobra.Command{
	Use:   "edit <name>",
	Short: "Edit a persona in $EDITOR",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]

		if !config.PersonaExists(name) {
			ui.Fatal("Persona '%s' not found", name)
		}

		openPersonaInEditor(name)
	},
}

var personaRmCmd = &cobra.Command{
	Use:   "rm <name>",
	Short: "Remove a persona",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]

		if !config.PersonaExists(name) {
			ui.Fatal("Persona '%s' not found", name)
		}

		if !personaForce {
			if !ui.PromptYesNo(fmt.Sprintf("Remove persona '%s'?", name), false) {
				fmt.Println(ui.Muted("Cancelled"))
				return
			}
		}

		if err := config.DeletePersona(name); err != nil {
			ui.Fatal("%v", err)
		}

		fmt.Printf("Removed persona '%s'\n", name)
	},
}

func openPersonaInEditor(name string) {
	path := config.PersonaPath(name)
	if err := openInEditor(path); err != nil {
		ui.Fatal("%v", err)
	}
}

func init() {
	rootCmd.AddCommand(personaCmd)

	personaCmd.AddCommand(personaListCmd)
	personaCmd.AddCommand(personaShowCmd)
	personaCmd.AddCommand(personaCreateCmd)
	personaCmd.AddCommand(personaEditCmd)
	personaCmd.AddCommand(personaRmCmd)

	personaCreateCmd.Flags().StringVarP(&personaModel, "model", "m", "", "Base model")
	personaCreateCmd.Flags().StringVarP(&personaSystem, "system", "s", "", "System prompt")
	personaCreateCmd.Flags().StringVar(&personaFrom, "from", "", "Copy from existing persona")
	personaCreateCmd.Flags().BoolVarP(&personaForce, "force", "f", false, "Overwrite existing persona")

	personaRmCmd.Flags().BoolVarP(&personaForce, "force", "f", false, "Skip confirmation")
}
