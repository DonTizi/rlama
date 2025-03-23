package tests

import (
	"testing"

	"github.com/dontizi/rlama/internal/domain"
	"github.com/dontizi/rlama/internal/service"
	"github.com/stretchr/testify/assert"
)

func TestChunker(t *testing.T) {
	// Create a test document
	doc := &domain.Document{
		ID:   "test-doc",
		Name: "test.txt",
		Content: "This is a test document. It contains multiple sentences. " +
			"We will use it to test the chunking functionality. " +
			"Each chunk should have a reasonable size. " +
			"The chunker should handle various cases properly.",
	}

	t.Run("DefaultConfig", func(t *testing.T) {
		chunker := service.NewChunkerService(service.DefaultChunkingConfig())
		chunks := chunker.ChunkDocument(doc)

		assert.NotEmpty(t, chunks)
		for _, chunk := range chunks {
			assert.NotEmpty(t, chunk.Content)
			assert.Equal(t, doc.ID, chunk.DocumentID)
		}
	})

	t.Run("CustomConfig", func(t *testing.T) {
		config := service.ChunkingConfig{
			ChunkSize:    50, // Smaller chunks
			ChunkOverlap: 10, // Small overlap
		}
		chunker := service.NewChunkerService(config)
		chunks := chunker.ChunkDocument(doc)

		assert.NotEmpty(t, chunks)
		for _, chunk := range chunks {
			assert.NotEmpty(t, chunk.Content)
			assert.Equal(t, doc.ID, chunk.DocumentID)
			// Chunks should be roughly the configured size
			assert.LessOrEqual(t, len(chunk.Content), config.ChunkSize+config.ChunkOverlap)
		}
	})
}
