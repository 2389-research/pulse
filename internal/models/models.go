// ABOUTME: Core data models for journal entries, social posts, and embeddings.
// ABOUTME: Provides constructor functions and type definitions for pulse storage.
package models

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// JournalEntry represents a private journal entry with named sections.
type JournalEntry struct {
	ID        uuid.UUID
	Sections  map[string]string // feelings, project_notes, user_context, technical_insights, world_knowledge
	CreatedAt time.Time
	FilePath  string
	Type      string // "project" or "user"
}

// ValidSections lists the allowed journal section names.
var ValidSections = []string{
	"feelings",
	"project_notes",
	"user_context",
	"technical_insights",
	"world_knowledge",
}

// IsValidSection returns true if the given section name is valid.
func IsValidSection(name string) bool {
	for _, s := range ValidSections {
		if s == name {
			return true
		}
	}
	return false
}

// NewJournalEntry creates a journal entry with generated UUID and timestamp.
func NewJournalEntry(sections map[string]string, entryType string) *JournalEntry {
	return &JournalEntry{
		ID:        uuid.New(),
		Sections:  sections,
		CreatedAt: time.Now(),
		Type:      entryType,
	}
}

// SocialPost represents a social media post.
type SocialPost struct {
	ID           uuid.UUID
	AuthorName   string
	Content      string
	Tags         []string
	CreatedAt    time.Time
	ParentPostID *uuid.UUID
	Synced       bool
}

// NewSocialPost creates a social post with generated UUID and timestamp.
func NewSocialPost(authorName, content string, tags []string, parentPostID *uuid.UUID) *SocialPost {
	return &SocialPost{
		ID:           uuid.New(),
		AuthorName:   authorName,
		Content:      content,
		Tags:         tags,
		CreatedAt:    time.Now(),
		ParentPostID: parentPostID,
		Synced:       false,
	}
}

// SectionTitle converts a snake_case section name to a Title Case heading.
func SectionTitle(name string) string {
	parts := strings.Split(name, "_")
	for i, part := range parts {
		if len(part) > 0 {
			parts[i] = strings.ToUpper(part[:1]) + part[1:]
		}
	}
	return strings.Join(parts, " ")
}

// SectionKey converts a Title Case heading to a snake_case key.
func SectionKey(heading string) string {
	parts := strings.Fields(strings.ToLower(heading))
	return strings.Join(parts, "_")
}

// Embedding represents a vector embedding for a journal entry.
type Embedding struct {
	Vector    []float32 `json:"vector"`
	Text      string    `json:"text"`
	Sections  []string  `json:"sections"`
	Timestamp int64     `json:"timestamp"`
	Path      string    `json:"path"`
}
