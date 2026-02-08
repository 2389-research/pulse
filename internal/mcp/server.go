// ABOUTME: MCP server initialization and configuration for pulse.
// ABOUTME: Sets up server with journal and social tools for AI agent access.
package mcp

import (
	"context"
	"fmt"

	gomcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/2389-research/pulse/internal/storage"
)

// Server wraps the MCP server with journal and social storage.
type Server struct {
	mcp     *gomcp.Server
	journal storage.JournalStore
	social  storage.SocialStore
	remote  *storage.RemoteClient
}

// ServerOption configures optional Server dependencies.
type ServerOption func(*Server)

// WithRemoteClient sets the remote API client for social posting.
func WithRemoteClient(rc *storage.RemoteClient) ServerOption {
	return func(s *Server) {
		s.remote = rc
	}
}

// NewServer creates an MCP server with journal and social capabilities.
func NewServer(journal storage.JournalStore, social storage.SocialStore, opts ...ServerOption) (*Server, error) {
	if journal == nil {
		return nil, fmt.Errorf("journal store is required")
	}
	if social == nil {
		return nil, fmt.Errorf("social store is required")
	}

	mcpServer := gomcp.NewServer(
		&gomcp.Implementation{
			Name:    "pulse",
			Version: "1.0.0",
		},
		nil,
	)

	s := &Server{
		mcp:     mcpServer,
		journal: journal,
		social:  social,
	}

	for _, opt := range opts {
		opt(s)
	}

	s.registerJournalTools()
	s.registerSocialTools()

	return s, nil
}

// Serve starts the MCP server in stdio mode.
func (s *Server) Serve(ctx context.Context) error {
	return s.mcp.Run(ctx, &gomcp.StdioTransport{})
}
