// ABOUTME: Tests for MCP server creation and validation.
// ABOUTME: Verifies server requires both journal and social stores.
package mcp

import (
	"testing"

	"github.com/2389-research/pulse/internal/storage"
)

func TestNewServerRequiresJournalStore(t *testing.T) {
	tmpDir := t.TempDir()
	social, _ := storage.NewSocialMDStore(tmpDir)

	_, err := NewServer(nil, social)
	if err == nil {
		t.Error("expected error when journal store is nil")
	}
}

func TestNewServerRequiresSocialStore(t *testing.T) {
	tmpDir := t.TempDir()
	journal, _ := storage.NewJournalMDStore(tmpDir, tmpDir)

	_, err := NewServer(journal, nil)
	if err == nil {
		t.Error("expected error when social store is nil")
	}
}

func TestNewServerSuccess(t *testing.T) {
	tmpDir := t.TempDir()
	journal, _ := storage.NewJournalMDStore(tmpDir, tmpDir)
	social, _ := storage.NewSocialMDStore(tmpDir)

	server, err := NewServer(journal, social)
	if err != nil {
		t.Fatalf("NewServer error: %v", err)
	}
	if server == nil {
		t.Error("expected non-nil server")
	}
}

func TestNewServerWithRemote(t *testing.T) {
	tmpDir := t.TempDir()
	journal, _ := storage.NewJournalMDStore(tmpDir, tmpDir)
	social, _ := storage.NewSocialMDStore(tmpDir)
	remote := storage.NewRemoteClient("http://example.com", "key", "team")

	server, err := NewServer(journal, social, WithRemoteClient(remote))
	if err != nil {
		t.Fatalf("NewServer error: %v", err)
	}
	if server.remote == nil {
		t.Error("expected remote client to be set")
	}
}
