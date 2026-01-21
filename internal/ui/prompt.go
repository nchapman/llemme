package ui

import (
	"fmt"
	"strings"
)

// PromptYesNo asks a yes/no question and returns true if user confirms.
// defaultYes controls the default when user presses enter.
// On error (e.g., EOF or closed stdin), returns false as a safe default.
func PromptYesNo(prompt string, defaultYes bool) bool {
	if defaultYes {
		fmt.Printf("%s [Y/n] ", prompt)
	} else {
		fmt.Printf("%s [y/N] ", prompt)
	}

	var response string
	if _, err := fmt.Scanln(&response); err != nil {
		return false
	}
	response = strings.TrimSpace(strings.ToLower(response))

	if defaultYes {
		return response == "" || response == "y" || response == "yes"
	}
	return response == "y" || response == "yes"
}
