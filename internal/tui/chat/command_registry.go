package chat

import "strings"

// CommandDef defines a slash command for the TUI
type CommandDef struct {
	Name        string   // Primary name (e.g., "/help")
	Aliases     []string // Alternative names (e.g., ["/?"])
	Description string   // Short description for popup
}

// Commands is the list of available slash commands
var Commands = []CommandDef{
	{Name: "/help", Aliases: []string{"/?"}, Description: "Show help"},
	{Name: "/clear", Description: "Clear conversation"},
	{Name: "/system", Description: "Show/set system prompt"},
	{Name: "/set", Description: "Change a setting"},
	{Name: "/show", Description: "Show current settings"},
	{Name: "/reload", Description: "Reload model"},
	{Name: "/bye", Aliases: []string{"/exit", "/quit"}, Description: "Exit chat"},
}

// FilterCommands returns commands where the name or aliases start with the query
func FilterCommands(query string) []CommandDef {
	if query == "" {
		return Commands
	}

	query = strings.ToLower(query)
	var results []CommandDef

	for _, cmd := range Commands {
		if strings.HasPrefix(strings.ToLower(cmd.Name), query) {
			results = append(results, cmd)
			continue
		}
		for _, alias := range cmd.Aliases {
			if strings.HasPrefix(strings.ToLower(alias), query) {
				results = append(results, cmd)
				break
			}
		}
	}
	return results
}
