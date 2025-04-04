package service

import (
	"fmt"
	"strings"

	"github.com/dontizi/rlama/internal/client"
	"github.com/dontizi/rlama/internal/domain"
	"github.com/dontizi/rlama/internal/repository"
)

// RagService interface defines the contract for RAG operations
type RagService interface {
	CreateRagWithOptions(modelName, ragName, folderPath string, options DocumentLoaderOptions) error
	GetRagChunks(ragName string, filter ChunkFilter) ([]*domain.DocumentChunk, error)
	LoadRag(ragName string) (*domain.RagSystem, error)
	Query(rag *domain.RagSystem, query string, contextSize int) (string, error)
	AddDocsWithOptions(ragName string, folderPath string, options DocumentLoaderOptions) error
	UpdateModel(ragName string, newModel string) error
	UpdateRag(rag *domain.RagSystem) error
	UpdateRerankerModel(ragName string, model string) error
	ListAllRags() ([]string, error)
	GetOllamaClient() *client.OllamaClient
	// Directory watching methods
	SetupDirectoryWatching(ragName string, dirPath string, watchInterval int, options DocumentLoaderOptions) error
	DisableDirectoryWatching(ragName string) error
	CheckWatchedDirectory(ragName string) (int, error)
	// Web watching methods
	SetupWebWatching(ragName string, websiteURL string, watchInterval int, options domain.WebWatchOptions) error
	DisableWebWatching(ragName string) error
	CheckWatchedWebsite(ragName string) (int, error)
	// Search recherche des chunks pertinents dans un RAG
	Search(rag *domain.RagSystem, query string, limit int) ([]*domain.DocumentChunk, error)
}

// RagServiceImpl implements the RagService interface
type RagServiceImpl struct {
	documentLoader   *DocumentLoader
	embeddingService *EmbeddingService
	ragRepository    *repository.RagRepository
	ollamaClient     *client.OllamaClient
	rerankerService  *RerankerService
	agentService     *AgentService
}

// NewRagService creates a new instance of RagService
func NewRagService(clientParam *client.OllamaClient) *RagServiceImpl {
	var ollamaClient *client.OllamaClient

	if clientParam == nil {
		ollamaClient = client.NewDefaultOllamaClient()
	} else {
		ollamaClient = clientParam
	}

	return &RagServiceImpl{
		documentLoader:   NewDocumentLoader(),
		embeddingService: NewEmbeddingService(ollamaClient),
		ragRepository:    repository.NewRagRepository(),
		ollamaClient:     ollamaClient,
		rerankerService:  NewRerankerService(ollamaClient),
		agentService:     NewAgentService(ollamaClient, nil),
	}
}

// NewRagServiceWithEmbedding creates a new RagService with a specific embedding service
func NewRagServiceWithEmbedding(ollamaClient *client.OllamaClient, embeddingService *EmbeddingService) RagService {
	if ollamaClient == nil {
		ollamaClient = client.NewDefaultOllamaClient()
	}

	return &RagServiceImpl{
		documentLoader:   NewDocumentLoader(),
		embeddingService: embeddingService,
		ragRepository:    repository.NewRagRepository(),
		ollamaClient:     ollamaClient,
		rerankerService:  NewRerankerService(ollamaClient),
		agentService:     NewAgentService(ollamaClient, nil),
	}
}

// GetOllamaClient returns the Ollama client
func (rs *RagServiceImpl) GetOllamaClient() *client.OllamaClient {
	return rs.ollamaClient
}

// CreateRagWithOptions creates a new RAG system with options
func (rs *RagServiceImpl) CreateRagWithOptions(modelName, ragName, folderPath string, options DocumentLoaderOptions) error {
	// Check if Ollama is available
	if err := rs.ollamaClient.CheckOllamaAndModel(modelName); err != nil {
		return err
	}

	// Check if the RAG already exists
	if rs.ragRepository.Exists(ragName) {
		return fmt.Errorf("a RAG with name '%s' already exists", ragName)
	}

	// Load documents with options
	docs, err := rs.documentLoader.LoadDocumentsFromFolderWithOptions(folderPath, options)
	if err != nil {
		return fmt.Errorf("error loading documents: %w", err)
	}

	if len(docs) == 0 {
		return fmt.Errorf("no valid documents found in folder %s", folderPath)
	}

	fmt.Printf("Successfully loaded %d documents. Chunking documents...\n", len(docs))

	// Create the RAG system
	rag := domain.NewRagSystem(ragName, modelName)
	rag.ChunkingStrategy = options.ChunkingStrategy
	rag.APIProfileName = options.APIProfileName

	// Configure reranking options - enable by default
	rag.RerankerEnabled = true // Always enable reranking by default
	fmt.Println("Reranking enabled for better retrieval accuracy")

	// Only disable if explicitly set to false in options
	if !options.EnableReranker && options.RerankerModel == "" {
		// Check if EnableReranker field was explicitly set
		// This prevents the zero-value (false) from disabling reranking when the field isn't set
		rag.RerankerEnabled = false
		fmt.Println("Reranking disabled by user configuration")
	}

	// Set reranker model if specified, otherwise use the same model
	if options.RerankerModel != "" {
		rag.RerankerModel = options.RerankerModel
	} else {
		rag.RerankerModel = modelName
	}

	// Set reranker weight
	if options.RerankerWeight > 0 {
		rag.RerankerWeight = options.RerankerWeight
	} else {
		rag.RerankerWeight = 0.7 // Default to 70% reranker, 30% vector
	}

	// Set default TopK if not already set
	if rag.RerankerTopK <= 0 {
		rag.RerankerTopK = 5 // Default to 5 results
	}

	// Set chunking options in WatchOptions too
	rag.WatchOptions.ChunkSize = options.ChunkSize
	rag.WatchOptions.ChunkOverlap = options.ChunkOverlap
	rag.WatchOptions.ChunkingStrategy = options.ChunkingStrategy

	// Create chunker service
	chunkerService := NewChunkerService(ChunkingConfig{
		ChunkSize:        options.ChunkSize,
		ChunkOverlap:     options.ChunkOverlap,
		ChunkingStrategy: options.ChunkingStrategy,
	})

	// Process each document - chunk and generate embeddings
	var allChunks []*domain.DocumentChunk
	for _, doc := range docs {
		// Add the document to the RAG
		rag.AddDocument(doc)

		// Chunk the document
		chunks := chunkerService.ChunkDocument(doc)

		// Update total chunks in metadata
		for i, chunk := range chunks {
			chunk.ChunkNumber = i
			chunk.TotalChunks = len(chunks)
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

// GetRagChunks gets chunks from a RAG with filtering
func (rs *RagServiceImpl) GetRagChunks(ragName string, filter ChunkFilter) ([]*domain.DocumentChunk, error) {
	// Load the RAG
	rag, err := rs.LoadRag(ragName)
	if err != nil {
		return nil, fmt.Errorf("error loading RAG: %w", err)
	}

	var filteredChunks []*domain.DocumentChunk

	// Apply filters
	for _, chunk := range rag.Chunks {
		// Apply document name filter if provided
		if filter.DocumentSubstring != "" {
			docID := chunk.DocumentID
			doc := rag.GetDocumentByID(docID)
			if doc != nil && !strings.Contains(doc.Name, filter.DocumentSubstring) {
				continue
			}
		}

		filteredChunks = append(filteredChunks, chunk)
	}

	return filteredChunks, nil
}

// LoadRag loads a RAG system
func (rs *RagServiceImpl) LoadRag(ragName string) (*domain.RagSystem, error) {
	return rs.ragRepository.Load(ragName)
}

// Query performs a query on a RAG system
func (rs *RagServiceImpl) Query(rag *domain.RagSystem, query string, contextSize int) (string, error) {
	// Get the appropriate LLM client based on the model
	llmClient := client.GetLLMClient(rag.ModelName, rs.ollamaClient)

	// Rechercher les chunks pertinents
	relevantChunks, err := rs.Search(rag, query, contextSize)
	if err != nil {
		return "", fmt.Errorf("error searching for relevant chunks: %w", err)
	}

	if len(relevantChunks) == 0 {
		return "Je n'ai pas trouvé d'information pertinente pour répondre à votre question.", nil
	}

	// Construire le contexte à partir des chunks
	var context strings.Builder
	for _, chunk := range relevantChunks {
		context.WriteString(chunk.Content)
		context.WriteString("\n\n")
	}

	// Construire le prompt
	prompt := fmt.Sprintf(`En tant qu'assistant, utilisez le contexte suivant pour répondre à la question. 
Si vous ne trouvez pas la réponse dans le contexte, dites-le honnêtement.

Contexte:
%s

Question: %s

Réponse:`, context.String(), query)

	// Générer la réponse
	response, err := llmClient.GenerateCompletion(rag.ModelName, prompt)
	if err != nil {
		return "", fmt.Errorf("error generating completion: %w", err)
	}

	return response, nil
}

// AddDocsWithOptions adds documents to a RAG with options
func (rs *RagServiceImpl) AddDocsWithOptions(ragName string, folderPath string, options DocumentLoaderOptions) error {
	// Load the existing RAG system
	rag, err := rs.LoadRag(ragName)
	if err != nil {
		return fmt.Errorf("error loading RAG '%s': %w", ragName, err)
	}

	// Check if Ollama is available
	if err := rs.ollamaClient.CheckOllamaAndModel(rag.ModelName); err != nil {
		return err
	}

	// Load new documents with options
	newDocs, err := rs.documentLoader.LoadDocumentsFromFolderWithOptions(folderPath, options)
	if err != nil {
		return fmt.Errorf("error loading documents: %w", err)
	}

	if len(newDocs) == 0 {
		return fmt.Errorf("no valid documents found in folder %s", folderPath)
	}

	fmt.Printf("Successfully loaded %d new documents. Chunking documents...\n", len(newDocs))

	// Create chunker service with the same options as the RAG or from provided options
	chunkSize := rag.WatchOptions.ChunkSize
	chunkOverlap := rag.WatchOptions.ChunkOverlap
	chunkingStrategy := rag.ChunkingStrategy

	// Override with provided options if specified
	if options.ChunkSize > 0 {
		chunkSize = options.ChunkSize
	}
	if options.ChunkOverlap > 0 {
		chunkOverlap = options.ChunkOverlap
	}
	if options.ChunkingStrategy != "" {
		chunkingStrategy = options.ChunkingStrategy
	}

	// Create chunker with configured options
	chunkerService := NewChunkerService(ChunkingConfig{
		ChunkSize:        chunkSize,
		ChunkOverlap:     chunkOverlap,
		ChunkingStrategy: chunkingStrategy,
	})

	// Check for duplicates
	existingDocPaths := make(map[string]bool)
	for _, doc := range rag.Documents {
		existingDocPaths[doc.Path] = true
	}

	var uniqueDocs []*domain.Document
	var skippedDocs int

	// Filter out duplicate documents
	for _, doc := range newDocs {
		if existingDocPaths[doc.Path] {
			skippedDocs++
			continue
		}
		uniqueDocs = append(uniqueDocs, doc)
		existingDocPaths[doc.Path] = true // Mark as processed to avoid future duplicates
	}

	if len(uniqueDocs) == 0 {
		return fmt.Errorf("all %d documents already exist in the RAG, none added", skippedDocs)
	}

	if skippedDocs > 0 {
		fmt.Printf("Skipped %d documents that were already in the RAG.\n", skippedDocs)
	}

	// Process each unique document - chunk and generate embeddings
	var allChunks []*domain.DocumentChunk
	for _, doc := range uniqueDocs {
		// Add the document to the RAG
		rag.AddDocument(doc)

		// Chunk the document
		chunks := chunkerService.ChunkDocument(doc)

		// Update total chunks in metadata
		for i, chunk := range chunks {
			chunk.ChunkNumber = i
			chunk.TotalChunks = len(chunks)
		}

		allChunks = append(allChunks, chunks...)
	}

	fmt.Printf("Generated %d chunks from %d new documents. Generating embeddings...\n",
		len(allChunks), len(uniqueDocs))

	// Generate embeddings for all chunks
	err = rs.embeddingService.GenerateChunkEmbeddings(allChunks, rag.ModelName)
	if err != nil {
		return fmt.Errorf("error generating embeddings: %w", err)
	}

	// Add all chunks to the RAG
	for _, chunk := range allChunks {
		rag.AddChunk(chunk)
	}

	// Update the RAG's chunk options based on the most recent settings
	rag.WatchOptions.ChunkSize = chunkSize
	rag.WatchOptions.ChunkOverlap = chunkOverlap
	rag.ChunkingStrategy = chunkingStrategy

	// Update reranker settings if specified in options
	if options.RerankerModel != "" {
		rag.RerankerModel = options.RerankerModel
	}
	if options.RerankerWeight > 0 {
		rag.RerankerWeight = options.RerankerWeight
	}
	if options.EnableReranker {
		rag.RerankerEnabled = true
	}

	// Save the updated RAG
	err = rs.ragRepository.Save(rag)
	if err != nil {
		return fmt.Errorf("error saving the updated RAG: %w", err)
	}

	fmt.Printf("Successfully added %d new documents (%d chunks) to RAG '%s'.\n",
		len(uniqueDocs), len(allChunks), ragName)
	return nil
}

// UpdateModel updates the model of a RAG
func (rs *RagServiceImpl) UpdateModel(ragName string, newModel string) error {
	return nil
}

// UpdateRag updates a RAG system
func (rs *RagServiceImpl) UpdateRag(rag *domain.RagSystem) error {
	// Save the updated RAG
	err := rs.ragRepository.Save(rag)
	if err != nil {
		return fmt.Errorf("error saving updated RAG: %w", err)
	}

	fmt.Printf("RAG '%s' updated successfully.\n", rag.Name)
	return nil
}

// ListAllRags lists all available RAGs
func (rs *RagServiceImpl) ListAllRags() ([]string, error) {
	return nil, nil
}

// SetupDirectoryWatching sets up directory watching for a RAG
func (rs *RagServiceImpl) SetupDirectoryWatching(ragName string, dirPath string, watchInterval int, options DocumentLoaderOptions) error {
	return nil
}

// DisableDirectoryWatching disables directory watching for a RAG
func (rs *RagServiceImpl) DisableDirectoryWatching(ragName string) error {
	return nil
}

// CheckWatchedDirectory checks a watched directory for changes
func (rs *RagServiceImpl) CheckWatchedDirectory(ragName string) (int, error) {
	return 0, nil
}

// SetupWebWatching sets up web watching for a RAG
func (rs *RagServiceImpl) SetupWebWatching(ragName string, websiteURL string, watchInterval int, options domain.WebWatchOptions) error {
	return nil
}

// DisableWebWatching disables web watching for a RAG
func (rs *RagServiceImpl) DisableWebWatching(ragName string) error {
	return nil
}

// CheckWatchedWebsite checks a watched website for changes
func (rs *RagServiceImpl) CheckWatchedWebsite(ragName string) (int, error) {
	return 0, nil
}

// UpdateRerankerModel updates the reranker model of a RAG
func (rs *RagServiceImpl) UpdateRerankerModel(ragName string, model string) error {
	// Load the RAG
	rag, err := rs.LoadRag(ragName)
	if err != nil {
		return fmt.Errorf("error loading RAG: %w", err)
	}

	// Update the reranker model
	rag.RerankerModel = model

	// Save the updated RAG
	err = rs.ragRepository.Save(rag)
	if err != nil {
		return fmt.Errorf("error saving updated RAG: %w", err)
	}

	fmt.Printf("Reranker model updated to '%s' for RAG '%s'.\n", model, rag.Name)
	return nil
}

// Search recherche des chunks pertinents dans un RAG
func (rs *RagServiceImpl) Search(rag *domain.RagSystem, query string, limit int) ([]*domain.DocumentChunk, error) {
	return rs.GetRagChunks(rag.Name, ChunkFilter{
		Query:       query,
		Limit:       limit,
		ShowContent: true,
	})
}
