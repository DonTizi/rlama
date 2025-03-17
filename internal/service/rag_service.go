package service

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/dontizi/rlama/internal/client"
	"github.com/dontizi/rlama/internal/domain"
	"github.com/dontizi/rlama/internal/repository"
)

// Remove duplicate interface declaration and keep this one
type RagService interface {
	CreateRagWithOptions(modelName, ragName, folderPath string, options DocumentLoaderOptions) error
	GetRagChunks(ragName string, filter ChunkFilter) ([]*domain.DocumentChunk, error)
	LoadRag(ragName string) (*domain.RagSystem, error)
	Query(rag *domain.RagSystem, query string, contextSize int) (string, error)
	AddDocsWithOptions(ragName string, folderPath string, options DocumentLoaderOptions) error
	UpdateModel(ragName string, newModel string) error
	UpdateRag(rag *domain.RagSystem) error
	ListAllRags() ([]string, error)
	GetOllamaClient() *client.OllamaClient
	// Add new methods for watching
	SetupDirectoryWatching(ragName string, dirPath string, watchInterval int, options DocumentLoaderOptions) error
	DisableDirectoryWatching(ragName string) error
	CheckWatchedDirectory(ragName string) (int, error)
	// Add any other required methods here
	// Web watching methods (new)
	SetupWebWatching(ragName string, websiteURL string, watchInterval int, options domain.WebWatchOptions) error
	DisableWebWatching(ragName string) error
	CheckWatchedWebsite(ragName string) (int, error)
	// Search recherche des chunks pertinents dans un RAG
	Search(rag *domain.RagSystem, query string, limit int) ([]*domain.DocumentChunk, error)
}

// Update the struct implementation to match the interface
type RagServiceImpl struct {
	documentLoader   *DocumentLoader
	embeddingService *EmbeddingService
	ragRepository    *repository.RagRepository
	ollamaClient     *client.OllamaClient
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
	}
}

// CreateRagWithOptions creates a new RAG system with the specified options
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
	
	// Create chunker service
	chunkerService := NewChunkerService(ChunkingConfig{
		ChunkSize:    options.ChunkSize,
		ChunkOverlap: options.ChunkOverlap,
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

// Modify the existing CreateRag to use CreateRagWithOptions
func (rs *RagServiceImpl) CreateRag(modelName, ragName, folderPath string) error {
	return rs.CreateRagWithOptions(modelName, ragName, folderPath, DocumentLoaderOptions{})
}

// LoadRag loads a RAG system
func (rs *RagServiceImpl) LoadRag(ragName string) (*domain.RagSystem, error) {
	rag, err := rs.ragRepository.Load(ragName)
	if err != nil {
		return nil, fmt.Errorf("error loading RAG '%s': %w", ragName, err)
	}

	return rag, nil
}

// Query performs a query on a RAG system
func (rs *RagServiceImpl) Query(rag *domain.RagSystem, query string, contextSize int) (string, error) {
	// Rechercher les chunks pertinents
	relevantChunks, err := rs.Search(rag, query, contextSize)
	if err != nil {
		return "", fmt.Errorf("error searching for relevant chunks: %w", err)
	}

	if len(relevantChunks) == 0 {
		return "", fmt.Errorf("no relevant information found in the RAG system")
	}

	// Construire le prompt avec les chunks pertinents
	context := ""
	for _, chunk := range relevantChunks {
		docInfo := fmt.Sprintf("[Source: %s", chunk.DocumentID)
		
		// Ajouter l'URL si disponible
		for _, doc := range rag.Documents {
			if doc.ID == chunk.DocumentID && doc.URL != "" {
				docInfo += fmt.Sprintf(", URL: %s", doc.URL)
				break
			}
		}
		docInfo += "]\n"
		
		context += docInfo + chunk.Content + "\n\n"
	}

	// Construire le prompt avec les chunks pertinents
	systemMessage := "You are a helpful assistant that provides accurate information based on the documents you've been given. Answer the question based on the context provided. If you don't know the answer based on the context, say that you don't know rather than making up an answer."
	
	// Construire les messages pour l'API chat
	messages := []map[string]string{
		{"role": "system", "content": systemMessage},
		{"role": "system", "content": "Context information from documents:\n\n" + context},
		{"role": "user", "content": query},
	}

	// Générer la réponse
	response, err := rs.ollamaClient.GenerateCompletion(rag.ModelName, messages)
	if err != nil {
		return "", fmt.Errorf("error generating response: %w", err)
	}

	return response, nil
}

// UpdateRag updates an existing RAG system
func (rs *RagServiceImpl) UpdateRag(rag *domain.RagSystem) error {
	err := rs.ragRepository.Save(rag)
	if err != nil {
		return fmt.Errorf("error updating the RAG: %w", err)
	}
	return nil
}

// AddDocsWithOptions adds documents to an existing RAG system with options
func (rs *RagServiceImpl) AddDocsWithOptions(ragName string, folderPath string, options DocumentLoaderOptions) error {
	// Load existing RAG
	rag, err := rs.LoadRag(ragName)
	if err != nil {
		return err
	}

	// Load documents with options
	docs, err := rs.documentLoader.LoadDocumentsFromFolderWithOptions(folderPath, options)
	if err != nil {
		return fmt.Errorf("error loading documents: %w", err)
	}

	if len(docs) == 0 {
		return fmt.Errorf("no valid documents found in folder %s", folderPath)
	}

	// Create chunker service with default config
	chunkerService := NewChunkerService(DefaultChunkingConfig())
	
	// Process documents
	var allChunks []*domain.DocumentChunk
	for _, doc := range docs {
		chunks := chunkerService.ChunkDocument(doc)
		for _, chunk := range chunks {
			chunk.UpdateTotalChunks(len(chunks))
		}
		allChunks = append(allChunks, chunks...)
	}

	// Generate embeddings
	err = rs.embeddingService.GenerateChunkEmbeddings(allChunks, rag.ModelName)
	if err != nil {
		return err
	}

	// Add new chunks
	chunksAdded := 0
	existingChunks := make(map[string]bool)
	for _, chunk := range rag.Chunks {
		existingChunks[chunk.ID] = true
	}
	for _, chunk := range allChunks {
		if !existingChunks[chunk.ID] {
			rag.AddChunk(chunk)
			chunksAdded++
		}
	}

	// Save updated RAG
	return rs.UpdateRag(rag)
}

// UpdateModel updates the model of an existing RAG system
func (rs *RagServiceImpl) UpdateModel(ragName string, newModel string) error {
	rag, err := rs.LoadRag(ragName)
	if err != nil {
		return fmt.Errorf("error loading RAG: %w", err)
	}

	rag.ModelName = newModel
	return rs.UpdateRag(rag)
}

// Add method to list all RAGs
func (rs *RagServiceImpl) ListAllRags() ([]string, error) {
	return rs.ragRepository.ListAll()
}

// GetOllamaClient returns the Ollama client
func (rs *RagServiceImpl) GetOllamaClient() *client.OllamaClient {
	return rs.ollamaClient
}

// SetupDirectoryWatching configures a RAG to watch a directory for changes
func (rs *RagServiceImpl) SetupDirectoryWatching(ragName string, dirPath string, watchInterval int, options DocumentLoaderOptions) error {
	// Load the RAG
	rag, err := rs.LoadRag(ragName)
	if err != nil {
		return fmt.Errorf("error loading RAG: %w", err)
	}
	
	// Check if the directory exists
	dirInfo, err := os.Stat(dirPath)
	if os.IsNotExist(err) {
		return fmt.Errorf("directory '%s' does not exist", dirPath)
	} else if err != nil {
		return fmt.Errorf("error accessing directory: %w", err)
	}
	
	if !dirInfo.IsDir() {
		return fmt.Errorf("'%s' is not a directory", dirPath)
	}
	
	// Set up watching configuration
	rag.WatchedDir = dirPath
	rag.WatchInterval = watchInterval
	rag.WatchEnabled = true
	rag.LastWatchedAt = time.Time{} // Zero time to force first check
	
	// Save watch options
	rag.WatchOptions = domain.DocumentWatchOptions{
		ExcludeDirs:  options.ExcludeDirs,
		ExcludeExts:  options.ExcludeExts,
		ProcessExts:  options.ProcessExts,
		ChunkSize:    options.ChunkSize,
		ChunkOverlap: options.ChunkOverlap,
	}
	
	// Update the RAG
	return rs.UpdateRag(rag)
}

// DisableDirectoryWatching disables directory watching for a RAG
func (rs *RagServiceImpl) DisableDirectoryWatching(ragName string) error {
	// Load the RAG
	rag, err := rs.LoadRag(ragName)
	if err != nil {
		return fmt.Errorf("error loading RAG: %w", err)
	}
	
	// Disable watching
	rag.WatchEnabled = false
	
	// Update the RAG
	return rs.UpdateRag(rag)
}

// CheckWatchedDirectory manually checks a RAG's watched directory
func (rs *RagServiceImpl) CheckWatchedDirectory(ragName string) (int, error) {
	// Load the RAG
	rag, err := rs.LoadRag(ragName)
	if err != nil {
		return 0, fmt.Errorf("error loading RAG: %w", err)
	}
	
	// Check if watching is enabled
	if !rag.WatchEnabled || rag.WatchedDir == "" {
		return 0, fmt.Errorf("directory watching is not enabled for RAG '%s'", ragName)
	}
	
	// Create a file watcher and check for updates
	fileWatcher := NewFileWatcher(rs)
	return fileWatcher.CheckAndUpdateRag(rag)
}

// SetupWebWatching configures a RAG to watch a website for changes
func (rs *RagServiceImpl) SetupWebWatching(ragName string, websiteURL string, watchInterval int, options domain.WebWatchOptions) error {
	// Load the RAG
	rag, err := rs.LoadRag(ragName)
	if err != nil {
		return fmt.Errorf("error loading RAG: %w", err)
	}
	
	// Validate URL
	_, err = url.Parse(websiteURL)
	if err != nil {
		return fmt.Errorf("invalid website URL: %w", err)
	}
	
	// Set up watching configuration
	rag.WatchedURL = websiteURL
	rag.WebWatchInterval = watchInterval
	rag.WebWatchEnabled = true
	rag.LastWebWatchAt = time.Time{} // Zero time to force first check
	
	// Save watch options
	rag.WebWatchOptions = options
	
	// Update the RAG
	return rs.UpdateRag(rag)
}

// DisableWebWatching disables website watching for a RAG
func (rs *RagServiceImpl) DisableWebWatching(ragName string) error {
	// Load the RAG
	rag, err := rs.LoadRag(ragName)
	if err != nil {
		return fmt.Errorf("error loading RAG: %w", err)
	}
	
	// Disable watching
	rag.WebWatchEnabled = false
	
	// Update the RAG
	return rs.UpdateRag(rag)
}

// CheckWatchedWebsite manually checks a RAG's watched website for new content
func (rs *RagServiceImpl) CheckWatchedWebsite(ragName string) (int, error) {
	// Load the RAG
	rag, err := rs.LoadRag(ragName)
	if err != nil {
		return 0, fmt.Errorf("error loading RAG: %w", err)
	}
	
	// Check if watching is enabled
	if !rag.WebWatchEnabled || rag.WatchedURL == "" {
		return 0, fmt.Errorf("website watching is not enabled for RAG '%s'", ragName)
	}
	
	// Créer un web watcher, vérifier si agentService est nil
	var webWatcher *WebWatcher
	if rs.agentService == nil {
		webWatcher = NewWebWatcherWithoutAgent(rs)
	} else {
		webWatcher = NewWebWatcher(rs, rs.agentService)
	}
	
	return webWatcher.CheckAndUpdateRag(rag)
}

// Search recherche des chunks pertinents dans un RAG
func (rs *RagServiceImpl) Search(rag *domain.RagSystem, query string, limit int) ([]*domain.DocumentChunk, error) {
	// Cette fonction devrait déjà exister dans votre implémentation - sinon il faudra la créer
	// Elle devrait utiliser le système vectoriel pour trouver les chunks pertinents
	return rs.GetRagChunks(rag.Name, ChunkFilter{
		Query: query,
		Limit: limit,
	})
}

// Remove the entire ChunkFilter struct and just keep its functionality
func (rs *RagServiceImpl) GetRagChunks(ragName string, filter ChunkFilter) ([]*domain.DocumentChunk, error) {
	rag, err := rs.ragRepository.Load(ragName)
	if err != nil {
		return nil, fmt.Errorf("error loading RAG: %w", err)
	}

	var filtered []*domain.DocumentChunk
	for _, chunk := range rag.Chunks {
		// Apply document filter
		if filter.DocumentID != "" && chunk.DocumentID != filter.DocumentID {
			continue
		}

		// Clone chunk to avoid modifying original
		c := *chunk
		
		// Clear content if not requested
		if filter.Query != "" && !filter.ShowContent {
			c.Content = ""
		}
		
		// Apply document substring filter if provided
		if filter.DocumentSubstring != "" && !strings.Contains(chunk.DocumentID, filter.DocumentSubstring) {
			continue
		}

		filtered = append(filtered, &c)
	}

	return filtered, nil
}

// Méthode pour définir l'AgentService ultérieurement
func (r *RagServiceImpl) SetAgentService(agentService *AgentService) {
	r.agentService = agentService
}

// Correction de la méthode initializeWebWatcher
func (r *RagServiceImpl) initializeWebWatcher() *WebWatcher {
	if r.agentService == nil {
		// Sans AgentService, créer un WebWatcher avec fonctionnalités limitées
		return NewWebWatcherWithoutAgent(r)
	}
	return NewWebWatcher(r, r.agentService)
} 