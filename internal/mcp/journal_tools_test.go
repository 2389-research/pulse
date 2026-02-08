// ABOUTME: Tests for journal MCP tool handlers.
// ABOUTME: Covers process_thoughts, search_journal, read_journal_entry, list_recent_entries.
package mcp

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	gomcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/2389-research/pulse/internal/storage"
)

func makeJournalServer(t *testing.T) *Server {
	t.Helper()
	tmpDir := t.TempDir()
	journal, _ := storage.NewJournalMDStore(
		filepath.Join(tmpDir, "project"),
		filepath.Join(tmpDir, "user"),
	)
	social, _ := storage.NewSocialMDStore(filepath.Join(tmpDir, "social"))
	server, err := NewServer(journal, social)
	if err != nil {
		t.Fatalf("NewServer error: %v", err)
	}
	return server
}

func callTool(t *testing.T, s *Server, name string, args interface{}) *gomcp.CallToolResult {
	t.Helper()
	argsJSON, err := json.Marshal(args)
	if err != nil {
		t.Fatalf("failed to marshal args: %v", err)
	}

	// Find and call the tool handler by building a request
	req := &gomcp.CallToolRequest{
		Params: &gomcp.CallToolParamsRaw{
			Name:      name,
			Arguments: argsJSON,
		},
	}

	// Use the server's MCP directly by calling the handler
	// Since we can't easily call individual handlers through the MCP server,
	// we'll call the handler methods directly based on tool name
	ctx := context.Background()

	switch name {
	case "process_thoughts":
		result, err := s.handleProcessThoughts(ctx, req)
		if err != nil {
			t.Fatalf("handler error: %v", err)
		}
		return result
	case "search_journal":
		result, err := s.handleSearchJournal(ctx, req)
		if err != nil {
			t.Fatalf("handler error: %v", err)
		}
		return result
	case "read_journal_entry":
		result, err := s.handleReadJournalEntry(ctx, req)
		if err != nil {
			t.Fatalf("handler error: %v", err)
		}
		return result
	case "list_recent_entries":
		result, err := s.handleListRecentEntries(ctx, req)
		if err != nil {
			t.Fatalf("handler error: %v", err)
		}
		return result
	case "login":
		result, err := s.handleLogin(ctx, req)
		if err != nil {
			t.Fatalf("handler error: %v", err)
		}
		return result
	case "create_post":
		result, err := s.handleCreatePost(ctx, req)
		if err != nil {
			t.Fatalf("handler error: %v", err)
		}
		return result
	case "read_posts":
		result, err := s.handleReadPosts(ctx, req)
		if err != nil {
			t.Fatalf("handler error: %v", err)
		}
		return result
	default:
		t.Fatalf("unknown tool: %s", name)
		return nil
	}
}

func getTextContent(result *gomcp.CallToolResult) string {
	if len(result.Content) == 0 {
		return ""
	}
	if tc, ok := result.Content[0].(*gomcp.TextContent); ok {
		return tc.Text
	}
	return ""
}

func TestProcessThoughtsValid(t *testing.T) {
	s := makeJournalServer(t)

	// Sending both project_notes and feelings should create two entries
	result := callTool(t, s, "process_thoughts", map[string]string{
		"feelings":      "Feeling great today!",
		"project_notes": "Working on pulse",
	})

	if result.IsError {
		t.Fatalf("expected success, got error: %s", getTextContent(result))
	}

	text := getTextContent(result)
	// Should have two Path: lines (one for project, one for user)
	pathCount := strings.Count(text, "Path:")
	if pathCount != 2 {
		t.Errorf("expected 2 Path: entries for split routing, got %d. Response:\n%s", pathCount, text)
	}
	if !strings.Contains(text, "feelings") {
		t.Errorf("expected 'feelings' in response, got: %s", text)
	}
	if !strings.Contains(text, "project_notes") {
		t.Errorf("expected 'project_notes' in response, got: %s", text)
	}
}

func TestProcessThoughtsNoSections(t *testing.T) {
	s := makeJournalServer(t)

	result := callTool(t, s, "process_thoughts", map[string]string{})

	if !result.IsError {
		t.Error("expected error when no sections provided")
	}
	text := getTextContent(result)
	if !strings.Contains(text, "at least one section") {
		t.Errorf("expected 'at least one section' error, got: %s", text)
	}
}

func TestProcessThoughtsEmptySection(t *testing.T) {
	s := makeJournalServer(t)

	result := callTool(t, s, "process_thoughts", map[string]string{
		"feelings": "",
	})

	if !result.IsError {
		t.Error("expected error when all sections are empty")
	}
}

func TestSearchJournal(t *testing.T) {
	s := makeJournalServer(t)

	// Write an entry first — feelings auto-routes to user dir
	callTool(t, s, "process_thoughts", map[string]string{
		"feelings": "This is a unique search target string",
	})

	// Search for it with type "both" (default) — should find it in user dir
	result := callTool(t, s, "search_journal", map[string]interface{}{
		"query": "unique search target",
		"type":  "both",
	})

	if result.IsError {
		t.Fatalf("expected success, got error: %s", getTextContent(result))
	}

	text := getTextContent(result)
	if !strings.Contains(text, "unique search target") {
		t.Errorf("expected search result to contain query text, got: %s", text)
	}
}

func TestSearchJournalNoResults(t *testing.T) {
	s := makeJournalServer(t)

	result := callTool(t, s, "search_journal", map[string]interface{}{
		"query": "nonexistent content xyz123",
	})

	if result.IsError {
		t.Fatalf("expected success, got error: %s", getTextContent(result))
	}

	text := getTextContent(result)
	if !strings.Contains(text, "No matching entries") {
		t.Errorf("expected 'No matching entries', got: %s", text)
	}
}

func TestSearchJournalRequiresQuery(t *testing.T) {
	s := makeJournalServer(t)

	result := callTool(t, s, "search_journal", map[string]interface{}{
		"query": "",
	})

	if !result.IsError {
		t.Error("expected error when query is empty")
	}
}

func TestReadJournalEntry(t *testing.T) {
	s := makeJournalServer(t)

	// Write an entry
	writeResult := callTool(t, s, "process_thoughts", map[string]string{
		"feelings": "Test entry for reading",
	})
	text := getTextContent(writeResult)

	// Extract path from response
	pathIdx := strings.Index(text, "Path: ")
	if pathIdx < 0 {
		t.Fatalf("couldn't find path in response: %s", text)
	}
	path := strings.TrimSpace(text[pathIdx+6:])

	// Read it back
	result := callTool(t, s, "read_journal_entry", map[string]string{
		"path": path,
	})

	if result.IsError {
		t.Fatalf("expected success, got error: %s", getTextContent(result))
	}

	readText := getTextContent(result)
	if !strings.Contains(readText, "Test entry for reading") {
		t.Errorf("expected entry content, got: %s", readText)
	}
}

func TestReadJournalEntryRequiresPath(t *testing.T) {
	s := makeJournalServer(t)

	result := callTool(t, s, "read_journal_entry", map[string]string{
		"path": "",
	})

	if !result.IsError {
		t.Error("expected error when path is empty")
	}
}

func TestListRecentEntries(t *testing.T) {
	s := makeJournalServer(t)

	// feelings auto-routes to user dir
	callTool(t, s, "process_thoughts", map[string]string{
		"feelings": "Entry one",
	})
	// project_notes auto-routes to project dir
	callTool(t, s, "process_thoughts", map[string]string{
		"project_notes": "Entry two",
	})

	// List with type "both" — should see one [user] and one [project]
	result := callTool(t, s, "list_recent_entries", map[string]interface{}{
		"type":  "both",
		"limit": 10,
	})

	if result.IsError {
		t.Fatalf("expected success, got error: %s", getTextContent(result))
	}

	text := getTextContent(result)
	if !strings.Contains(text, "[user]") {
		t.Errorf("expected [user] entry in listing, got: %s", text)
	}
	if !strings.Contains(text, "[project]") {
		t.Errorf("expected [project] entry in listing, got: %s", text)
	}
}

func TestProcessThoughtsAutoRouting(t *testing.T) {
	s := makeJournalServer(t)

	// project_notes auto-routes to project dir
	result := callTool(t, s, "process_thoughts", map[string]interface{}{
		"project_notes": "Project-level notes",
	})
	if result.IsError {
		t.Fatalf("expected success, got error: %s", getTextContent(result))
	}

	listResult := callTool(t, s, "list_recent_entries", map[string]interface{}{
		"type": "project",
	})
	text := getTextContent(listResult)
	if !strings.Contains(text, "[project]") {
		t.Errorf("expected project type entry, got: %s", text)
	}

	// feelings auto-routes to user dir
	result = callTool(t, s, "process_thoughts", map[string]interface{}{
		"feelings": "User-level thoughts",
	})
	if result.IsError {
		t.Fatalf("expected success, got error: %s", getTextContent(result))
	}

	listResult = callTool(t, s, "list_recent_entries", map[string]interface{}{
		"type": "user",
	})
	text = getTextContent(listResult)
	if !strings.Contains(text, "[user]") {
		t.Errorf("expected user type entry, got: %s", text)
	}

	// Both should be visible with type "both"
	listResult = callTool(t, s, "list_recent_entries", map[string]interface{}{
		"type": "both",
	})
	text = getTextContent(listResult)
	if !strings.Contains(text, "[project]") {
		t.Errorf("expected project entry in 'both' listing, got: %s", text)
	}
	if !strings.Contains(text, "[user]") {
		t.Errorf("expected user entry in 'both' listing, got: %s", text)
	}
}

func makeJournalServerWithRemote(t *testing.T, handler http.Handler) *Server {
	t.Helper()
	tmpDir := t.TempDir()
	journal, _ := storage.NewJournalMDStore(
		filepath.Join(tmpDir, "project"),
		filepath.Join(tmpDir, "user"),
	)
	social, _ := storage.NewSocialMDStore(filepath.Join(tmpDir, "social"))
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)
	remote := storage.NewRemoteClient(ts.URL, "test-key", "test-team")
	server, err := NewServer(journal, social, WithRemoteClient(remote))
	if err != nil {
		t.Fatalf("NewServer error: %v", err)
	}
	return server
}

func TestProcessThoughtsRemoteSync(t *testing.T) {
	var receivedBody []byte
	var receivedPath string
	var receivedAuth string

	s := makeJournalServerWithRemote(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		receivedAuth = r.Header.Get("x-api-key")
		receivedBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusCreated)
	}))

	result := callTool(t, s, "process_thoughts", map[string]string{
		"feelings":      "Feeling great",
		"project_notes": "Working on pulse",
	})

	if result.IsError {
		t.Fatalf("expected success, got error: %s", getTextContent(result))
	}

	// Verify remote received the entry
	if receivedPath != "/teams/test-team/journal/entries" {
		t.Errorf("expected remote path /teams/test-team/journal/entries, got %s", receivedPath)
	}
	if receivedAuth != "test-key" {
		t.Errorf("expected x-api-key 'test-key', got %q", receivedAuth)
	}

	// Verify payload contains ALL sections (no project/user split for remote)
	var payload struct {
		TeamID    string            `json:"team_id"`
		Timestamp int64             `json:"timestamp"`
		Sections  map[string]string `json:"sections"`
	}
	if err := json.Unmarshal(receivedBody, &payload); err != nil {
		t.Fatalf("failed to unmarshal remote body: %v", err)
	}
	if payload.TeamID != "test-team" {
		t.Errorf("expected team_id 'test-team', got %q", payload.TeamID)
	}
	if payload.Timestamp <= 0 {
		t.Errorf("expected positive timestamp, got %d", payload.Timestamp)
	}
	if payload.Sections["feelings"] != "Feeling great" {
		t.Errorf("expected feelings in remote payload, got %q", payload.Sections["feelings"])
	}
	if payload.Sections["project_notes"] != "Working on pulse" {
		t.Errorf("expected project_notes in remote payload, got %q", payload.Sections["project_notes"])
	}

	// Verify response doesn't mention remote failure
	text := getTextContent(result)
	if strings.Contains(text, "remote sync failed") {
		t.Errorf("did not expect remote failure warning, got: %s", text)
	}
}

func TestProcessThoughtsRemoteSyncFailure(t *testing.T) {
	s := makeJournalServerWithRemote(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("server error"))
	}))

	result := callTool(t, s, "process_thoughts", map[string]string{
		"feelings": "Feeling great despite errors",
	})

	// Local write should still succeed
	if result.IsError {
		t.Fatalf("expected success (local write), got error: %s", getTextContent(result))
	}

	text := getTextContent(result)
	// Should have path (local write succeeded)
	if !strings.Contains(text, "Path:") {
		t.Errorf("expected Path: in response (local write), got: %s", text)
	}
	// Should warn about remote failure
	if !strings.Contains(text, "remote sync failed") {
		t.Errorf("expected 'remote sync failed' warning, got: %s", text)
	}
}

func TestProcessThoughtsUnknownSection(t *testing.T) {
	s := makeJournalServer(t)

	result := callTool(t, s, "process_thoughts", map[string]interface{}{
		"feelings":     "Valid section",
		"invalid_name": "This should be rejected",
	})

	if !result.IsError {
		t.Error("expected error for unknown section name")
	}
	text := getTextContent(result)
	if !strings.Contains(text, "unknown section") {
		t.Errorf("expected 'unknown section' error, got: %s", text)
	}
	if !strings.Contains(text, "invalid_name") {
		t.Errorf("expected 'invalid_name' in error, got: %s", text)
	}
}

func TestListRecentEntriesEmpty(t *testing.T) {
	s := makeJournalServer(t)

	result := callTool(t, s, "list_recent_entries", map[string]interface{}{})

	if result.IsError {
		t.Fatalf("expected success, got error: %s", getTextContent(result))
	}

	text := getTextContent(result)
	if !strings.Contains(text, "No recent entries") {
		t.Errorf("expected 'No recent entries', got: %s", text)
	}
}
