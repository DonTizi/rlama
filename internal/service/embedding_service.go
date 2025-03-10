package service

import (
	"fmt"

	"github.com/dontizi/rlama/internal/client"
	"github.com/dontizi/rlama/internal/domain"
)

// EmbeddingService manages the generation of embeddings for documents
type EmbeddingService struct {
	ollamaClient *client.OllamaClient
}

// NewEmbeddingService creates a new instance of EmbeddingService
func NewEmbeddingService(ollamaClient *client.OllamaClient) *EmbeddingService {
	if ollamaClient == nil {
		ollamaClient = client.NewDefaultOllamaClient()
	}
	return &EmbeddingService{
		ollamaClient: ollamaClient,
	}
}

// GenerateEmbeddings generates embeddings for a list of documents
func (es *EmbeddingService) GenerateEmbeddings(docs []*domain.Document, modelName string) error {
	// Try to use bge-m3 for embeddings first
	embeddingModel := "bge-m3"
	
	// Process all documents
	for _, doc := range docs {
		// Generate embedding with bge-m3 first
		embedding, err := es.ollamaClient.GenerateEmbedding(embeddingModel, doc.Content)
		
		// If bge-m3 fails, fallback to the specified model
		if err != nil {
			fmt.Printf("⚠️ Could not use %s for embeddings: %v\n", embeddingModel, err)
			fmt.Printf("Falling back to %s for embeddings. For better performance, consider:\n", modelName)
			fmt.Printf("  ollama pull bge-m3\n\n")
			
			// Try with the specified model instead
			embedding, err = es.ollamaClient.GenerateEmbedding(modelName, doc.Content)
			if err != nil {
				return fmt.Errorf("error generating embedding for %s: %w", doc.Path, err)
			}
		}

		doc.Embedding = embedding
	}

	return nil
}

// GenerateQueryEmbedding generates an embedding for a query
func (es *EmbeddingService) GenerateQueryEmbedding(query string, modelName string) ([]float32, error) {
	// Try to use bge-m3 for embeddings first
	embeddingModel := "bge-m3"
	
	// Generate embedding with bge-m3
	embedding, err := es.ollamaClient.GenerateEmbedding(embeddingModel, query)
	
	// If bge-m3 fails, fallback to the specified model
	if err != nil {
		// We don't need to show the warning again if already shown in GenerateEmbeddings
		
		// Try with the specified model instead
		embedding, err = es.ollamaClient.GenerateEmbedding(modelName, query)
		if err != nil {
			return nil, fmt.Errorf("error generating embedding for query: %w", err)
		}
	}

	return embedding, nil
}

// Add a new method for generating embeddings for document chunks
func (es *EmbeddingService) GenerateChunkEmbeddings(chunks []*domain.DocumentChunk, modelName string) error {
	// Try to use bge-m3 for embeddings first
	embeddingModel := "bge-m3"
	
	// Process all chunks
	for i, chunk := range chunks {
		fmt.Printf("Generating embedding for chunk %d/%d\r", i+1, len(chunks))
		
		// Generate embedding with bge-m3 first
		embedding, err := es.ollamaClient.GenerateEmbedding(embeddingModel, chunk.Content)
		
		// If bge-m3 fails, fallback to the specified model
		if err != nil {
			if i == 0 {  // Only show warning once
				fmt.Printf("\n⚠️ Could not use %s for embeddings: %v\n", embeddingModel, err)
				fmt.Printf("Falling back to %s for embeddings. For better performance, consider:\n", modelName)
				fmt.Printf("  ollama pull bge-m3\n\n")
			}
			
			// Try with the specified model instead
			embedding, err = es.ollamaClient.GenerateEmbedding(modelName, chunk.Content)
			if err != nil {
				return fmt.Errorf("error generating embedding for chunk %s: %w", chunk.ID, err)
			}
		}

		chunk.Embedding = embedding
	}
	fmt.Println() // Add a newline after progress indicator
	return nil
}