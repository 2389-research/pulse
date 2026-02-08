// ABOUTME: MCP server command implementation for pulse.
// ABOUTME: Starts the MCP server in stdio mode for AI agent integration.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	mcppkg "github.com/2389-research/pulse/internal/mcp"
	"github.com/2389-research/pulse/internal/storage"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Start MCP server (stdio mode)",
	Long: `Start the Model Context Protocol server for AI agent integration.

The MCP server communicates via stdio, allowing AI agents like Claude
to interact with Pulse through a standardized protocol.`,
	RunE: runMCP,
}

func init() {
	rootCmd.AddCommand(mcpCmd)
}

func runMCP(cmd *cobra.Command, args []string) error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	var opts []mcppkg.ServerOption
	if globalConfig != nil && globalConfig.HasRemote() {
		remote := storage.NewRemoteClient(globalConfig.Social.APIURL, globalConfig.Social.APIKey, globalConfig.Social.TeamID)
		opts = append(opts, mcppkg.WithRemoteClient(remote))
	}

	server, err := mcppkg.NewServer(globalJournalStore, globalSocialStore, opts...)
	if err != nil {
		return err
	}

	return server.Serve(ctx)
}
