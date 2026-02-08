// ABOUTME: HTTP client for remote social media API posting.
// ABOUTME: Syncs local social posts to a remote API when configured.
package storage

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/2389-research/pulse/internal/models"
	"github.com/google/uuid"
)

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
func (r *RemoteClient) CreateJournalEntry(sections map[string]string, timestamp time.Time) error {
	payload := remoteJournalPayload{
		TeamID:    r.teamID,
		Timestamp: timestamp.UnixMilli(),
		Sections:  sections,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal journal entry: %w", err)
	}

	req, err := http.NewRequest("POST", r.apiURL+"/teams/"+r.teamID+"/journal/entries", bytes.NewReader(body))
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
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("remote API returned %d: %s", resp.StatusCode, string(respBody))
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
func (r *RemoteClient) ReadJournalEntries(limit int) ([]*models.JournalEntry, error) {
	req, err := http.NewRequest("GET", r.apiURL+"/teams/"+r.teamID+"/journal/entries", nil)
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
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("remote API returned %d: %s", resp.StatusCode, string(respBody))
	}

	var listResp remoteJournalListResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
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
func (r *RemoteClient) CreatePost(post *models.SocialPost) error {
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

	req, err := http.NewRequest("POST", r.apiURL+"/teams/"+r.teamID+"/posts", bytes.NewReader(body))
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
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("remote API returned %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// ReadPosts fetches posts from the remote API.
func (r *RemoteClient) ReadPosts(opts ListPostsOptions) ([]*models.SocialPost, error) {
	req, err := http.NewRequest("GET", r.apiURL+"/teams/"+r.teamID+"/posts", nil)
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
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("remote API returned %d: %s", resp.StatusCode, string(respBody))
	}

	var listResp remoteListResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
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
