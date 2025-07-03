package prompt

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"

	"github.com/AlecAivazis/survey/v2"
	"github.com/AlecAivazis/survey/v2/terminal"
)

// InteractiveSelect shows an interactive selection menu using arrow keys
func InteractiveSelect(title string, options []string, defaultIndex int) (int, error) {
	var selected string

	prompt := &survey.Select{
		Message: title,
		Options: options,
		Default: options[defaultIndex],
	}

	err := survey.AskOne(prompt, &selected, survey.WithIcons(func(icons *survey.IconSet) {
		icons.SelectFocus.Text = "▶"
		icons.MarkedOption.Text = "✓"
		icons.UnmarkedOption.Text = " "
	}))

	if err != nil {
		// If user cancels, return the default
		if err == terminal.InterruptErr {
			return defaultIndex, nil
		}
		return -1, err
	}

	// Find the selected index
	for i, opt := range options {
		if opt == selected {
			return i, nil
		}
	}

	return defaultIndex, nil
}

// InteractiveMultiSelect shows a multi-select menu using arrow keys and space to select
func InteractiveMultiSelect(title string, options []string, defaults []int) ([]int, error) {
	defaultOptions := make([]string, len(defaults))
	for i, idx := range defaults {
		if idx < len(options) {
			defaultOptions[i] = options[idx]
		}
	}

	var selected []string
	prompt := &survey.MultiSelect{
		Message: title,
		Options: options,
		Default: defaultOptions,
	}

	err := survey.AskOne(prompt, &selected, survey.WithIcons(func(icons *survey.IconSet) {
		icons.SelectFocus.Text = "▶"
		icons.MarkedOption.Text = "[✓]"
		icons.UnmarkedOption.Text = "[ ]"
	}))

	if err != nil {
		return defaults, err
	}

	// Convert selected strings back to indices
	indices := []int{}
	for _, sel := range selected {
		for i, opt := range options {
			if opt == sel {
				indices = append(indices, i)
				break
			}
		}
	}

	return indices, nil
}

// InteractiveConfirm shows a yes/no confirmation prompt
func InteractiveConfirm(message string, defaultYes bool) bool {
	var result bool
	prompt := &survey.Confirm{
		Message: message,
		Default: defaultYes,
	}

	err := survey.AskOne(prompt, &result)
	if err != nil {
		return defaultYes
	}

	return result
}

// ClearScreen clears the terminal screen
func ClearScreen() {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "cls")
	default:
		cmd = exec.Command("clear")
	}

	cmd.Stdout = os.Stdout
	cmd.Run()
}

// ShowProgress shows a progress spinner
func ShowProgress(message string, work func() error) error {
	// For now, just print the message and do the work
	// In the future, we can add a spinner
	fmt.Printf("%s...\n", message)
	return work()
}
