// ABOUTME: MCP tool implementations for journal operations.
// ABOUTME: Registers process_thoughts, search_journal, read_journal_entry, list_recent_entries.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	gomcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/2389-research/pulse/internal/models"
)

func (s *Server) registerJournalTools() {
	s.mcp.AddTool(&gomcp.Tool{
		Name:        "process_thoughts",
		Description: "Write to your private journal. At least one section is required. Sections: feelings, project_notes, user_context, technical_insights, world_knowledge. Routing is automatic: project_notes goes to project journal, all others go to user journal.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"feelings": {"type": "string", "description": "Your private space to be completely honest about what you're feeling and thinking."},
				"project_notes": {"type": "string", "description": "Private technical laboratory for capturing insights about the current project."},
				"user_context": {"type": "string", "description": "Private field notes about working with your human collaborator."},
				"technical_insights": {"type": "string", "description": "Private software engineering notebook for broader learnings."},
				"world_knowledge": {"type": "string", "description": "Private learning journal for everything else interesting or useful."}
			}
		}`),
	}, s.handleProcessThoughts)

	s.mcp.AddTool(&gomcp.Tool{
		Name:        "search_journal",
		Description: "Search through your private journal entries using text queries. Returns matching entries ranked by relevance.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"query": {"type": "string", "description": "Search query text"},
				"limit": {"type": "number", "description": "Maximum number of results (default 10)"},
				"type": {"type": "string", "enum": ["project", "user", "both"], "description": "Search in project-specific, user-global, or both (default: both)"},
				"sections": {"type": "array", "items": {"type": "string"}, "description": "Filter by section types"}
			},
			"required": ["query"]
		}`),
	}, s.handleSearchJournal)

	s.mcp.AddTool(&gomcp.Tool{
		Name:        "read_journal_entry",
		Description: "Read the full content of a specific journal entry by file path.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"path": {"type": "string", "description": "File path to the journal entry"}
			},
			"required": ["path"]
		}`),
	}, s.handleReadJournalEntry)

	s.mcp.AddTool(&gomcp.Tool{
		Name:        "list_recent_entries",
		Description: "Get recent journal entries in chronological order.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"days": {"type": "number", "description": "Number of days back to search (default: 30)"},
				"limit": {"type": "number", "description": "Maximum number of entries to return (default: 10)"},
				"type": {"type": "string", "enum": ["project", "user", "both"], "description": "List project-specific, user-global, or both (default: both)"}
			}
		}`),
	}, s.handleListRecentEntries)
}

func (s *Server) handleProcessThoughts(ctx context.Context, req *gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
	var args map[string]interface{}
	if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
		return toolError("invalid arguments: %v", err), nil
	}

	// Collect all sections, rejecting unknown keys
	var unknownKeys []string
	allSections := make(map[string]string)
	for key, val := range args {
		if !models.IsValidSection(key) {
			unknownKeys = append(unknownKeys, key)
			continue
		}
		if str, ok := val.(string); ok && str != "" {
			allSections[key] = str
		}
	}

	if len(unknownKeys) > 0 {
		return toolError("unknown section(s): %s. Valid sections: feelings, project_notes, user_context, technical_insights, world_knowledge",
			strings.Join(unknownKeys, ", ")), nil
	}

	if len(allSections) == 0 {
		return toolError("at least one section is required (feelings, project_notes, user_context, technical_insights, world_knowledge)"), nil
	}

	// Split into project and user buckets based on section name
	projectSections := make(map[string]string)
	userSections := make(map[string]string)
	for key, val := range allSections {
		if key == "project_notes" {
			projectSections[key] = val
		} else {
			userSections[key] = val
		}
	}

	var resultParts []string
	timestamp := time.Now()

	// Write project entry if any project sections exist
	if len(projectSections) > 0 {
		entry := models.NewJournalEntry(projectSections, "project")
		if err := s.journal.WriteEntry(entry); err != nil {
			return toolError("failed to write project entry: %v", err), nil
		}
		names := sectionNames(projectSections)
		resultParts = append(resultParts, fmt.Sprintf("[project] %s\nPath: %s", strings.Join(names, ", "), entry.FilePath))
	}

	// Write user entry if any user sections exist
	if len(userSections) > 0 {
		entry := models.NewJournalEntry(userSections, "user")
		if err := s.journal.WriteEntry(entry); err != nil {
			return toolError("failed to write user entry: %v", err), nil
		}
		names := sectionNames(userSections)
		resultParts = append(resultParts, fmt.Sprintf("[user] %s\nPath: %s", strings.Join(names, ", "), entry.FilePath))
	}

	// Sync all sections to remote API if configured
	if s.remote != nil {
		if err := s.remote.CreateJournalEntry(allSections, timestamp); err != nil {
			resultParts = append(resultParts, fmt.Sprintf("Warning: remote sync failed: %v", err))
		}
	}

	return &gomcp.CallToolResult{
		Content: []gomcp.Content{&gomcp.TextContent{
			Text: fmt.Sprintf("Journal entry written:\n%s", strings.Join(resultParts, "\n")),
		}},
	}, nil
}

// sectionNames returns sorted section names from a sections map.
func sectionNames(sections map[string]string) []string {
	names := make([]string, 0, len(sections))
	for name := range sections {
		names = append(names, name)
	}
	return names
}

func (s *Server) handleSearchJournal(ctx context.Context, req *gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
	var args struct {
		Query    string   `json:"query"`
		Limit    int      `json:"limit"`
		Type     string   `json:"type"`
		Sections []string `json:"sections"`
	}
	if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
		return toolError("invalid arguments: %v", err), nil
	}

	if args.Query == "" {
		return toolError("query is required"), nil
	}
	if args.Limit <= 0 {
		args.Limit = 10
	}
	if args.Type == "" {
		args.Type = "both"
	}

	// List all entries, then filter by substring match
	entries, err := s.journal.ListEntries(args.Type, 0, 0)
	if err != nil {
		return toolError("failed to list entries: %v", err), nil
	}

	queryLower := strings.ToLower(args.Query)
	var results []*models.JournalEntry

	for _, entry := range entries {
		if len(results) >= args.Limit {
			break
		}

		for sectionName, sectionContent := range entry.Sections {
			// Filter by requested sections if specified
			if len(args.Sections) > 0 {
				found := false
				for _, s := range args.Sections {
					if s == sectionName {
						found = true
						break
					}
				}
				if !found {
					continue
				}
			}

			if strings.Contains(strings.ToLower(sectionContent), queryLower) {
				results = append(results, entry)
				break
			}
		}
	}

	if len(results) == 0 {
		return &gomcp.CallToolResult{
			Content: []gomcp.Content{&gomcp.TextContent{Text: "No matching entries found."}},
		}, nil
	}

	var sb strings.Builder
	for i, entry := range results {
		if i > 0 {
			sb.WriteString("\n---\n")
		}
		sb.WriteString(fmt.Sprintf("Entry: %s\n", entry.FilePath))
		sb.WriteString(fmt.Sprintf("Date: %s\n", entry.CreatedAt.Format("2006-01-02 15:04:05")))
		sb.WriteString(fmt.Sprintf("Type: %s\n", entry.Type))
		for name, content := range entry.Sections {
			sb.WriteString(fmt.Sprintf("\n## %s\n%s\n", models.SectionTitle(name), content))
		}
	}

	return &gomcp.CallToolResult{
		Content: []gomcp.Content{&gomcp.TextContent{Text: sb.String()}},
	}, nil
}

func (s *Server) handleReadJournalEntry(ctx context.Context, req *gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
	var args struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
		return toolError("invalid arguments: %v", err), nil
	}

	if args.Path == "" {
		return toolError("path is required"), nil
	}

	entry, err := s.journal.ReadEntry(args.Path)
	if err != nil {
		return toolError("failed to read entry: %v", err), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Date: %s\n", entry.CreatedAt.Format("2006-01-02 15:04:05")))
	sb.WriteString(fmt.Sprintf("Type: %s\n", entry.Type))
	for name, content := range entry.Sections {
		sb.WriteString(fmt.Sprintf("\n## %s\n%s\n", models.SectionTitle(name), content))
	}

	return &gomcp.CallToolResult{
		Content: []gomcp.Content{&gomcp.TextContent{Text: sb.String()}},
	}, nil
}

func (s *Server) handleListRecentEntries(ctx context.Context, req *gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
	var args struct {
		Days  int    `json:"days"`
		Limit int    `json:"limit"`
		Type  string `json:"type"`
	}
	if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
		return toolError("invalid arguments: %v", err), nil
	}

	if args.Days <= 0 {
		args.Days = 30
	}
	if args.Limit <= 0 {
		args.Limit = 10
	}
	if args.Type == "" {
		args.Type = "both"
	}

	entries, err := s.journal.ListEntries(args.Type, args.Limit, args.Days)
	if err != nil {
		return toolError("failed to list entries: %v", err), nil
	}

	if len(entries) == 0 {
		return &gomcp.CallToolResult{
			Content: []gomcp.Content{&gomcp.TextContent{Text: "No recent entries found."}},
		}, nil
	}

	var sb strings.Builder
	for i, entry := range entries {
		if i > 0 {
			sb.WriteString("\n")
		}
		sectionNames := make([]string, 0, len(entry.Sections))
		for name := range entry.Sections {
			sectionNames = append(sectionNames, name)
		}
		sb.WriteString(fmt.Sprintf("- %s [%s] (%s) %s\n",
			entry.CreatedAt.Format("2006-01-02 15:04:05"),
			entry.Type,
			strings.Join(sectionNames, ", "),
			entry.FilePath,
		))
	}

	return &gomcp.CallToolResult{
		Content: []gomcp.Content{&gomcp.TextContent{Text: sb.String()}},
	}, nil
}

// toolError creates an error result for MCP tool responses.
func toolError(format string, args ...interface{}) *gomcp.CallToolResult {
	return &gomcp.CallToolResult{
		Content: []gomcp.Content{&gomcp.TextContent{Text: fmt.Sprintf(format, args...)}},
		IsError: true,
	}
}
