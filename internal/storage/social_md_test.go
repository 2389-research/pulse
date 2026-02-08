// ABOUTME: Tests for markdown-based social post storage.
// ABOUTME: Covers create/list roundtrip, filtering, identity, and pagination.
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

func TestSocialCreateListRoundtrip(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewSocialMDStore(tmpDir)
	if err != nil {
		t.Fatalf("NewSocialMDStore error: %v", err)
	}
	defer func() { _ = store.Close() }()

	post := models.NewSocialPost("turbo_gecko", "Hello from pulse!", []string{"greeting", "test"}, nil)
	if err := store.CreatePost(post); err != nil {
		t.Fatalf("CreatePost error: %v", err)
	}

	posts, err := store.ListPosts(ListPostsOptions{Limit: 10})
	if err != nil {
		t.Fatalf("ListPosts error: %v", err)
	}

	if len(posts) != 1 {
		t.Fatalf("expected 1 post, got %d", len(posts))
	}

	got := posts[0]
	if got.ID != post.ID {
		t.Errorf("ID mismatch: got %s, want %s", got.ID, post.ID)
	}
	if got.AuthorName != "turbo_gecko" {
		t.Errorf("AuthorName: got %q, want %q", got.AuthorName, "turbo_gecko")
	}
	if got.Content != "Hello from pulse!" {
		t.Errorf("Content: got %q, want %q", got.Content, "Hello from pulse!")
	}
	if len(got.Tags) != 2 || got.Tags[0] != "greeting" || got.Tags[1] != "test" {
		t.Errorf("Tags: got %v, want [greeting, test]", got.Tags)
	}
}

func TestSocialAgentFilter(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewSocialMDStore(tmpDir)
	if err != nil {
		t.Fatalf("NewSocialMDStore error: %v", err)
	}
	defer func() { _ = store.Close() }()

	post1 := &models.SocialPost{
		ID:         uuid.New(),
		AuthorName: "agent_a",
		Content:    "Post from agent A",
		CreatedAt:  time.Now(),
	}
	post2 := &models.SocialPost{
		ID:         uuid.New(),
		AuthorName: "agent_b",
		Content:    "Post from agent B",
		CreatedAt:  time.Now().Add(time.Second),
	}

	if err := store.CreatePost(post1); err != nil {
		t.Fatalf("CreatePost error: %v", err)
	}
	if err := store.CreatePost(post2); err != nil {
		t.Fatalf("CreatePost error: %v", err)
	}

	posts, err := store.ListPosts(ListPostsOptions{
		Limit:       10,
		AgentFilter: "agent_a",
	})
	if err != nil {
		t.Fatalf("ListPosts error: %v", err)
	}

	if len(posts) != 1 {
		t.Fatalf("expected 1 post for agent_a, got %d", len(posts))
	}
	if posts[0].AuthorName != "agent_a" {
		t.Errorf("expected agent_a, got %s", posts[0].AuthorName)
	}
}

func TestSocialTagFilter(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewSocialMDStore(tmpDir)
	if err != nil {
		t.Fatalf("NewSocialMDStore error: %v", err)
	}
	defer func() { _ = store.Close() }()

	post1 := models.NewSocialPost("agent", "Tagged post", []string{"important"}, nil)
	post2 := models.NewSocialPost("agent", "Untagged post", nil, nil)

	if err := store.CreatePost(post1); err != nil {
		t.Fatalf("CreatePost error: %v", err)
	}
	if err := store.CreatePost(post2); err != nil {
		t.Fatalf("CreatePost error: %v", err)
	}

	posts, err := store.ListPosts(ListPostsOptions{
		Limit:     10,
		TagFilter: "important",
	})
	if err != nil {
		t.Fatalf("ListPosts error: %v", err)
	}

	if len(posts) != 1 {
		t.Fatalf("expected 1 tagged post, got %d", len(posts))
	}
	if posts[0].Content != "Tagged post" {
		t.Errorf("expected 'Tagged post', got %q", posts[0].Content)
	}
}

func TestSocialThreadFilter(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewSocialMDStore(tmpDir)
	if err != nil {
		t.Fatalf("NewSocialMDStore error: %v", err)
	}
	defer func() { _ = store.Close() }()

	// Create a root post
	root := models.NewSocialPost("agent", "Root post", nil, nil)
	if err := store.CreatePost(root); err != nil {
		t.Fatalf("CreatePost error: %v", err)
	}

	// Create a reply
	reply := models.NewSocialPost("agent", "Reply post", nil, &root.ID)
	if err := store.CreatePost(reply); err != nil {
		t.Fatalf("CreatePost error: %v", err)
	}

	// Create an unrelated post
	unrelated := models.NewSocialPost("agent", "Unrelated post", nil, nil)
	if err := store.CreatePost(unrelated); err != nil {
		t.Fatalf("CreatePost error: %v", err)
	}

	posts, err := store.ListPosts(ListPostsOptions{
		Limit:    10,
		ThreadID: root.ID.String(),
	})
	if err != nil {
		t.Fatalf("ListPosts error: %v", err)
	}

	if len(posts) != 2 {
		t.Fatalf("expected 2 posts in thread (root + reply), got %d", len(posts))
	}
}

func TestSocialIdentity(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewSocialMDStore(tmpDir)
	if err != nil {
		t.Fatalf("NewSocialMDStore error: %v", err)
	}
	defer func() { _ = store.Close() }()

	// Initially empty
	name, err := store.GetIdentity()
	if err != nil {
		t.Fatalf("GetIdentity error: %v", err)
	}
	if name != "" {
		t.Errorf("expected empty identity, got %q", name)
	}

	// Set identity
	if err := store.SetIdentity("turbo_gecko"); err != nil {
		t.Fatalf("SetIdentity error: %v", err)
	}

	// Read back
	name, err = store.GetIdentity()
	if err != nil {
		t.Fatalf("GetIdentity error: %v", err)
	}
	if name != "turbo_gecko" {
		t.Errorf("expected 'turbo_gecko', got %q", name)
	}

	// Verify the file exists
	identityPath := filepath.Join(tmpDir, "_identity.yaml")
	if _, statErr := os.Stat(identityPath); statErr != nil {
		t.Errorf("identity file not found at %s", identityPath)
	}
}

func TestSocialPagination(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewSocialMDStore(tmpDir)
	if err != nil {
		t.Fatalf("NewSocialMDStore error: %v", err)
	}
	defer func() { _ = store.Close() }()

	// Create 5 posts with distinct timestamps
	for i := 0; i < 5; i++ {
		post := &models.SocialPost{
			ID:         uuid.New(),
			AuthorName: "agent",
			Content:    "Post content",
			CreatedAt:  time.Now().Add(time.Duration(i) * time.Second),
		}
		if err := store.CreatePost(post); err != nil {
			t.Fatalf("CreatePost error: %v", err)
		}
	}

	// Get first page
	page1, err := store.ListPosts(ListPostsOptions{Limit: 2, Offset: 0})
	if err != nil {
		t.Fatalf("ListPosts page 1 error: %v", err)
	}
	if len(page1) != 2 {
		t.Fatalf("expected 2 posts on page 1, got %d", len(page1))
	}

	// Get second page
	page2, err := store.ListPosts(ListPostsOptions{Limit: 2, Offset: 2})
	if err != nil {
		t.Fatalf("ListPosts page 2 error: %v", err)
	}
	if len(page2) != 2 {
		t.Fatalf("expected 2 posts on page 2, got %d", len(page2))
	}

	// Pages should have different posts
	if page1[0].ID == page2[0].ID {
		t.Error("page 1 and page 2 should not overlap")
	}

	// Get last page
	page3, err := store.ListPosts(ListPostsOptions{Limit: 2, Offset: 4})
	if err != nil {
		t.Fatalf("ListPosts page 3 error: %v", err)
	}
	if len(page3) != 1 {
		t.Fatalf("expected 1 post on page 3, got %d", len(page3))
	}
}

func TestSocialMarkSynced(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewSocialMDStore(tmpDir)
	if err != nil {
		t.Fatalf("NewSocialMDStore error: %v", err)
	}
	defer func() { _ = store.Close() }()

	post := models.NewSocialPost("agent", "Sync test", nil, nil)
	if err := store.CreatePost(post); err != nil {
		t.Fatalf("CreatePost error: %v", err)
	}

	// Initially not synced
	posts, err := store.ListPosts(ListPostsOptions{Limit: 10})
	if err != nil {
		t.Fatalf("ListPosts error: %v", err)
	}
	if posts[0].Synced {
		t.Error("expected post to not be synced initially")
	}

	// Mark synced
	if err := store.MarkSynced(post.ID.String()); err != nil {
		t.Fatalf("MarkSynced error: %v", err)
	}

	// Verify synced
	posts, err = store.ListPosts(ListPostsOptions{Limit: 10})
	if err != nil {
		t.Fatalf("ListPosts error: %v", err)
	}
	if !posts[0].Synced {
		t.Error("expected post to be synced after MarkSynced")
	}
}

func TestSocialMarkSyncedNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewSocialMDStore(tmpDir)
	if err != nil {
		t.Fatalf("NewSocialMDStore error: %v", err)
	}
	defer func() { _ = store.Close() }()

	// Create one post so the posts directory exists
	post := models.NewSocialPost("agent", "Some post", nil, nil)
	if err := store.CreatePost(post); err != nil {
		t.Fatalf("CreatePost error: %v", err)
	}

	// Attempt to mark a non-existent post as synced
	fakeID := uuid.New().String()
	err = store.MarkSynced(fakeID)
	if err == nil {
		t.Fatal("expected error when marking non-existent post as synced")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}
}

func TestSocialEmptyStore(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewSocialMDStore(tmpDir)
	if err != nil {
		t.Fatalf("NewSocialMDStore error: %v", err)
	}
	defer func() { _ = store.Close() }()

	posts, err := store.ListPosts(ListPostsOptions{Limit: 10})
	if err != nil {
		t.Fatalf("ListPosts error: %v", err)
	}
	if len(posts) != 0 {
		t.Errorf("expected 0 posts from empty store, got %d", len(posts))
	}
}
