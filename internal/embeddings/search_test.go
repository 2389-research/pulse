// ABOUTME: Tests for embedding search, cosine similarity, and sidecar file operations.
// ABOUTME: Uses a simple test embedder for deterministic vector testing.
package embeddings

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/2389-research/pulse/internal/models"
)

// testEmbedder produces deterministic embeddings for testing.
type testEmbedder struct {
	dim int
}

func (e *testEmbedder) Embed(text string) ([]float32, error) {
	// Simple hash-based embedding for deterministic testing.
	// Same text always produces the same vector.
	vec := make([]float32, e.dim)
	for i := range vec {
		h := 0
		for j, c := range text {
			h += int(c) * (i + 1) * (j + 1)
		}
		vec[i] = float32(h%1000) / 1000.0
	}
	// Normalize
	var norm float64
	for _, v := range vec {
		norm += float64(v) * float64(v)
	}
	norm = math.Sqrt(norm)
	if norm > 0 {
		for i := range vec {
			vec[i] = float32(float64(vec[i]) / norm)
		}
	}
	return vec, nil
}

func (e *testEmbedder) Dimension() int {
	return e.dim
}

func TestCosineSimilarityIdentical(t *testing.T) {
	a := []float32{1, 2, 3}
	score := CosineSimilarity(a, a)
	if math.Abs(score-1.0) > 0.0001 {
		t.Errorf("expected ~1.0 for identical vectors, got %f", score)
	}
}

func TestCosineSimilarityOrthogonal(t *testing.T) {
	a := []float32{1, 0, 0}
	b := []float32{0, 1, 0}
	score := CosineSimilarity(a, b)
	if math.Abs(score) > 0.0001 {
		t.Errorf("expected ~0.0 for orthogonal vectors, got %f", score)
	}
}

func TestCosineSimilarityOpposite(t *testing.T) {
	a := []float32{1, 0, 0}
	b := []float32{-1, 0, 0}
	score := CosineSimilarity(a, b)
	if math.Abs(score+1.0) > 0.0001 {
		t.Errorf("expected ~-1.0 for opposite vectors, got %f", score)
	}
}

func TestCosineSimilarityDifferentLengths(t *testing.T) {
	a := []float32{1, 2}
	b := []float32{1, 2, 3}
	score := CosineSimilarity(a, b)
	if score != 0 {
		t.Errorf("expected 0.0 for different length vectors, got %f", score)
	}
}

func TestCosineSimilarityEmpty(t *testing.T) {
	score := CosineSimilarity(nil, nil)
	if score != 0 {
		t.Errorf("expected 0.0 for nil vectors, got %f", score)
	}
}

func TestWriteEmbedding(t *testing.T) {
	tmpDir := t.TempDir()
	mdPath := filepath.Join(tmpDir, "test-entry.md")

	// Create a dummy md file
	if err := os.WriteFile(mdPath, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	embedder := &testEmbedder{dim: 8}
	sections := map[string]string{
		"feelings":      "I feel great",
		"project_notes": "Working on pulse",
	}

	err := WriteEmbedding(mdPath, embedder, sections)
	if err != nil {
		t.Fatalf("WriteEmbedding error: %v", err)
	}

	// Verify sidecar file exists
	embPath := filepath.Join(tmpDir, "test-entry.embedding")
	data, err := os.ReadFile(embPath)
	if err != nil {
		t.Fatalf("failed to read embedding file: %v", err)
	}

	var emb models.Embedding
	if err := json.Unmarshal(data, &emb); err != nil {
		t.Fatalf("failed to unmarshal embedding: %v", err)
	}

	if len(emb.Vector) != 8 {
		t.Errorf("expected 8-dim vector, got %d", len(emb.Vector))
	}
	if emb.Path != mdPath {
		t.Errorf("expected path %s, got %s", mdPath, emb.Path)
	}
	if len(emb.Sections) != 2 {
		t.Errorf("expected 2 sections, got %d", len(emb.Sections))
	}
}

func TestSearchWithEmbeddings(t *testing.T) {
	tmpDir := t.TempDir()
	embedder := &testEmbedder{dim: 8}

	// Create some test embedding files
	for _, name := range []string{"entry1", "entry2"} {
		mdPath := filepath.Join(tmpDir, name+".md")
		_ = os.WriteFile(mdPath, []byte("test"), 0644)

		text := "content for " + name
		vec, _ := embedder.Embed(text)
		emb := models.Embedding{
			Vector:   vec,
			Text:     text,
			Sections: []string{"feelings"},
			Path:     mdPath,
		}
		data, _ := json.Marshal(emb)
		_ = os.WriteFile(filepath.Join(tmpDir, name+".embedding"), data, 0644)
	}

	results, err := SearchWithEmbeddings(embedder, []string{tmpDir}, "content for entry1", SearchOptions{Limit: 10})
	if err != nil {
		t.Fatalf("SearchWithEmbeddings error: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	// The first result should have a higher score (more similar to query)
	if results[0].Score < results[1].Score {
		t.Error("expected results sorted by score descending")
	}
}

func TestSearchWithEmbeddingsSectionFilter(t *testing.T) {
	tmpDir := t.TempDir()
	embedder := &testEmbedder{dim: 8}

	// Create embedding with "feelings" section
	vec1, _ := embedder.Embed("feelings content")
	emb1 := models.Embedding{
		Vector:   vec1,
		Text:     "feelings content",
		Sections: []string{"feelings"},
		Path:     filepath.Join(tmpDir, "entry1.md"),
	}
	data1, _ := json.Marshal(emb1)
	_ = os.WriteFile(filepath.Join(tmpDir, "entry1.embedding"), data1, 0644)

	// Create embedding with "project_notes" section
	vec2, _ := embedder.Embed("project content")
	emb2 := models.Embedding{
		Vector:   vec2,
		Text:     "project content",
		Sections: []string{"project_notes"},
		Path:     filepath.Join(tmpDir, "entry2.md"),
	}
	data2, _ := json.Marshal(emb2)
	_ = os.WriteFile(filepath.Join(tmpDir, "entry2.embedding"), data2, 0644)

	// Search with section filter
	results, err := SearchWithEmbeddings(embedder, []string{tmpDir}, "content", SearchOptions{
		Limit:    10,
		Sections: []string{"feelings"},
	})
	if err != nil {
		t.Fatalf("SearchWithEmbeddings error: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result with section filter, got %d", len(results))
	}
}

func TestSearchNonexistentRoot(t *testing.T) {
	embedder := &testEmbedder{dim: 8}

	results, err := SearchWithEmbeddings(embedder, []string{"/nonexistent/path"}, "query", SearchOptions{Limit: 10})
	if err != nil {
		t.Fatalf("expected no error for nonexistent root, got: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}
