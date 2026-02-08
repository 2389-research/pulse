// ABOUTME: CLI commands for journal operations.
// ABOUTME: Provides write, search, list, and read subcommands for the journal.
package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/2389-research/pulse/internal/models"
)

var journalCmd = &cobra.Command{
	Use:   "journal",
	Short: "Manage journal entries",
	Long:  "Write, search, list, and read private journal entries.",
}

var journalWriteCmd = &cobra.Command{
	Use:   "write",
	Short: "Write a journal entry",
	Long:  "Create a journal entry with one or more sections.",
	RunE:  runJournalWrite,
}

var journalSearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search journal entries",
	Long:  "Search journal entries by substring matching.",
	Args:  cobra.ExactArgs(1),
	RunE:  runJournalSearch,
}

var journalListCmd = &cobra.Command{
	Use:   "list",
	Short: "List recent journal entries",
	Long:  "List journal entries sorted by date.",
	RunE:  runJournalList,
}

var journalReadCmd = &cobra.Command{
	Use:   "read <path>",
	Short: "Read a journal entry",
	Long:  "Read a specific journal entry by file path.",
	Args:  cobra.ExactArgs(1),
	RunE:  runJournalRead,
}

// Flags
var (
	feelings          string
	projectNotes      string
	userContext       string
	technicalInsights string
	worldKnowledge    string
	journalLimit      int
	journalDays       int
	journalType       string
)

func init() {
	rootCmd.AddCommand(journalCmd)
	journalCmd.AddCommand(journalWriteCmd)
	journalCmd.AddCommand(journalSearchCmd)
	journalCmd.AddCommand(journalListCmd)
	journalCmd.AddCommand(journalReadCmd)

	journalWriteCmd.Flags().StringVar(&feelings, "feelings", "", "Feelings section content")
	journalWriteCmd.Flags().StringVar(&projectNotes, "project-notes", "", "Project notes section content")
	journalWriteCmd.Flags().StringVar(&userContext, "user-context", "", "User context section content")
	journalWriteCmd.Flags().StringVar(&technicalInsights, "technical-insights", "", "Technical insights section content")
	journalWriteCmd.Flags().StringVar(&worldKnowledge, "world-knowledge", "", "World knowledge section content")

	journalListCmd.Flags().IntVar(&journalLimit, "limit", 10, "Maximum number of entries to show")
	journalListCmd.Flags().IntVar(&journalDays, "days", 30, "Number of days back to search")
	journalListCmd.Flags().StringVar(&journalType, "type", "both", "Entry type: project, user, or both")

	journalSearchCmd.Flags().IntVar(&journalLimit, "limit", 10, "Maximum number of results")
	journalSearchCmd.Flags().StringVar(&journalType, "type", "both", "Entry type: project, user, or both")
}

func runJournalWrite(cmd *cobra.Command, args []string) error {
	sections := make(map[string]string)
	if feelings != "" {
		sections["feelings"] = feelings
	}
	if projectNotes != "" {
		sections["project_notes"] = projectNotes
	}
	if userContext != "" {
		sections["user_context"] = userContext
	}
	if technicalInsights != "" {
		sections["technical_insights"] = technicalInsights
	}
	if worldKnowledge != "" {
		sections["world_knowledge"] = worldKnowledge
	}

	if len(sections) == 0 {
		return fmt.Errorf("at least one section is required (--feelings, --project-notes, --user-context, --technical-insights, --world-knowledge)")
	}

	entry := models.NewJournalEntry(sections, "user")
	if err := globalJournalStore.WriteEntry(entry); err != nil {
		return fmt.Errorf("failed to write entry: %w", err)
	}

	sectionNames := make([]string, 0, len(sections))
	for name := range sections {
		sectionNames = append(sectionNames, name)
	}
	fmt.Printf("Journal entry written: %s\n", entry.FilePath)
	fmt.Printf("Sections: %s\n", strings.Join(sectionNames, ", "))
	return nil
}

func runJournalSearch(cmd *cobra.Command, args []string) error {
	query := args[0]

	entries, err := globalJournalStore.ListEntries(journalType, 0, 0)
	if err != nil {
		return fmt.Errorf("failed to list entries: %w", err)
	}

	// Include remote entries in search if configured
	if globalRemoteClient != nil {
		remoteEntries, err := globalRemoteClient.ReadJournalEntries(0)
		if err != nil {
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to fetch remote entries: %v\n", err)
		} else {
			entries = append(entries, remoteEntries...)
			sort.Slice(entries, func(i, j int) bool {
				return entries[i].CreatedAt.After(entries[j].CreatedAt)
			})
		}
	}

	queryLower := strings.ToLower(query)
	count := 0

	for _, entry := range entries {
		if count >= journalLimit {
			break
		}

		matched := false
		for _, content := range entry.Sections {
			if strings.Contains(strings.ToLower(content), queryLower) {
				matched = true
				break
			}
		}

		if matched {
			count++
			fmt.Printf("--- %s [%s] %s\n", entry.CreatedAt.Format("2006-01-02 15:04:05"), entry.Type, entry.FilePath)
			for name, content := range entry.Sections {
				fmt.Printf("  ## %s\n  %s\n", name, truncate(content, 100))
			}
			fmt.Println()
		}
	}

	if count == 0 {
		fmt.Println("No matching entries found.")
	}
	return nil
}

func runJournalList(cmd *cobra.Command, args []string) error {
	entries, err := globalJournalStore.ListEntries(journalType, journalLimit, journalDays)
	if err != nil {
		return fmt.Errorf("failed to list entries: %w", err)
	}

	// Merge remote entries if configured
	if globalRemoteClient != nil {
		remoteEntries, err := globalRemoteClient.ReadJournalEntries(journalLimit)
		if err != nil {
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to fetch remote entries: %v\n", err)
		} else {
			entries = append(entries, remoteEntries...)
			sort.Slice(entries, func(i, j int) bool {
				return entries[i].CreatedAt.After(entries[j].CreatedAt)
			})
			if journalLimit > 0 && len(entries) > journalLimit {
				entries = entries[:journalLimit]
			}
		}
	}

	if len(entries) == 0 {
		fmt.Println("No entries found.")
		return nil
	}

	for _, entry := range entries {
		sectionNames := make([]string, 0, len(entry.Sections))
		for name := range entry.Sections {
			sectionNames = append(sectionNames, name)
		}
		fmt.Printf("%s [%s] (%s) %s\n",
			entry.CreatedAt.Format("2006-01-02 15:04:05"),
			entry.Type,
			strings.Join(sectionNames, ", "),
			entry.FilePath,
		)
	}
	return nil
}

func runJournalRead(cmd *cobra.Command, args []string) error {
	path := args[0]

	entry, err := globalJournalStore.ReadEntry(path)
	if err != nil {
		return fmt.Errorf("failed to read entry: %w", err)
	}

	fmt.Printf("Date: %s\n", entry.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("Type: %s\n", entry.Type)
	fmt.Println()

	for _, name := range models.ValidSections {
		content, ok := entry.Sections[name]
		if !ok || content == "" {
			continue
		}
		fmt.Printf("## %s\n%s\n\n", name, content)
	}
	return nil
}

// truncate shortens a string to maxLen runes, adding "..." if truncated.
func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}
