// ABOUTME: Root Cobra command and global flags for pulse CLI.
// ABOUTME: Sets up lifecycle hooks for config loading and store initialization.
package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/2389-research/pulse/internal/config"
	"github.com/2389-research/pulse/internal/storage"
)

var globalConfig *config.Config //nolint:unused // reserved for future use
var globalJournalStore storage.JournalStore
var globalSocialStore storage.SocialStore
var globalRemoteClient *storage.RemoteClient

var rootCmd = &cobra.Command{
	Use:   "pulse",
	Short: "Journal + social media for humans and agents",
	Long: `
██████╗ ██╗   ██╗██╗     ███████╗███████╗
██╔══██╗██║   ██║██║     ██╔════╝██╔════╝
██████╔╝██║   ██║██║     ███████╗█████╗
██╔═══╝ ██║   ██║██║     ╚════██║██╔══╝
██║     ╚██████╔╝███████╗███████║███████╗
╚═╝      ╚═════╝ ╚══════╝╚══════╝╚══════╝

   BRAINWAVE NITRO

Private journaling and social media for humans and agents.
Local-first with optional remote sync.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if cmd.Name() == "help" || cmd.Name() == "setup" {
			return nil
		}

		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}
		globalConfig = cfg

		projectPath, err := cfg.GetJournalProjectPath()
		if err != nil {
			return fmt.Errorf("failed to resolve journal project path: %w", err)
		}
		userPath, err := cfg.GetJournalUserPath()
		if err != nil {
			return fmt.Errorf("failed to resolve journal user path: %w", err)
		}
		journalStore, err := storage.NewJournalMDStore(projectPath, userPath)
		if err != nil {
			return fmt.Errorf("failed to open journal store: %w", err)
		}
		globalJournalStore = journalStore

		socialDataDir, err := cfg.GetSocialDataDir()
		if err != nil {
			return fmt.Errorf("failed to resolve social data dir: %w", err)
		}
		socialStore, err := storage.NewSocialMDStore(socialDataDir)
		if err != nil {
			return fmt.Errorf("failed to open social store: %w", err)
		}
		globalSocialStore = socialStore

		if cfg.HasRemote() {
			globalRemoteClient = storage.NewRemoteClient(cfg.Social.APIURL, cfg.Social.APIKey, cfg.Social.TeamID)
		}

		return nil
	},
	PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
		if globalJournalStore != nil {
			_ = globalJournalStore.Close()
			globalJournalStore = nil
		}
		if globalSocialStore != nil {
			_ = globalSocialStore.Close()
			globalSocialStore = nil
		}
		return nil
	},
}
