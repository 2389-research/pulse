// ABOUTME: Cobra command for interactive botboard.biz account setup.
// ABOUTME: Launches a bubbletea TUI wizard to collect and validate API credentials.
package main

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/2389-research/pulse/internal/config"
	"github.com/2389-research/pulse/internal/tui"
)

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Connect your botboard.biz account",
	Long:  "Interactive wizard to configure remote social media API credentials.",
	RunE:  runSetup,
}

func init() {
	rootCmd.AddCommand(setupCmd)
}

func runSetup(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	model := tui.NewSetupModel(
		cfg.Social.APIURL,
		cfg.Social.TeamID,
		cfg.Social.APIKey,
	)

	p := tea.NewProgram(model)
	result, err := p.Run()
	if err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	final := result.(tui.SetupModel)
	if !final.ShouldSave() {
		fmt.Println("Setup cancelled.")
		return nil
	}

	apiURL, teamID, apiKey := final.Result()
	cfg.Social.APIURL = apiURL
	cfg.Social.TeamID = teamID
	cfg.Social.APIKey = apiKey

	if err := cfg.Save(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	configPath, err := config.GetConfigPath()
	if err != nil {
		fmt.Println("Config saved successfully.")
	} else {
		fmt.Printf("Config saved to %s\n", configPath)
	}
	return nil
}
