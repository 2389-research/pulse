// ABOUTME: Tests for botboard.biz API connection validation.
// ABOUTME: Uses httptest to verify auth headers, query params, and error handling.
package tui

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestValidateConnection_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/teams/test-team/posts" {
			t.Errorf("expected /teams/test-team/posts, got %s", r.URL.Path)
		}
		if r.Header.Get("x-api-key") != "test-key" {
			t.Errorf("expected x-api-key=test-key, got %s", r.Header.Get("x-api-key"))
		}
		if r.URL.Query().Get("limit") != "1" {
			t.Errorf("expected limit=1, got %s", r.URL.Query().Get("limit"))
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"posts":[],"totalCount":0}`))
	}))
	defer server.Close()

	err := ValidateConnection(context.Background(), server.URL, "test-key", "test-team")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestValidateConnection_Unauthorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"invalid api key"}`))
	}))
	defer server.Close()

	err := ValidateConnection(context.Background(), server.URL, "bad-key", "test-team")
	if err == nil {
		t.Fatal("expected error for 401 response")
	}
}

func TestValidateConnection_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`internal error`))
	}))
	defer server.Close()

	err := ValidateConnection(context.Background(), server.URL, "test-key", "test-team")
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

func TestValidateConnection_Unreachable(t *testing.T) {
	err := ValidateConnection(context.Background(), "http://localhost:1", "test-key", "test-team")
	if err == nil {
		t.Fatal("expected error for unreachable server")
	}
}

func TestValidateConnection_Cancelled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[]`))
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	err := ValidateConnection(ctx, server.URL, "test-key", "test-team")
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}
