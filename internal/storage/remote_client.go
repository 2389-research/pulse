// ABOUTME: HTTP client for remote social media API posting.
// ABOUTME: Syncs local social posts to a remote API when configured.
package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/2389-research/pulse/internal/models"
	"github.com/google/uuid"
)

// maxResponseBytes caps how much of an HTTP response body we will read into
// memory. Prevents OOM when a remote server sends unexpectedly large payloads.
const maxResponseBytes = 1 << 20 // 1 MiB

// maxErrorBodyLen caps the length of response body text included in error messages.
const maxErrorBodyLen = 1024

// limitedReadBody reads up to maxResponseBytes from r and returns the bytes.
func limitedReadBody(r io.Reader) ([]byte, error) {
	return io.ReadAll(io.LimitReader(r, maxResponseBytes))
}

// truncatedErrorBody reads a limited response body and truncates it for use
// in error messages so that unexpectedly large responses don't bloat errors.
func truncatedErrorBody(r io.Reader) string {
	b, _ := limitedReadBody(r)
	if len(b) > maxErrorBodyLen {
		return string(b[:maxErrorBodyLen]) + "...(truncated)"
	}
	return string(b)
}

// RemoteClient posts social media content to a remote API.
type RemoteClient struct {
	apiURL string
	apiKey string
	teamID string
	client *http.Client
}

// NewRemoteClient creates a remote client with the given credentials.
func NewRemoteClient(apiURL, apiKey, teamID string) *RemoteClient {
	apiURL = strings.TrimRight(apiURL, "/")
	apiURL = strings.TrimSuffix(apiURL, "/v1")
	return &RemoteClient{
		apiURL: apiURL,
		apiKey: apiKey,
		teamID: teamID,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

// teamPath returns the base URL path for team-scoped API endpoints,
// with the team ID properly escaped to prevent path traversal.
func (r *RemoteClient) teamPath() string {
	return r.apiURL + "/teams/" + url.PathEscape(r.teamID)
}

// remotePostPayload is the JSON body sent to the remote API.
type remotePostPayload struct {
	Content      string   `json:"content"`
	AuthorName   string   `json:"author"`
	Tags         []string `json:"tags,omitempty"`
	ParentPostID string   `json:"parentPostId,omitempty"`
}

// remoteTimestamp represents a Firestore timestamp with _seconds and _nanoseconds.
type remoteTimestamp struct {
	Seconds     int64 `json:"_seconds"`
	Nanoseconds int64 `json:"_nanoseconds"`
}

// remotePostResponse maps a single post from the remote API response.
type remotePostResponse struct {
	PostID       string          `json:"postId"`
	Author       string          `json:"author"`
	Content      string          `json:"content"`
	Tags         []string        `json:"tags"`
	CreatedAt    remoteTimestamp `json:"createdAt"`
	ParentPostID string          `json:"parentPostId"`
}

// remoteListResponse is the top-level response envelope from GET /teams/{teamID}/posts.
type remoteListResponse struct {
	Posts      []remotePostResponse `json:"posts"`
	TotalCount int                  `json:"totalCount"`
}

// remoteJournalPayload is the JSON body sent to the remote journal API.
type remoteJournalPayload struct {
	TeamID    string            `json:"team_id"`
	Timestamp int64             `json:"timestamp"`
	Sections  map[string]string `json:"sections"`
}

// CreateJournalEntry posts a journal entry to the remote API.
func (r *RemoteClient) CreateJournalEntry(ctx context.Context, sections map[string]string, timestamp time.Time) error {
	payload := remoteJournalPayload{
		TeamID:    r.teamID,
		Timestamp: timestamp.UnixMilli(),
		Sections:  sections,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal journal entry: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", r.teamPath()+"/journal/entries", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", r.apiKey)

	resp, err := r.client.Do(req)
	if err != nil {
		return fmt.Errorf("remote API request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("remote API returned %d: %s", resp.StatusCode, truncatedErrorBody(resp.Body))
	}

	return nil
}

// remoteJournalEntryResponse maps a single journal entry from the remote API response.
type remoteJournalEntryResponse struct {
	ID        string            `json:"id"`
	TeamID    string            `json:"team_id"`
	Timestamp int64             `json:"timestamp"`
	CreatedAt string            `json:"created_at"`
	Sections  map[string]string `json:"sections"`
}

// remoteJournalListResponse is the top-level response envelope from GET /teams/{teamID}/journal/entries.
type remoteJournalListResponse struct {
	Entries    []remoteJournalEntryResponse `json:"entries"`
	TotalCount int                          `json:"total_count"`
	HasMore    bool                         `json:"has_more"`
	NextCursor string                       `json:"next_cursor"`
}

// ReadJournalEntries fetches journal entries from the remote API.
func (r *RemoteClient) ReadJournalEntries(ctx context.Context, limit int) ([]*models.JournalEntry, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", r.teamPath()+"/journal/entries", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("x-api-key", r.apiKey)

	q := req.URL.Query()
	if limit > 0 {
		q.Set("limit", fmt.Sprintf("%d", limit))
	}
	req.URL.RawQuery = q.Encode()

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("remote API request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("remote API returned %d: %s", resp.StatusCode, truncatedErrorBody(resp.Body))
	}

	var listResp remoteJournalListResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxResponseBytes)).Decode(&listResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	entries := make([]*models.JournalEntry, 0, len(listResp.Entries))
	for _, re := range listResp.Entries {
		entry := &models.JournalEntry{
			Sections: re.Sections,
			Type:     "remote",
		}
		if id, err := uuid.Parse(re.ID); err == nil {
			entry.ID = id
		}
		// Timestamp is Unix milliseconds
		if re.Timestamp > 0 {
			entry.CreatedAt = time.UnixMilli(re.Timestamp)
		}
		entries = append(entries, entry)
	}

	return entries, nil
}

// CreatePost sends a social post to the remote API.
func (r *RemoteClient) CreatePost(ctx context.Context, post *models.SocialPost) error {
	payload := remotePostPayload{
		Content:    post.Content,
		AuthorName: post.AuthorName,
		Tags:       post.Tags,
	}
	if post.ParentPostID != nil {
		payload.ParentPostID = post.ParentPostID.String()
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal post: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", r.teamPath()+"/posts", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", r.apiKey)

	resp, err := r.client.Do(req)
	if err != nil {
		return fmt.Errorf("remote API request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("remote API returned %d: %s", resp.StatusCode, truncatedErrorBody(resp.Body))
	}

	return nil
}

// ReadPosts fetches posts from the remote API.
func (r *RemoteClient) ReadPosts(ctx context.Context, opts ListPostsOptions) ([]*models.SocialPost, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", r.teamPath()+"/posts", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("x-api-key", r.apiKey)

	q := req.URL.Query()
	if opts.Limit > 0 {
		q.Set("limit", fmt.Sprintf("%d", opts.Limit))
	}
	if opts.Offset > 0 {
		q.Set("offset", fmt.Sprintf("%d", opts.Offset))
	}
	if opts.AgentFilter != "" {
		q.Set("agent", opts.AgentFilter)
	}
	if opts.TagFilter != "" {
		q.Set("tag", opts.TagFilter)
	}
	if opts.ThreadID != "" {
		q.Set("thread_id", opts.ThreadID)
	}
	req.URL.RawQuery = q.Encode()

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("remote API request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("remote API returned %d: %s", resp.StatusCode, truncatedErrorBody(resp.Body))
	}

	var listResp remoteListResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxResponseBytes)).Decode(&listResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	posts := make([]*models.SocialPost, 0, len(listResp.Posts))
	for _, rp := range listResp.Posts {
		post := &models.SocialPost{
			AuthorName: rp.Author,
			Content:    rp.Content,
			Tags:       rp.Tags,
		}
		if id, err := uuid.Parse(rp.PostID); err == nil {
			post.ID = id
		}
		if rp.CreatedAt.Seconds > 0 {
			post.CreatedAt = time.Unix(rp.CreatedAt.Seconds, rp.CreatedAt.Nanoseconds)
		}
		if rp.ParentPostID != "" {
			if pid, err := uuid.Parse(rp.ParentPostID); err == nil {
				post.ParentPostID = &pid
			}
		}
		posts = append(posts, post)
	}

	return posts, nil
}
