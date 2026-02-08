// ABOUTME: Markdown-based journal storage with dual-root support.
// ABOUTME: Stores entries as markdown files with YAML frontmatter in date-based directories.
package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/harper/suite/mdstore"
	"gopkg.in/yaml.v3"

	"github.com/2389-research/pulse/internal/models"
)

// JournalMDStore stores journal entries as markdown files in dual-root directories.
type JournalMDStore struct {
	projectPath string // project-local root (.private-journal/ in cwd)
	userPath    string // user-global root (~/.private-journal/)
}

// journalFrontmatter is the YAML frontmatter for journal entry files.
type journalFrontmatter struct {
	ID   string `yaml:"id"`
	Date string `yaml:"date"`
	Type string `yaml:"type"`
}

// NewJournalMDStore creates a journal store with the given project and user root paths.
func NewJournalMDStore(projectPath, userPath string) (*JournalMDStore, error) {
	return &JournalMDStore{
		projectPath: projectPath,
		userPath:    userPath,
	}, nil
}

// WriteEntry persists a journal entry to the appropriate root directory.
func (s *JournalMDStore) WriteEntry(entry *models.JournalEntry) error {
	root := s.userPath
	if entry.Type == "project" {
		root = s.projectPath
	}

	dateDir := entry.CreatedAt.Format("2006-01-02")
	timeStr := entry.CreatedAt.Format("15-04-05-000000")
	shortID := entry.ID.String()[:8]
	filename := timeStr + "-" + shortID + ".md"
	dir := filepath.Join(root, dateDir)
	path := filepath.Join(dir, filename)

	fm := journalFrontmatter{
		ID:   entry.ID.String(),
		Date: mdstore.FormatTime(entry.CreatedAt),
		Type: entry.Type,
	}

	body := renderSections(entry.Sections)

	content, err := mdstore.RenderFrontmatter(fm, body)
	if err != nil {
		return fmt.Errorf("failed to render frontmatter: %w", err)
	}

	if err := mdstore.AtomicWrite(path, []byte(content)); err != nil {
		return fmt.Errorf("failed to write entry: %w", err)
	}

	entry.FilePath = path
	return nil
}

// ReadEntry reads a journal entry from the given file path.
// The path must be within one of the journal roots (project or user).
func (s *JournalMDStore) ReadEntry(path string) (*models.JournalEntry, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}

	absProject, _ := filepath.Abs(s.projectPath)
	absUser, _ := filepath.Abs(s.userPath)

	if !strings.HasPrefix(absPath, absProject+string(filepath.Separator)) &&
		!strings.HasPrefix(absPath, absUser+string(filepath.Separator)) {
		return nil, fmt.Errorf("path %q is outside journal roots", path)
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read entry: %w", err)
	}

	return parseJournalEntry(absPath, string(data))
}

// ListEntries lists journal entries, filtered by type and date range.
func (s *JournalMDStore) ListEntries(entryType string, limit int, days int) ([]*models.JournalEntry, error) {
	var entries []*models.JournalEntry

	var roots []struct {
		path     string
		fileType string
	}

	if entryType == "both" || entryType == "user" || entryType == "" {
		roots = append(roots, struct {
			path     string
			fileType string
		}{s.userPath, "user"})
	}
	if entryType == "both" || entryType == "project" || entryType == "" {
		roots = append(roots, struct {
			path     string
			fileType string
		}{s.projectPath, "project"})
	}

	var cutoff time.Time
	if days > 0 {
		cutoff = time.Now().AddDate(0, 0, -days)
	}

	for _, root := range roots {
		rootEntries, err := listEntriesInRoot(root.path, cutoff)
		if err != nil {
			return nil, fmt.Errorf("failed to list entries in %s: %w", root.path, err)
		}
		entries = append(entries, rootEntries...)
	}

	// Sort by date descending (most recent first)
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].CreatedAt.After(entries[j].CreatedAt)
	})

	if limit > 0 && len(entries) > limit {
		entries = entries[:limit]
	}

	return entries, nil
}

// Close releases any resources held by the store.
func (s *JournalMDStore) Close() error {
	return nil
}

// listEntriesInRoot scans a root directory for journal entries.
func listEntriesInRoot(root string, cutoff time.Time) ([]*models.JournalEntry, error) {
	if _, err := os.Stat(root); os.IsNotExist(err) {
		return nil, nil
	}

	dateDirs, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}

	var entries []*models.JournalEntry

	for _, dateDir := range dateDirs {
		if !dateDir.IsDir() {
			continue
		}

		// Check date cutoff by directory name
		if !cutoff.IsZero() {
			dirDate, err := time.Parse("2006-01-02", dateDir.Name())
			if err != nil {
				continue
			}
			// Compare dates only: truncate cutoff to start of day
			cutoffDate := cutoff.Truncate(24 * time.Hour)
			if dirDate.Before(cutoffDate) {
				continue
			}
		}

		dirPath := filepath.Join(root, dateDir.Name())
		files, err := os.ReadDir(dirPath)
		if err != nil {
			continue
		}

		for _, file := range files {
			if file.IsDir() || !strings.HasSuffix(file.Name(), ".md") {
				continue
			}

			filePath := filepath.Join(dirPath, file.Name())
			data, err := os.ReadFile(filePath)
			if err != nil {
				continue
			}

			entry, err := parseJournalEntry(filePath, string(data))
			if err != nil {
				continue
			}

			entries = append(entries, entry)
		}
	}

	return entries, nil
}

// parseJournalEntry parses a markdown file into a JournalEntry.
func parseJournalEntry(path string, content string) (*models.JournalEntry, error) {
	yamlStr, body := mdstore.ParseFrontmatter(content)
	if yamlStr == "" {
		return nil, fmt.Errorf("no frontmatter found in %s", path)
	}

	var fm journalFrontmatter
	if err := yaml.Unmarshal([]byte(yamlStr), &fm); err != nil {
		return nil, fmt.Errorf("failed to parse frontmatter: %w", err)
	}

	id, err := uuid.Parse(fm.ID)
	if err != nil {
		return nil, fmt.Errorf("invalid UUID in frontmatter: %w", err)
	}

	createdAt, err := mdstore.ParseTime(fm.Date)
	if err != nil {
		return nil, fmt.Errorf("invalid date in frontmatter: %w", err)
	}

	sections := parseSections(body)

	return &models.JournalEntry{
		ID:        id,
		Sections:  sections,
		CreatedAt: createdAt,
		FilePath:  path,
		Type:      fm.Type,
	}, nil
}

// renderSections converts a sections map to markdown body text.
func renderSections(sections map[string]string) string {
	// Render in a stable order
	var sb strings.Builder
	for _, name := range models.ValidSections {
		content, ok := sections[name]
		if !ok || content == "" {
			continue
		}
		sb.WriteString(fmt.Sprintf("\n## %s\n%s\n", models.SectionTitle(name), content))
	}
	return sb.String()
}

// parseSections extracts sections from markdown body text.
func parseSections(body string) map[string]string {
	sections := make(map[string]string)
	lines := strings.Split(body, "\n")

	var currentSection string
	var currentContent strings.Builder

	for _, line := range lines {
		if strings.HasPrefix(line, "## ") {
			// Save previous section
			if currentSection != "" {
				sections[currentSection] = strings.TrimSpace(currentContent.String())
			}
			// Start new section
			heading := strings.TrimPrefix(line, "## ")
			currentSection = models.SectionKey(heading)
			currentContent.Reset()
		} else if currentSection != "" {
			currentContent.WriteString(line)
			currentContent.WriteString("\n")
		}
	}

	// Save last section
	if currentSection != "" {
		sections[currentSection] = strings.TrimSpace(currentContent.String())
	}

	return sections
}
