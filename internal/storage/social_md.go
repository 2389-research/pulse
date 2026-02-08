// ABOUTME: Markdown-based social post storage with identity persistence.
// ABOUTME: Stores posts as markdown files and identity in YAML, with filtering and pagination.
package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/harperreed/mdstore"
	"gopkg.in/yaml.v3"

	"github.com/2389-research/pulse/internal/models"
)

// SocialMDStore stores social posts as markdown files in a data directory.
type SocialMDStore struct {
	dataDir string // root directory for social data
}

// socialFrontmatter is the YAML frontmatter for social post files.
type socialFrontmatter struct {
	ID           string   `yaml:"id"`
	Author       string   `yaml:"author"`
	Tags         []string `yaml:"tags,omitempty"`
	CreatedAt    string   `yaml:"created_at"`
	ParentPostID string   `yaml:"parent_post_id,omitempty"`
	Synced       bool     `yaml:"synced"`
}

// identityFile is the YAML structure for _identity.yaml.
type identityFile struct {
	AgentName string `yaml:"agent_name"`
}

// NewSocialMDStore creates a social store with the given data directory.
func NewSocialMDStore(dataDir string) (*SocialMDStore, error) {
	return &SocialMDStore{
		dataDir: dataDir,
	}, nil
}

// CreatePost persists a social post to disk.
func (s *SocialMDStore) CreatePost(post *models.SocialPost) error {
	postsDir := filepath.Join(s.dataDir, "posts")
	dateDir := post.CreatedAt.Format("2006-01-02")
	timeStr := post.CreatedAt.Format("15-04-05-000000")
	shortID := post.ID.String()[:8]
	filename := timeStr + "-" + shortID + ".md"
	dir := filepath.Join(postsDir, dateDir)
	path := filepath.Join(dir, filename)

	fm := socialFrontmatter{
		ID:        post.ID.String(),
		Author:    post.AuthorName,
		Tags:      post.Tags,
		CreatedAt: mdstore.FormatTime(post.CreatedAt),
		Synced:    post.Synced,
	}
	if post.ParentPostID != nil {
		fm.ParentPostID = post.ParentPostID.String()
	}

	content, err := mdstore.RenderFrontmatter(fm, post.Content+"\n")
	if err != nil {
		return fmt.Errorf("failed to render post: %w", err)
	}

	return mdstore.AtomicWrite(path, []byte(content))
}

// ListPosts returns posts matching the given filter options.
func (s *SocialMDStore) ListPosts(opts ListPostsOptions) ([]*models.SocialPost, error) {
	postsDir := filepath.Join(s.dataDir, "posts")

	if _, err := os.Stat(postsDir); os.IsNotExist(err) {
		return nil, nil
	}

	dateDirs, err := os.ReadDir(postsDir)
	if err != nil {
		return nil, err
	}

	var allPosts []*models.SocialPost

	for _, dateDir := range dateDirs {
		if !dateDir.IsDir() {
			continue
		}

		dirPath := filepath.Join(postsDir, dateDir.Name())
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

			post, err := parseSocialPost(string(data))
			if err != nil {
				continue
			}

			// Apply filters
			if opts.AgentFilter != "" && post.AuthorName != opts.AgentFilter {
				continue
			}
			if opts.TagFilter != "" && !containsTag(post.Tags, opts.TagFilter) {
				continue
			}
			if opts.ThreadID != "" {
				if post.ParentPostID == nil || post.ParentPostID.String() != opts.ThreadID {
					// Also match if the post itself IS the thread root
					if post.ID.String() != opts.ThreadID {
						continue
					}
				}
			}

			allPosts = append(allPosts, post)
		}
	}

	// Sort by date descending (most recent first)
	sort.Slice(allPosts, func(i, j int) bool {
		return allPosts[i].CreatedAt.After(allPosts[j].CreatedAt)
	})

	// Apply pagination
	if opts.Offset > 0 {
		if opts.Offset >= len(allPosts) {
			return nil, nil
		}
		allPosts = allPosts[opts.Offset:]
	}

	limit := opts.Limit
	if limit <= 0 {
		limit = 10
	}
	if limit > len(allPosts) {
		limit = len(allPosts)
	}

	return allPosts[:limit], nil
}

// GetIdentity returns the currently set agent name.
func (s *SocialMDStore) GetIdentity() (string, error) {
	path := filepath.Join(s.dataDir, "_identity.yaml")
	var id identityFile
	if err := mdstore.ReadYAML(path, &id); err != nil {
		return "", err
	}
	return id.AgentName, nil
}

// SetIdentity persists the agent name.
func (s *SocialMDStore) SetIdentity(name string) error {
	path := filepath.Join(s.dataDir, "_identity.yaml")
	return mdstore.WriteYAML(path, &identityFile{AgentName: name})
}

// errPostFound is a sentinel used to short-circuit filepath.Walk after finding the target post.
var errPostFound = fmt.Errorf("post found")

// MarkSynced marks a post as synced by rewriting the file with synced: true.
// Returns an error if the post is not found.
func (s *SocialMDStore) MarkSynced(postID string) error {
	postsDir := filepath.Join(s.dataDir, "posts")
	found := false

	walkErr := filepath.Walk(postsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		yamlStr, body := mdstore.ParseFrontmatter(string(data))
		if yamlStr == "" {
			return nil
		}

		var fm socialFrontmatter
		if err := yaml.Unmarshal([]byte(yamlStr), &fm); err != nil {
			return nil
		}

		if fm.ID != postID {
			return nil
		}

		// Found it - rewrite with synced: true
		found = true
		fm.Synced = true
		content, err := mdstore.RenderFrontmatter(fm, body)
		if err != nil {
			return err
		}

		if err := mdstore.AtomicWrite(path, []byte(content)); err != nil {
			return err
		}

		return errPostFound
	})

	if walkErr != nil && walkErr != errPostFound {
		return walkErr
	}

	if !found {
		return fmt.Errorf("post %s not found", postID)
	}

	return nil
}

// Close releases any resources held by the store.
func (s *SocialMDStore) Close() error {
	return nil
}

// parseSocialPost parses a markdown file into a SocialPost.
func parseSocialPost(content string) (*models.SocialPost, error) {
	yamlStr, body := mdstore.ParseFrontmatter(content)
	if yamlStr == "" {
		return nil, fmt.Errorf("no frontmatter found")
	}

	var fm socialFrontmatter
	if err := yaml.Unmarshal([]byte(yamlStr), &fm); err != nil {
		return nil, fmt.Errorf("failed to parse frontmatter: %w", err)
	}

	id, err := uuid.Parse(fm.ID)
	if err != nil {
		return nil, fmt.Errorf("invalid UUID: %w", err)
	}

	createdAt, err := mdstore.ParseTime(fm.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("invalid date: %w", err)
	}

	post := &models.SocialPost{
		ID:         id,
		AuthorName: fm.Author,
		Content:    strings.TrimSpace(body),
		Tags:       fm.Tags,
		CreatedAt:  createdAt,
		Synced:     fm.Synced,
	}

	if fm.ParentPostID != "" {
		parentID, err := uuid.Parse(fm.ParentPostID)
		if err == nil {
			post.ParentPostID = &parentID
		}
	}

	return post, nil
}

// containsTag checks if a tag list contains a specific tag.
func containsTag(tags []string, tag string) bool {
	for _, t := range tags {
		if t == tag {
			return true
		}
	}
	return false
}
