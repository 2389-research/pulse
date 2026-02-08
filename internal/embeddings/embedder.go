// ABOUTME: Embedding interface and implementations for journal search.
// ABOUTME: Provides local ONNX-based embeddings with fallback to substring matching.
package embeddings

// Embedder generates vector embeddings from text.
type Embedder interface {
	// Embed returns a vector embedding for the given text.
	Embed(text string) ([]float32, error)

	// Dimension returns the dimensionality of the output vectors.
	Dimension() int
}
