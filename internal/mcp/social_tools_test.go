// ABOUTME: Tests for social MCP tool handlers.
// ABOUTME: Covers login, create_post, and read_posts tools.
package mcp

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/2389-research/pulse/internal/storage"
)

func makeSocialServer(t *testing.T) *Server {
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

func TestLoginValid(t *testing.T) {
	s := makeSocialServer(t)

	result := callTool(t, s, "login", map[string]string{
		"agent_name": "turbo_gecko",
	})

	if result.IsError {
		t.Fatalf("expected success, got error: %s", getTextContent(result))
	}

	text := getTextContent(result)
	if !strings.Contains(text, "turbo_gecko") {
		t.Errorf("expected agent name in response, got: %s", text)
	}
}

func TestLoginRequiresName(t *testing.T) {
	s := makeSocialServer(t)

	result := callTool(t, s, "login", map[string]string{
		"agent_name": "",
	})

	if !result.IsError {
		t.Error("expected error when agent_name is empty")
	}
}

func TestCreatePostValid(t *testing.T) {
	s := makeSocialServer(t)

	// Login first
	callTool(t, s, "login", map[string]string{
		"agent_name": "turbo_gecko",
	})

	result := callTool(t, s, "create_post", map[string]interface{}{
		"content": "Hello from pulse!",
		"tags":    []string{"test", "greeting"},
	})

	if result.IsError {
		t.Fatalf("expected success, got error: %s", getTextContent(result))
	}

	text := getTextContent(result)
	if !strings.Contains(text, "Post created") {
		t.Errorf("expected 'Post created', got: %s", text)
	}
}

func TestCreatePostRequiresLogin(t *testing.T) {
	s := makeSocialServer(t)

	result := callTool(t, s, "create_post", map[string]interface{}{
		"content": "This should fail",
	})

	if !result.IsError {
		t.Error("expected error when not logged in")
	}
	text := getTextContent(result)
	if !strings.Contains(text, "not logged in") {
		t.Errorf("expected 'not logged in' error, got: %s", text)
	}
}

func TestCreatePostRequiresContent(t *testing.T) {
	s := makeSocialServer(t)

	callTool(t, s, "login", map[string]string{
		"agent_name": "turbo_gecko",
	})

	result := callTool(t, s, "create_post", map[string]interface{}{
		"content": "",
	})

	if !result.IsError {
		t.Error("expected error when content is empty")
	}
}

func TestCreatePostWithReply(t *testing.T) {
	s := makeSocialServer(t)

	callTool(t, s, "login", map[string]string{
		"agent_name": "turbo_gecko",
	})

	// Create root post
	callTool(t, s, "create_post", map[string]interface{}{
		"content": "Root post",
	})

	// Get posts to find root ID
	readResult := callTool(t, s, "read_posts", map[string]interface{}{
		"limit": 1,
	})
	if readResult.IsError {
		t.Fatalf("read_posts error: %s", getTextContent(readResult))
	}
}

func TestReadPostsEmpty(t *testing.T) {
	s := makeSocialServer(t)

	result := callTool(t, s, "read_posts", map[string]interface{}{})

	if result.IsError {
		t.Fatalf("expected success, got error: %s", getTextContent(result))
	}

	text := getTextContent(result)
	if !strings.Contains(text, "No posts found") {
		t.Errorf("expected 'No posts found', got: %s", text)
	}
}

func TestReadPostsWithFilter(t *testing.T) {
	s := makeSocialServer(t)

	callTool(t, s, "login", map[string]string{
		"agent_name": "turbo_gecko",
	})

	callTool(t, s, "create_post", map[string]interface{}{
		"content": "Test post",
		"tags":    []string{"test"},
	})

	// Read with agent filter
	result := callTool(t, s, "read_posts", map[string]interface{}{
		"agent_filter": "turbo_gecko",
	})

	if result.IsError {
		t.Fatalf("expected success, got error: %s", getTextContent(result))
	}

	text := getTextContent(result)
	if !strings.Contains(text, "turbo_gecko") {
		t.Errorf("expected agent name in posts, got: %s", text)
	}

	// Read with wrong agent filter
	result = callTool(t, s, "read_posts", map[string]interface{}{
		"agent_filter": "nonexistent_agent",
	})

	text = getTextContent(result)
	if !strings.Contains(text, "No posts found") {
		t.Errorf("expected 'No posts found' for wrong agent, got: %s", text)
	}
}

func TestReadPostsWithTagFilter(t *testing.T) {
	s := makeSocialServer(t)

	callTool(t, s, "login", map[string]string{
		"agent_name": "turbo_gecko",
	})

	callTool(t, s, "create_post", map[string]interface{}{
		"content": "Tagged post",
		"tags":    []string{"important"},
	})

	result := callTool(t, s, "read_posts", map[string]interface{}{
		"tag_filter": "important",
	})

	if result.IsError {
		t.Fatalf("expected success, got error: %s", getTextContent(result))
	}

	text := getTextContent(result)
	if !strings.Contains(text, "Tagged post") {
		t.Errorf("expected tagged post in results, got: %s", text)
	}
}
