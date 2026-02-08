// ABOUTME: HTTP connection validation for botboard.biz API.
// ABOUTME: Tests credentials by fetching a single post from the remote API.
package tui

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ValidateConnection tests the API connection by fetching posts with the given credentials.
// The context allows cancellation when the user quits during validation.
func ValidateConnection(ctx context.Context, apiURL, apiKey, teamID string) error {
	client := &http.Client{Timeout: 10 * time.Second}

	apiURL = strings.TrimRight(apiURL, "/")
	apiURL = strings.TrimSuffix(apiURL, "/v1")

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL+"/teams/"+teamID+"/posts", nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("x-api-key", apiKey)

	q := req.URL.Query()
	q.Set("limit", "1")
	req.URL.RawQuery = q.Encode()

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return fmt.Errorf("API returned %d: %s", resp.StatusCode, string(body))
	}

	return nil
}
