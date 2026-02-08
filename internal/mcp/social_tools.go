// ABOUTME: MCP tool implementations for social media operations.
// ABOUTME: Registers login, create_post, and read_posts tools.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	gomcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/2389-research/pulse/internal/models"
	"github.com/2389-research/pulse/internal/storage"
)

func (s *Server) registerSocialTools() {
	s.mcp.AddTool(&gomcp.Tool{
		Name:        "login",
		Description: "Authenticate and set your unique agent identity for the social media session.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"agent_name": {"type": "string", "description": "Your unique social media handle/username.", "minLength": 1}
			},
			"required": ["agent_name"]
		}`),
	}, s.handleLogin)

	s.mcp.AddTool(&gomcp.Tool{
		Name:        "create_post",
		Description: "Create a new post or reply within the team.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"content": {"type": "string", "description": "The content of the post.", "minLength": 1},
				"tags": {"type": "array", "items": {"type": "string"}, "description": "Optional tags for the post"},
				"parent_post_id": {"type": "string", "description": "ID of the post to reply to (optional)"}
			},
			"required": ["content"]
		}`),
	}, s.handleCreatePost)

	s.mcp.AddTool(&gomcp.Tool{
		Name:        "read_posts",
		Description: "Retrieve posts from the social feed with optional filtering.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"limit": {"type": "number", "description": "Maximum number of posts to retrieve (default 10)"},
				"offset": {"type": "number", "description": "Number of posts to skip (default 0)"},
				"agent_filter": {"type": "string", "description": "Filter posts by author name"},
				"tag_filter": {"type": "string", "description": "Filter posts by tag"},
				"thread_id": {"type": "string", "description": "Get posts in a specific thread"}
			}
		}`),
	}, s.handleReadPosts)
}

func (s *Server) handleLogin(ctx context.Context, req *gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
	var args struct {
		AgentName string `json:"agent_name"`
	}
	if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
		return toolError("invalid arguments: %v", err), nil
	}

	if args.AgentName == "" {
		return toolError("agent_name is required"), nil
	}

	if err := s.social.SetIdentity(args.AgentName); err != nil {
		return toolError("failed to set identity: %v", err), nil
	}

	return &gomcp.CallToolResult{
		Content: []gomcp.Content{&gomcp.TextContent{
			Text: fmt.Sprintf("Logged in as %s", args.AgentName),
		}},
	}, nil
}

func (s *Server) handleCreatePost(ctx context.Context, req *gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
	var args struct {
		Content      string   `json:"content"`
		Tags         []string `json:"tags"`
		ParentPostID string   `json:"parent_post_id"`
	}
	if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
		return toolError("invalid arguments: %v", err), nil
	}

	if args.Content == "" {
		return toolError("content is required"), nil
	}

	identity, err := s.social.GetIdentity()
	if err != nil {
		return toolError("failed to get identity: %v", err), nil
	}
	if identity == "" {
		return toolError("not logged in - use the login tool first"), nil
	}

	var parentID *uuid.UUID
	if args.ParentPostID != "" {
		parsed, err := uuid.Parse(args.ParentPostID)
		if err != nil {
			return toolError("invalid parent_post_id: %v", err), nil
		}
		parentID = &parsed
	}

	post := models.NewSocialPost(identity, args.Content, args.Tags, parentID)
	if err := s.social.CreatePost(post); err != nil {
		return toolError("failed to create post: %v", err), nil
	}

	// Sync to remote if configured
	if s.remote != nil {
		if err := s.remote.CreatePost(post); err != nil {
			// Local write succeeded, remote failed - note but don't error
			return &gomcp.CallToolResult{
				Content: []gomcp.Content{&gomcp.TextContent{
					Text: fmt.Sprintf("Post created locally (ID: %s) but remote sync failed: %v", post.ID.String()[:8], err),
				}},
			}, nil
		}
		if err := s.social.MarkSynced(post.ID.String()); err != nil {
			// Non-fatal: post was synced but we couldn't update the flag
			_ = err
		}
	}

	return &gomcp.CallToolResult{
		Content: []gomcp.Content{&gomcp.TextContent{
			Text: fmt.Sprintf("Post created (ID: %s)", post.ID.String()[:8]),
		}},
	}, nil
}

func (s *Server) handleReadPosts(ctx context.Context, req *gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
	var args struct {
		Limit       int    `json:"limit"`
		Offset      int    `json:"offset"`
		AgentFilter string `json:"agent_filter"`
		TagFilter   string `json:"tag_filter"`
		ThreadID    string `json:"thread_id"`
	}
	if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
		return toolError("invalid arguments: %v", err), nil
	}

	if args.Limit <= 0 {
		args.Limit = 10
	}

	opts := storage.ListPostsOptions{
		Limit:       args.Limit,
		Offset:      args.Offset,
		AgentFilter: args.AgentFilter,
		TagFilter:   args.TagFilter,
		ThreadID:    args.ThreadID,
	}

	posts, err := s.social.ListPosts(opts)
	if err != nil {
		return toolError("failed to list posts: %v", err), nil
	}

	if len(posts) == 0 {
		return &gomcp.CallToolResult{
			Content: []gomcp.Content{&gomcp.TextContent{Text: "No posts found."}},
		}, nil
	}

	var sb strings.Builder
	for _, post := range posts {
		sb.WriteString(fmt.Sprintf("---\n@%s [%s]", post.AuthorName, post.CreatedAt.Format("2006-01-02 15:04:05")))
		if len(post.Tags) > 0 {
			sb.WriteString(fmt.Sprintf(" #%s", strings.Join(post.Tags, " #")))
		}
		if post.ParentPostID != nil {
			sb.WriteString(fmt.Sprintf(" (reply to %s)", post.ParentPostID.String()[:8]))
		}
		sb.WriteString(fmt.Sprintf("\n%s\n", post.Content))
	}

	return &gomcp.CallToolResult{
		Content: []gomcp.Content{&gomcp.TextContent{Text: sb.String()}},
	}, nil
}
