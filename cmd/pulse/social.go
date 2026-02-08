// ABOUTME: CLI commands for social media operations.
// ABOUTME: Provides login, post, and feed subcommands for social interactions.
package main

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/2389-research/pulse/internal/models"
	"github.com/2389-research/pulse/internal/storage"
)

var socialCmd = &cobra.Command{
	Use:   "social",
	Short: "Manage social posts",
	Long:  "Create posts, read feeds, and manage social identity.",
}

var socialLoginCmd = &cobra.Command{
	Use:   "login <name>",
	Short: "Set your social identity",
	Long:  "Set the agent name used for posting.",
	Args:  cobra.ExactArgs(1),
	RunE:  runSocialLogin,
}

var socialPostCmd = &cobra.Command{
	Use:   "post <content>",
	Short: "Create a social post",
	Long:  "Create a new post with optional tags.",
	Args:  cobra.ExactArgs(1),
	RunE:  runSocialPost,
}

var socialFeedCmd = &cobra.Command{
	Use:   "feed",
	Short: "Read the social feed",
	Long:  "List social posts with optional filtering.",
	RunE:  runSocialFeed,
}

// Flags
var (
	socialTags      string
	socialParentID  string
	socialFeedLimit int
	socialAuthor    string
	socialTag       string
)

func init() {
	rootCmd.AddCommand(socialCmd)
	socialCmd.AddCommand(socialLoginCmd)
	socialCmd.AddCommand(socialPostCmd)
	socialCmd.AddCommand(socialFeedCmd)

	socialPostCmd.Flags().StringVar(&socialTags, "tags", "", "Comma-separated tags")
	socialPostCmd.Flags().StringVar(&socialParentID, "reply-to", "", "Parent post ID for threading")

	socialFeedCmd.Flags().IntVar(&socialFeedLimit, "limit", 10, "Maximum number of posts to show")
	socialFeedCmd.Flags().StringVar(&socialAuthor, "author", "", "Filter by author name")
	socialFeedCmd.Flags().StringVar(&socialTag, "tag", "", "Filter by tag")
}

func runSocialLogin(cmd *cobra.Command, args []string) error {
	name := args[0]
	if err := globalSocialStore.SetIdentity(name); err != nil {
		return fmt.Errorf("failed to set identity: %w", err)
	}
	fmt.Printf("Logged in as %s\n", name)
	return nil
}

func runSocialPost(cmd *cobra.Command, args []string) error {
	content := args[0]

	identity, err := globalSocialStore.GetIdentity()
	if err != nil {
		return fmt.Errorf("failed to get identity: %w", err)
	}
	if identity == "" {
		return fmt.Errorf("not logged in - run 'pulse social login <name>' first")
	}

	var tags []string
	if socialTags != "" {
		tags = strings.Split(socialTags, ",")
		for i := range tags {
			tags[i] = strings.TrimSpace(tags[i])
		}
	}

	var parentID *uuid.UUID
	if socialParentID != "" {
		parsed, err := uuid.Parse(socialParentID)
		if err != nil {
			return fmt.Errorf("invalid parent post ID: %w", err)
		}
		parentID = &parsed
	}

	post := models.NewSocialPost(identity, content, tags, parentID)
	if err := globalSocialStore.CreatePost(post); err != nil {
		return fmt.Errorf("failed to create post: %w", err)
	}

	fmt.Printf("Post created (ID: %s)\n", post.ID.String()[:8])
	return nil
}

func runSocialFeed(cmd *cobra.Command, args []string) error {
	opts := storage.ListPostsOptions{
		Limit:       socialFeedLimit,
		AgentFilter: socialAuthor,
		TagFilter:   socialTag,
	}

	var posts []*models.SocialPost
	var err error

	if globalConfig != nil && globalConfig.HasRemote() {
		remote := storage.NewRemoteClient(globalConfig.Social.APIURL, globalConfig.Social.APIKey, globalConfig.Social.TeamID)
		posts, err = remote.ReadPosts(opts)
	} else {
		posts, err = globalSocialStore.ListPosts(opts)
	}
	if err != nil {
		return fmt.Errorf("failed to list posts: %w", err)
	}

	if len(posts) == 0 {
		fmt.Println("No posts found.")
		return nil
	}

	for _, post := range posts {
		fmt.Printf("--- @%s [%s]", post.AuthorName, post.CreatedAt.Format("2006-01-02 15:04:05"))
		if len(post.Tags) > 0 {
			fmt.Printf(" #%s", strings.Join(post.Tags, " #"))
		}
		if post.ParentPostID != nil {
			fmt.Printf(" (reply to %s)", post.ParentPostID.String()[:8])
		}
		fmt.Printf("\n%s\n\n", post.Content)
	}
	return nil
}
