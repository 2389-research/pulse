// ABOUTME: Tests for remote social API client using httptest server.
// ABOUTME: Covers post creation, reading, error handling, and auth header passing.
package storage

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/2389-research/pulse/internal/models"
)

func TestRemoteClientCreatePost(t *testing.T) {
	var receivedBody []byte
	var receivedAuth string
	var receivedContentType string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/teams/test-team-id/posts" {
			t.Errorf("expected path /teams/test-team-id/posts, got %s", r.URL.Path)
		}
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		receivedAuth = r.Header.Get("x-api-key")
		receivedContentType = r.Header.Get("Content-Type")
		receivedBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	client := NewRemoteClient(server.URL, "test-api-key", "test-team-id")
	post := models.NewSocialPost("turbo_gecko", "Hello remote!", []string{"test"}, nil)

	err := client.CreatePost(post)
	if err != nil {
		t.Fatalf("CreatePost error: %v", err)
	}

	if receivedAuth != "test-api-key" {
		t.Errorf("expected 'test-api-key', got %q", receivedAuth)
	}
	if receivedContentType != "application/json" {
		t.Errorf("expected 'application/json', got %q", receivedContentType)
	}

	var payload remotePostPayload
	if err := json.Unmarshal(receivedBody, &payload); err != nil {
		t.Fatalf("failed to unmarshal request body: %v", err)
	}
	if payload.Content != "Hello remote!" {
		t.Errorf("expected content 'Hello remote!', got %q", payload.Content)
	}
	if payload.AuthorName != "turbo_gecko" {
		t.Errorf("expected author 'turbo_gecko', got %q", payload.AuthorName)
	}
}

func TestRemoteClientCreatePostError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("internal error"))
	}))
	defer server.Close()

	client := NewRemoteClient(server.URL, "key", "team")
	post := models.NewSocialPost("agent", "test", nil, nil)

	err := client.CreatePost(post)
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
	if err.Error() == "" {
		t.Error("expected non-empty error message")
	}
}

func TestRemoteClientReadPosts(t *testing.T) {
	var receivedAuth string
	var receivedPath string
	var receivedQuery string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		if r.Method != "GET" {
			t.Errorf("expected GET, got %s", r.Method)
		}
		receivedAuth = r.Header.Get("x-api-key")
		receivedQuery = r.URL.RawQuery

		resp := remoteListResponse{
			Posts: []remotePostResponse{
				{PostID: "00000000-0000-0000-0000-000000000001", Author: "agent1", Content: "Post 1", Tags: []string{"tag1"}, CreatedAt: remoteTimestamp{Seconds: 1700000000, Nanoseconds: 0}},
				{PostID: "00000000-0000-0000-0000-000000000002", Author: "agent2", Content: "Post 2", CreatedAt: remoteTimestamp{Seconds: 1700000100, Nanoseconds: 0}},
			},
			TotalCount: 2,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewRemoteClient(server.URL, "key", "team")
	posts, err := client.ReadPosts(ListPostsOptions{
		Limit:       5,
		AgentFilter: "agent1",
	})
	if err != nil {
		t.Fatalf("ReadPosts error: %v", err)
	}

	if receivedPath != "/teams/team/posts" {
		t.Errorf("expected path /teams/team/posts, got %s", receivedPath)
	}
	if receivedAuth != "key" {
		t.Errorf("expected 'key', got %q", receivedAuth)
	}
	if receivedQuery == "" {
		t.Error("expected query parameters to be sent")
	}

	if len(posts) != 2 {
		t.Fatalf("expected 2 posts, got %d", len(posts))
	}
	if posts[0].AuthorName != "agent1" {
		t.Errorf("expected author 'agent1', got %q", posts[0].AuthorName)
	}
	if posts[0].Content != "Post 1" {
		t.Errorf("expected content 'Post 1', got %q", posts[0].Content)
	}
}

func TestRemoteClientReadPostsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("unauthorized"))
	}))
	defer server.Close()

	client := NewRemoteClient(server.URL, "bad-key", "team")
	_, err := client.ReadPosts(ListPostsOptions{})
	if err == nil {
		t.Fatal("expected error for 401 response")
	}
}

func TestRemoteClientConnectionError(t *testing.T) {
	// Use a URL that will fail to connect
	client := NewRemoteClient("http://localhost:1", "key", "team")
	post := models.NewSocialPost("agent", "test", nil, nil)

	err := client.CreatePost(post)
	if err == nil {
		t.Fatal("expected error for connection failure")
	}
}

func TestRemoteClientQueryParams(t *testing.T) {
	var receivedQuery string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedQuery = r.URL.RawQuery
		resp := remoteListResponse{Posts: []remotePostResponse{}, TotalCount: 0}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewRemoteClient(server.URL, "key", "team")
	_, err := client.ReadPosts(ListPostsOptions{
		Limit:       10,
		AgentFilter: "bob",
		TagFilter:   "fun",
		ThreadID:    "abc-123",
	})
	if err != nil {
		t.Fatalf("ReadPosts error: %v", err)
	}

	// Verify query param names match the API contract
	if receivedQuery == "" {
		t.Fatal("expected query params")
	}
	// agent (not agent_filter), tag (not tag_filter)
	for _, expected := range []string{"agent=bob", "tag=fun", "thread_id=abc-123", "limit=10"} {
		if !strings.Contains(receivedQuery, expected) {
			t.Errorf("expected query to contain %q, got %q", expected, receivedQuery)
		}
	}
}

func TestRemoteClientCreateJournalEntry(t *testing.T) {
	var receivedBody []byte
	var receivedAuth string
	var receivedContentType string
	var receivedPath string
	var receivedMethod string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		receivedMethod = r.Method
		receivedAuth = r.Header.Get("x-api-key")
		receivedContentType = r.Header.Get("Content-Type")
		receivedBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	client := NewRemoteClient(server.URL, "test-api-key", "test-team-id")
	sections := map[string]string{
		"feelings":      "Feeling productive",
		"project_notes": "Working on pulse",
	}
	timestamp := time.Date(2024, 6, 1, 12, 30, 45, 123000000, time.UTC)

	err := client.CreateJournalEntry(sections, timestamp)
	if err != nil {
		t.Fatalf("CreateJournalEntry error: %v", err)
	}

	if receivedPath != "/teams/test-team-id/journal/entries" {
		t.Errorf("expected path /teams/test-team-id/journal/entries, got %s", receivedPath)
	}
	if receivedMethod != "POST" {
		t.Errorf("expected POST, got %s", receivedMethod)
	}
	if receivedAuth != "test-api-key" {
		t.Errorf("expected 'test-api-key', got %q", receivedAuth)
	}
	if receivedContentType != "application/json" {
		t.Errorf("expected 'application/json', got %q", receivedContentType)
	}

	var payload remoteJournalPayload
	if err := json.Unmarshal(receivedBody, &payload); err != nil {
		t.Fatalf("failed to unmarshal request body: %v", err)
	}
	if payload.TeamID != "test-team-id" {
		t.Errorf("expected team_id 'test-team-id', got %q", payload.TeamID)
	}
	// Unix ms for 2024-06-01T12:30:45.123Z
	expectedMs := timestamp.UnixMilli()
	if payload.Timestamp != expectedMs {
		t.Errorf("expected timestamp %d, got %d", expectedMs, payload.Timestamp)
	}
	if payload.Sections["feelings"] != "Feeling productive" {
		t.Errorf("expected feelings 'Feeling productive', got %q", payload.Sections["feelings"])
	}
	if payload.Sections["project_notes"] != "Working on pulse" {
		t.Errorf("expected project_notes 'Working on pulse', got %q", payload.Sections["project_notes"])
	}
}

func TestRemoteClientCreateJournalEntryError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("internal error"))
	}))
	defer server.Close()

	client := NewRemoteClient(server.URL, "key", "team")
	err := client.CreateJournalEntry(map[string]string{"feelings": "test"}, time.Now())
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("expected error to mention status code, got: %v", err)
	}
}

func TestRemoteClientReadJournalEntries(t *testing.T) {
	var receivedAuth string
	var receivedPath string
	var receivedQuery string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		receivedAuth = r.Header.Get("x-api-key")
		receivedQuery = r.URL.RawQuery
		if r.Method != "GET" {
			t.Errorf("expected GET, got %s", r.Method)
		}

		resp := remoteJournalListResponse{
			Entries: []remoteJournalEntryResponse{
				{
					ID:        "entry-1",
					TeamID:    "test-team",
					Timestamp: 1717243845123,
					CreatedAt: "2024-06-01T12:30:45.123Z",
					Sections: map[string]string{
						"feelings":      "Feeling great",
						"project_notes": "Working on pulse",
					},
				},
				{
					ID:        "entry-2",
					TeamID:    "test-team",
					Timestamp: 1717243900000,
					CreatedAt: "2024-06-01T12:31:40.000Z",
					Sections: map[string]string{
						"technical_insights": "Learned about TDD",
					},
				},
			},
			TotalCount: 2,
			HasMore:    false,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewRemoteClient(server.URL, "test-key", "test-team")
	entries, err := client.ReadJournalEntries(5)
	if err != nil {
		t.Fatalf("ReadJournalEntries error: %v", err)
	}

	if receivedPath != "/teams/test-team/journal/entries" {
		t.Errorf("expected path /teams/test-team/journal/entries, got %s", receivedPath)
	}
	if receivedAuth != "test-key" {
		t.Errorf("expected 'test-key', got %q", receivedAuth)
	}
	if !strings.Contains(receivedQuery, "limit=5") {
		t.Errorf("expected limit=5 in query, got %q", receivedQuery)
	}

	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Sections["feelings"] != "Feeling great" {
		t.Errorf("expected feelings 'Feeling great', got %q", entries[0].Sections["feelings"])
	}
	if entries[0].Sections["project_notes"] != "Working on pulse" {
		t.Errorf("expected project_notes, got %q", entries[0].Sections["project_notes"])
	}
	if entries[0].Type != "remote" {
		t.Errorf("expected type 'remote', got %q", entries[0].Type)
	}
	if entries[1].Sections["technical_insights"] != "Learned about TDD" {
		t.Errorf("expected technical_insights, got %q", entries[1].Sections["technical_insights"])
	}
}

func TestRemoteClientReadJournalEntriesError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("server error"))
	}))
	defer server.Close()

	client := NewRemoteClient(server.URL, "key", "team")
	_, err := client.ReadJournalEntries(10)
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

func TestRemoteClientStripsV1Suffix(t *testing.T) {
	var receivedPath string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		resp := remoteListResponse{Posts: []remotePostResponse{}, TotalCount: 0}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// URL with /v1 suffix should be stripped
	client := NewRemoteClient(server.URL+"/v1", "key", "myteam")
	_, err := client.ReadPosts(ListPostsOptions{Limit: 1})
	if err != nil {
		t.Fatalf("ReadPosts error: %v", err)
	}

	if receivedPath != "/teams/myteam/posts" {
		t.Errorf("expected /teams/myteam/posts (v1 stripped), got %s", receivedPath)
	}
}
