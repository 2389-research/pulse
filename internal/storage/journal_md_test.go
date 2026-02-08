// ABOUTME: Tests for markdown-based journal storage.
// ABOUTME: Covers write/read roundtrip, dual-root listing, date ordering, and section parsing.
package storage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/2389-research/pulse/internal/models"
)

func TestJournalWriteReadRoundtrip(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "project-journal")
	userDir := filepath.Join(tmpDir, "user-journal")

	store, err := NewJournalMDStore(projectDir, userDir)
	if err != nil {
		t.Fatalf("NewJournalMDStore error: %v", err)
	}
	defer func() { _ = store.Close() }()

	sections := map[string]string{
		"feelings":           "I'm feeling great today",
		"project_notes":      "Working on pulse implementation",
		"technical_insights": "Go interfaces are powerful",
	}
	entry := models.NewJournalEntry(sections, "user")

	if err := store.WriteEntry(entry); err != nil {
		t.Fatalf("WriteEntry error: %v", err)
	}

	if entry.FilePath == "" {
		t.Fatal("expected FilePath to be set after write")
	}

	// Verify file exists
	if _, err := os.Stat(entry.FilePath); os.IsNotExist(err) {
		t.Fatalf("entry file not created at %s", entry.FilePath)
	}

	// Read it back
	read, err := store.ReadEntry(entry.FilePath)
	if err != nil {
		t.Fatalf("ReadEntry error: %v", err)
	}

	if read.ID != entry.ID {
		t.Errorf("ID mismatch: got %s, want %s", read.ID, entry.ID)
	}
	if read.Type != "user" {
		t.Errorf("Type mismatch: got %s, want user", read.Type)
	}
	for name, content := range sections {
		if read.Sections[name] != content {
			t.Errorf("Section %q mismatch: got %q, want %q", name, read.Sections[name], content)
		}
	}
}

func TestJournalProjectEntry(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "project-journal")
	userDir := filepath.Join(tmpDir, "user-journal")

	store, err := NewJournalMDStore(projectDir, userDir)
	if err != nil {
		t.Fatalf("NewJournalMDStore error: %v", err)
	}
	defer func() { _ = store.Close() }()

	entry := models.NewJournalEntry(map[string]string{
		"project_notes": "This is a project-specific note",
	}, "project")

	if err := store.WriteEntry(entry); err != nil {
		t.Fatalf("WriteEntry error: %v", err)
	}

	// Verify it was written to the project directory
	if !strings.HasPrefix(entry.FilePath, projectDir) {
		t.Errorf("expected project entry under %s, got %s", projectDir, entry.FilePath)
	}
}

func TestJournalDualRootListing(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "project-journal")
	userDir := filepath.Join(tmpDir, "user-journal")

	store, err := NewJournalMDStore(projectDir, userDir)
	if err != nil {
		t.Fatalf("NewJournalMDStore error: %v", err)
	}
	defer func() { _ = store.Close() }()

	// Write a user entry
	userEntry := models.NewJournalEntry(map[string]string{
		"feelings": "User-level feeling",
	}, "user")
	if err := store.WriteEntry(userEntry); err != nil {
		t.Fatalf("WriteEntry (user) error: %v", err)
	}

	// Write a project entry
	projectEntry := models.NewJournalEntry(map[string]string{
		"project_notes": "Project-level note",
	}, "project")
	if err := store.WriteEntry(projectEntry); err != nil {
		t.Fatalf("WriteEntry (project) error: %v", err)
	}

	// List both types
	entries, err := store.ListEntries("both", 0, 0)
	if err != nil {
		t.Fatalf("ListEntries (both) error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	// List only user entries
	userEntries, err := store.ListEntries("user", 0, 0)
	if err != nil {
		t.Fatalf("ListEntries (user) error: %v", err)
	}
	if len(userEntries) != 1 {
		t.Fatalf("expected 1 user entry, got %d", len(userEntries))
	}
	if userEntries[0].Type != "user" {
		t.Errorf("expected user type, got %s", userEntries[0].Type)
	}

	// List only project entries
	projectEntries, err := store.ListEntries("project", 0, 0)
	if err != nil {
		t.Fatalf("ListEntries (project) error: %v", err)
	}
	if len(projectEntries) != 1 {
		t.Fatalf("expected 1 project entry, got %d", len(projectEntries))
	}
	if projectEntries[0].Type != "project" {
		t.Errorf("expected project type, got %s", projectEntries[0].Type)
	}
}

func TestJournalDateOrdering(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "project-journal")
	userDir := filepath.Join(tmpDir, "user-journal")

	store, err := NewJournalMDStore(projectDir, userDir)
	if err != nil {
		t.Fatalf("NewJournalMDStore error: %v", err)
	}
	defer func() { _ = store.Close() }()

	// Write entries with different timestamps
	older := &models.JournalEntry{
		ID:        uuid.New(),
		Sections:  map[string]string{"feelings": "older entry"},
		CreatedAt: time.Now().Add(-2 * time.Hour),
		Type:      "user",
	}
	newer := &models.JournalEntry{
		ID:        uuid.New(),
		Sections:  map[string]string{"feelings": "newer entry"},
		CreatedAt: time.Now().Add(-1 * time.Hour),
		Type:      "user",
	}

	if err := store.WriteEntry(older); err != nil {
		t.Fatalf("WriteEntry (older) error: %v", err)
	}
	if err := store.WriteEntry(newer); err != nil {
		t.Fatalf("WriteEntry (newer) error: %v", err)
	}

	entries, err := store.ListEntries("both", 0, 0)
	if err != nil {
		t.Fatalf("ListEntries error: %v", err)
	}

	if len(entries) < 2 {
		t.Fatalf("expected at least 2 entries, got %d", len(entries))
	}

	// Most recent should be first
	if entries[0].CreatedAt.Before(entries[1].CreatedAt) {
		t.Error("expected entries to be sorted most recent first")
	}
}

func TestJournalLimit(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "project-journal")
	userDir := filepath.Join(tmpDir, "user-journal")

	store, err := NewJournalMDStore(projectDir, userDir)
	if err != nil {
		t.Fatalf("NewJournalMDStore error: %v", err)
	}
	defer func() { _ = store.Close() }()

	for i := 0; i < 5; i++ {
		entry := &models.JournalEntry{
			ID:        uuid.New(),
			Sections:  map[string]string{"feelings": "entry"},
			CreatedAt: time.Now().Add(time.Duration(i) * time.Minute),
			Type:      "user",
		}
		if err := store.WriteEntry(entry); err != nil {
			t.Fatalf("WriteEntry error: %v", err)
		}
	}

	entries, err := store.ListEntries("both", 3, 0)
	if err != nil {
		t.Fatalf("ListEntries error: %v", err)
	}

	if len(entries) != 3 {
		t.Errorf("expected 3 entries with limit, got %d", len(entries))
	}
}

func TestJournalDaysFilter(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "project-journal")
	userDir := filepath.Join(tmpDir, "user-journal")

	store, err := NewJournalMDStore(projectDir, userDir)
	if err != nil {
		t.Fatalf("NewJournalMDStore error: %v", err)
	}
	defer func() { _ = store.Close() }()

	// Write a recent entry
	recent := &models.JournalEntry{
		ID:        uuid.New(),
		Sections:  map[string]string{"feelings": "recent"},
		CreatedAt: time.Now(),
		Type:      "user",
	}
	if err := store.WriteEntry(recent); err != nil {
		t.Fatalf("WriteEntry error: %v", err)
	}

	// List with 1 day filter should include today's entry
	entries, err := store.ListEntries("both", 0, 1)
	if err != nil {
		t.Fatalf("ListEntries error: %v", err)
	}

	if len(entries) != 1 {
		t.Errorf("expected 1 entry within 1 day, got %d", len(entries))
	}
}

func TestJournalSectionParsing(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "project-journal")
	userDir := filepath.Join(tmpDir, "user-journal")

	store, err := NewJournalMDStore(projectDir, userDir)
	if err != nil {
		t.Fatalf("NewJournalMDStore error: %v", err)
	}
	defer func() { _ = store.Close() }()

	allSections := map[string]string{
		"feelings":           "I feel good",
		"project_notes":      "Working on pulse",
		"user_context":       "Doctor Biz is great",
		"technical_insights": "Go is fast",
		"world_knowledge":    "The sky is blue",
	}
	entry := models.NewJournalEntry(allSections, "user")

	if err := store.WriteEntry(entry); err != nil {
		t.Fatalf("WriteEntry error: %v", err)
	}

	read, err := store.ReadEntry(entry.FilePath)
	if err != nil {
		t.Fatalf("ReadEntry error: %v", err)
	}

	for name, expected := range allSections {
		got := read.Sections[name]
		if got != expected {
			t.Errorf("section %q: got %q, want %q", name, got, expected)
		}
	}
}

func TestJournalReadEntryPathTraversal(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "project-journal")
	userDir := filepath.Join(tmpDir, "user-journal")

	store, err := NewJournalMDStore(projectDir, userDir)
	if err != nil {
		t.Fatalf("NewJournalMDStore error: %v", err)
	}
	defer func() { _ = store.Close() }()

	// Write a valid entry so we know the store works
	entry := models.NewJournalEntry(map[string]string{
		"feelings": "test entry",
	}, "user")
	if err := store.WriteEntry(entry); err != nil {
		t.Fatalf("WriteEntry error: %v", err)
	}

	// Create a file outside the journal roots
	outsideFile := filepath.Join(tmpDir, "secret.txt")
	if err := os.WriteFile(outsideFile, []byte("secret data"), 0o644); err != nil {
		t.Fatalf("failed to create outside file: %v", err)
	}

	// Attempt to read a path outside the journal roots
	_, err = store.ReadEntry(outsideFile)
	if err == nil {
		t.Fatal("expected error when reading path outside journal roots")
	}
	if !strings.Contains(err.Error(), "outside journal roots") {
		t.Errorf("expected 'outside journal roots' error, got: %v", err)
	}

	// Attempt a path traversal via ..
	traversalPath := filepath.Join(userDir, "..", "secret.txt")
	_, err = store.ReadEntry(traversalPath)
	if err == nil {
		t.Fatal("expected error when reading path with traversal")
	}
	if !strings.Contains(err.Error(), "outside journal roots") {
		t.Errorf("expected 'outside journal roots' error, got: %v", err)
	}

	// Confirm we can still read the valid entry
	read, err := store.ReadEntry(entry.FilePath)
	if err != nil {
		t.Fatalf("ReadEntry of valid path failed: %v", err)
	}
	if read.ID != entry.ID {
		t.Errorf("ID mismatch: got %s, want %s", read.ID, entry.ID)
	}
}

func TestJournalEmptyRoots(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "nonexistent-project")
	userDir := filepath.Join(tmpDir, "nonexistent-user")

	store, err := NewJournalMDStore(projectDir, userDir)
	if err != nil {
		t.Fatalf("NewJournalMDStore error: %v", err)
	}
	defer func() { _ = store.Close() }()

	// Listing non-existent roots should return empty, not error
	entries, err := store.ListEntries("both", 0, 0)
	if err != nil {
		t.Fatalf("ListEntries error: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
}
