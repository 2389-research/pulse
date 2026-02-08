// ABOUTME: Interface definition for journal entry storage.
// ABOUTME: Defines the contract for reading, writing, and listing journal entries.
package storage

import (
	"github.com/2389-research/pulse/internal/models"
)

// JournalStore defines operations for journal entry persistence.
type JournalStore interface {
	// WriteEntry persists a journal entry to disk.
	WriteEntry(entry *models.JournalEntry) error

	// ReadEntry reads a journal entry from the given file path.
	ReadEntry(path string) (*models.JournalEntry, error)

	// ListEntries lists journal entries, filtered by type ("project", "user", or "both").
	// limit caps the number of results. days limits how far back to look (0 = no limit).
	ListEntries(entryType string, limit int, days int) ([]*models.JournalEntry, error)

	// Close releases any resources held by the store.
	Close() error
}
