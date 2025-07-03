package prompt

import (
	"fmt"
	"strings"

	"github.com/c-bata/go-prompt"
)

// MenuOption represents a menu option
type MenuOption struct {
	Label       string
	Value       string
	Description string
}

// SelectMenu shows an interactive menu and returns the selected value
func SelectMenu(title string, options []MenuOption) string {
	fmt.Println(title)
	
	// Create a map for quick lookup
	optionMap := make(map[string]MenuOption)
	suggestions := []prompt.Suggest{}
	
	for i, opt := range options {
		// Use number keys for selection
		key := fmt.Sprintf("%d", i+1)
		optionMap[key] = opt
		suggestions = append(suggestions, prompt.Suggest{
			Text:        key,
			Description: fmt.Sprintf("%s - %s", opt.Label, opt.Description),
		})
	}
	
	// Show options
	fmt.Println()
	for i, opt := range options {
		fmt.Printf("  %d) %s", i+1, opt.Label)
		if opt.Description != "" {
			fmt.Printf(" - %s", opt.Description)
		}
		fmt.Println()
	}
	
	completer := func(d prompt.Document) []prompt.Suggest {
		return prompt.FilterHasPrefix(suggestions, d.GetWordBeforeCursor(), true)
	}
	
	fmt.Println()
	result := prompt.Input("Select option (1-"+fmt.Sprintf("%d", len(options))+"): ", completer,
		prompt.OptionPrefixTextColor(prompt.Blue),
		prompt.OptionPreviewSuggestionTextColor(prompt.Green),
		prompt.OptionSelectedSuggestionBGColor(prompt.DarkGray),
		prompt.OptionSuggestionBGColor(prompt.DarkBlue),
		prompt.OptionShowCompletionAtStart(),
	)
	
	// Check if it's a valid option
	if opt, ok := optionMap[result]; ok {
		return opt.Value
	}
	
	// Try to match by label (case insensitive)
	resultLower := strings.ToLower(strings.TrimSpace(result))
	for _, opt := range options {
		if strings.ToLower(opt.Label) == resultLower {
			return opt.Value
		}
	}
	
	// Default to first option
	if len(options) > 0 {
		fmt.Printf("Invalid selection. Defaulting to: %s\n", options[0].Label)
		return options[0].Value
	}
	
	return ""
}

// SimpleMenu shows a simple text-based menu without dependencies
func SimpleMenu(title string, options []MenuOption) string {
	fmt.Println(title)
	fmt.Println()
	
	// Show options
	for i, opt := range options {
		fmt.Printf("  %d) %s", i+1, opt.Label)
		if opt.Description != "" {
			fmt.Printf(" - %s", opt.Description)
		}
		fmt.Println()
	}
	
	fmt.Println()
	fmt.Printf("Select option (1-%d): ", len(options))
	
	var response string
	fmt.Scanln(&response)
	response = strings.TrimSpace(response)
	
	// Try to parse as number
	for i, opt := range options {
		if response == fmt.Sprintf("%d", i+1) {
			return opt.Value
		}
	}
	
	// Try to match by first letter
	responseLower := strings.ToLower(response)
	for _, opt := range options {
		if strings.HasPrefix(strings.ToLower(opt.Value), responseLower) {
			return opt.Value
		}
	}
	
	// Default
	if len(options) > 0 {
		fmt.Printf("Invalid selection. Defaulting to: %s\n", options[0].Label)
		return options[0].Value
	}
	
	return ""
}