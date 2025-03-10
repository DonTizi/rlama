package service

import (
	"fmt"
	"strings"

	"github.com/dontizi/rlama/internal/client"
	"github.com/dontizi/rlama/internal/domain"
	"github.com/dontizi/rlama/internal/repository"
)

// Add this flag variable at the top of the file
var smallModelMode bool

// RagService manages operations related to RAG systems
type RagService struct {
	documentLoader   *DocumentLoader
	embeddingService *EmbeddingService
	ragRepository    *repository.RagRepository
	ollamaClient     *client.OllamaClient
}

// NewRagService creates a new instance of RagService
func NewRagService(ollamaClient *client.OllamaClient) *RagService {
	if ollamaClient == nil {
		ollamaClient = client.NewDefaultOllamaClient()
	}
	
	return &RagService{
		documentLoader:   NewDocumentLoader(),
		embeddingService: NewEmbeddingService(ollamaClient),
		ragRepository:    repository.NewRagRepository(),
		ollamaClient:     ollamaClient,
	}
}

// CreateRag creates a new RAG system
func (rs *RagService) CreateRag(modelName, ragName, folderPath string, chunkSize, chunkOverlap int) error {
	// Check if Ollama is available
	if err := rs.ollamaClient.CheckOllamaAndModel(modelName); err != nil {
		return err
	}

	// Check if the RAG already exists
	if rs.ragRepository.Exists(ragName) {
		return fmt.Errorf("a RAG with name '%s' already exists", ragName)
	}

	// Load documents
	docs, err := rs.documentLoader.LoadDocumentsFromFolder(folderPath)
	if err != nil {
		return fmt.Errorf("error loading documents: %w", err)
	}

	if len(docs) == 0 {
		return fmt.Errorf("no valid documents found in folder %s", folderPath)
	}

	fmt.Printf("Successfully loaded %d documents. Chunking documents...\n", len(docs))
	
	// Create the RAG system
	rag := domain.NewRagSystem(ragName, modelName)
	
	// Create chunker service with provided parameters
	chunkerService := NewChunkerService(chunkSize, chunkOverlap)
	
	// Process each document - chunk and generate embeddings
	var allChunks []*domain.DocumentChunk
	for _, doc := range docs {
		// Add the document to the RAG
		rag.AddDocument(doc)
		
		// Chunk the document
		chunks := chunkerService.ChunkDocument(doc)
		
		// Update total chunks in metadata
		for _, chunk := range chunks {
			chunk.UpdateTotalChunks(len(chunks))
		}
		
		allChunks = append(allChunks, chunks...)
	}
	
	fmt.Printf("Generated %d chunks from %d documents. Generating embeddings...\n", 
		len(allChunks), len(docs))
	
	// Generate embeddings for all chunks
	err = rs.embeddingService.GenerateChunkEmbeddings(allChunks, modelName)
	if err != nil {
		return fmt.Errorf("error generating embeddings: %w", err)
	}
	
	// Add all chunks to the RAG
	for _, chunk := range allChunks {
		rag.AddChunk(chunk)
	}

	// Save the RAG
	err = rs.ragRepository.Save(rag)
	if err != nil {
		return fmt.Errorf("error saving the RAG: %w", err)
	}

	fmt.Printf("RAG created with %d indexed documents (%d chunks).\n", len(docs), len(allChunks))
	return nil
}

// LoadRag loads a RAG system
func (rs *RagService) LoadRag(ragName string) (*domain.RagSystem, error) {
	rag, err := rs.ragRepository.Load(ragName)
	if err != nil {
		return nil, fmt.Errorf("error loading RAG '%s': %w", ragName, err)
	}

	return rag, nil
}

// Query performs a query on a RAG system
func (rs *RagService) Query(rag *domain.RagSystem, query string) (string, error) {
	// Check if Ollama is available
	if err := rs.ollamaClient.CheckOllamaAndModel(rag.ModelName); err != nil {
		return "", err
	}
	
	// Generate embedding for the query
	queryEmbedding, err := rs.embeddingService.GenerateQueryEmbedding(query, rag.ModelName)
	if err != nil {
		return "", fmt.Errorf("error generating embedding for query: %w", err)
	}

	// Use MaxRetrievedChunks if set, otherwise default to 20
	numChunks := 20
	if rag.MaxRetrievedChunks > 0 {
		numChunks = rag.MaxRetrievedChunks
	} else {
		// Apply small model detection here as fallback
		modelSize, err := rs.ollamaClient.GetModelSize(rag.ModelName)
		if err == nil && modelSize <= 7 {
			numChunks = 5 // For small models
		}
	}
	
	// Update the search to use the dynamic numChunks
	results := rag.VectorStore.Search(queryEmbedding, numChunks)
	
	// Build the context
	var context strings.Builder
	context.WriteString("Relevant information:\n\n")
	
	// Track which documents we've included for reference
	includedDocs := make(map[string]bool)
	
	for _, result := range results {
		chunk := rag.GetChunkByID(result.ID)
		if chunk != nil {
			// Add chunk content with its metadata
			context.WriteString(fmt.Sprintf("--- %s ---\n%s\n\n", 
				chunk.GetMetadataString(), chunk.Content))
				
			includedDocs[chunk.DocumentID] = true
		}
	}
	
	// Adjust prompt complexity based on model size
	var prompt string
	if numChunks == 5 {
		// Simplified prompt for small models
		prompt = fmt.Sprintf(`You are a helpful AI assistant. Follow these instructions carefully:
1. Use ONLY the information below to answer the question.
2. If the information doesn't contain the answer, say "I don't have enough information."
3. Keep your answer concise (maximum 3 sentences).
4. Include source references using (Source: document name).

Relevant information:
%s

Question: %s

Answer:`, context.String(), query)
	} else {
		// More complex prompt for larger models
		prompt = fmt.Sprintf(`You are a helpful AI assistant. Use the information below to answer the question.

%s

Question: %s

Answer based on the provided information. If the information doesn't contain the answer, say so clearly.
Include references to the source documents in your answer using the format (Source: document name).`, 
		context.String(), query)
	}
	
	// Show search results to the user
	fmt.Println("\nSearching documents...\n")
	fmt.Printf("Found %d relevant sections across %d documents\n", 
		len(results), len(includedDocs))
	
	// Generate the response
	response, err := rs.ollamaClient.GenerateCompletion(rag.ModelName, prompt)
	if err != nil {
		return "", fmt.Errorf("error generating response: %w", err)
	}
	
	return response, nil
}

// UpdateRag updates an existing RAG system
func (rs *RagService) UpdateRag(rag *domain.RagSystem) error {
	err := rs.ragRepository.Save(rag)
	if err != nil {
		return fmt.Errorf("error updating the RAG: %w", err)
	}
	return nil
}

func (rs *RagService) reRankResults(chunks []*domain.DocumentChunk, query string) []*domain.DocumentChunk {
	// Initial retrieval already done
	
	// Re-rank with cross-encoder or embedding similarity
	// For example, using query-chunk cosine similarity with BGE embeddings
	
	// Ensure diversity with MMR
	var rerankedChunks []*domain.DocumentChunk
	// MMR implementation to balance relevance and diversity
	
	// Return only top 3-5 chunks for small LLM
	return rerankedChunks[:min(5, len(rerankedChunks))]
}

func (rs *RagService) OptimizedQuery(rag *domain.RagSystem, query string) (string, error) {
	// Generate query embedding first
	queryEmbedding, err := rs.embeddingService.GenerateQueryEmbedding(query, rag.ModelName)
	if err != nil {
		return "", err
	}
	
	// Hybrid search (new)
	vectorResults := rag.VectorStore.Search(queryEmbedding, 15)
	// Temporarily disable hybrid search
	mergedResults := vectorResults
	
	// Convert vector.SearchResult to DocumentChunk pointers
	var docChunks []*domain.DocumentChunk
	for _, result := range mergedResults {
		if chunk := rag.GetChunkByID(result.ID); chunk != nil {
			docChunks = append(docChunks, chunk)
		}
	}
	
	// Re-rank to get top 5 most relevant and diverse chunks (new)
	rerankedResults := rs.reRankResults(docChunks, query)
	
	// Build concise context with only the best chunks
	var context strings.Builder
	for _, result := range rerankedResults {
		chunk := rag.GetChunkByID(result.ID)
		if chunk != nil {
			context.WriteString(fmt.Sprintf("--- %s ---\n%s\n\n", 
				chunk.GetMetadataString(), chunk.Content))
		}
	}
	
	// Optimized prompt for small LLMs
	prompt := fmt.Sprintf(`You are a helpful AI assistant. Follow these instructions carefully:
1. Use ONLY the information below to answer the question.
2. If the information doesn't contain the answer, say "I don't have enough information."
3. Keep your answer concise.
4. Include source references using (Source: document name).

Relevant information:
%s

Question: %s

Answer:`, context.String(), query)
	
	// Generate response (existing code)
	response, err := rs.ollamaClient.GenerateCompletion(rag.ModelName, prompt)
	return response, err
}