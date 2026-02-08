// ABOUTME: Interface definition for social post storage.
// ABOUTME: Defines the contract for creating, listing, and managing social posts and identity.
package storage

import (
	"github.com/2389-research/pulse/internal/models"
)

// ListPostsOptions configures filtering and pagination for listing posts.
type ListPostsOptions struct {
	Limit       int
	Offset      int
	AgentFilter string
	TagFilter   string
	ThreadID    string // parent_post_id to filter by thread
}

// SocialStore defines operations for social post persistence.
type SocialStore interface {
	// CreatePost persists a social post to disk.
	CreatePost(post *models.SocialPost) error

	// ListPosts returns posts matching the given filter options.
	ListPosts(opts ListPostsOptions) ([]*models.SocialPost, error)

	// GetIdentity returns the currently set agent name, or empty string if unset.
	GetIdentity() (string, error)

	// SetIdentity persists the agent name for this session/installation.
	SetIdentity(name string) error

	// MarkSynced marks a post as synced with the remote API.
	MarkSynced(postID string) error

	// Close releases any resources held by the store.
	Close() error
}
