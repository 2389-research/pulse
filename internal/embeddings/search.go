// ABOUTME: Semantic search over journal entries using vector embeddings.
// ABOUTME: Falls back to substring matching when no embedder is available.
package embeddings

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/harperreed/mdstore"

	"github.com/2389-research/pulse/internal/models"
)

// SearchResult pairs a journal entry with its relevance score.
type SearchResult struct {
	Entry *models.JournalEntry
	Score float64
	Path  string
}

// SearchOptions configures a search operation.
type SearchOptions struct {
	Limit    int
	Type     string   // "project", "user", or "both"
	Sections []string // section name filter
}

// CosineSimilarity computes the cosine similarity between two vectors.
func CosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}

	var dotProduct, normA, normB float64
	for i := range a {
		dotProduct += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}

// SearchWithEmbeddings performs semantic search using vector embeddings.
// Scans .embedding sidecar files alongside journal .md files.
func SearchWithEmbeddings(embedder Embedder, roots []string, query string, opts SearchOptions) ([]SearchResult, error) {
	queryVec, err := embedder.Embed(query)
	if err != nil {
		return nil, err
	}

	var results []SearchResult

	for _, root := range roots {
		if _, err := os.Stat(root); os.IsNotExist(err) {
			continue
		}

		err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() || !strings.HasSuffix(path, ".embedding") {
				return nil
			}

			data, err := os.ReadFile(path)
			if err != nil {
				return nil
			}

			var emb models.Embedding
			if err := json.Unmarshal(data, &emb); err != nil {
				return nil
			}

			// Filter by sections if specified
			if len(opts.Sections) > 0 {
				match := false
				for _, s := range opts.Sections {
					for _, es := range emb.Sections {
						if s == es {
							match = true
							break
						}
					}
					if match {
						break
					}
				}
				if !match {
					return nil
				}
			}

			score := CosineSimilarity(queryVec, emb.Vector)
			results = append(results, SearchResult{
				Score: score,
				Path:  emb.Path,
			})

			return nil
		})
		if err != nil {
			continue
		}
	}

	// Sort by score descending
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	limit := opts.Limit
	if limit <= 0 {
		limit = 10
	}
	if limit > len(results) {
		limit = len(results)
	}

	return results[:limit], nil
}

// WriteEmbedding writes an embedding sidecar file alongside a journal entry.
func WriteEmbedding(mdPath string, embedder Embedder, sections map[string]string) error {
	// Concatenate all section content for embedding
	var parts []string
	var sectionNames []string
	for name, content := range sections {
		if content != "" {
			parts = append(parts, content)
			sectionNames = append(sectionNames, name)
		}
	}
	text := strings.Join(parts, "\n\n")

	vector, err := embedder.Embed(text)
	if err != nil {
		return err
	}

	emb := models.Embedding{
		Vector:    vector,
		Text:      text,
		Sections:  sectionNames,
		Timestamp: time.Now().Unix(),
		Path:      mdPath,
	}

	data, err := json.Marshal(emb)
	if err != nil {
		return err
	}

	embPath := strings.TrimSuffix(mdPath, ".md") + ".embedding"
	return mdstore.AtomicWrite(embPath, data)
}
