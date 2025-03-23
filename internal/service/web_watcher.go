package service

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dontizi/rlama/internal/crawler"
	"github.com/dontizi/rlama/internal/domain"
)

// WebWatcher is responsible for watching websites for content changes
type WebWatcher struct {
	ragService     RagService
	agentService   *AgentService
}

// NewWebWatcher creates a new web watcher service
func NewWebWatcher(ragService RagService, agentService *AgentService) *WebWatcher {
	return &WebWatcher{
		ragService:   ragService,
		agentService: agentService,
	}
}

// Fonction adaptée pour créer un WebWatcher sans AgentService
func NewWebWatcherWithoutAgent(ragService RagService) *WebWatcher {
	return &WebWatcher{
		ragService:   ragService,
		agentService: nil,
	}
}

// CheckAndUpdateRag checks for new content on the watched website and updates the RAG
func (ww *WebWatcher) CheckAndUpdateRag(rag *domain.RagSystem) (int, error) {
	if !rag.WebWatchEnabled || rag.WatchedURL == "" {
		return 0, nil // Watching not enabled
	}

	fmt.Printf("Checking for updates on %s\n", rag.WatchedURL)

	// Create a webcrawler to fetch the site content
	webCrawler, err := crawler.NewWebCrawler(
		rag.WatchedURL,
		rag.WebWatchOptions.MaxDepth,
		rag.WebWatchOptions.Concurrency,
		rag.WebWatchOptions.ExcludePaths,
	)
	if err != nil {
		return 0, fmt.Errorf("error initializing web crawler: %w", err)
	}

	// Start crawling
	documents, err := webCrawler.CrawlWebsite()
	if err != nil {
		return 0, fmt.Errorf("error crawling website: %w", err)
	}

	if len(documents) == 0 {
		fmt.Printf("No content found at %s\n", rag.WatchedURL)
		// Update last watched time even if no new documents
		rag.LastWebWatchAt = time.Now()
		err = ww.ragService.UpdateRag(rag)
		return 0, err
	}

	// Ensure all documents have a valid URL
	var validDocuments []*domain.Document // Changed to use pointers
	for i := range documents {
		doc := &documents[i] // Get the address of the document
		if doc.URL == "" {
			// Build a URL based on the path or a unique identifier
			if doc.Path != "" {
				doc.URL = rag.WatchedURL + doc.Path
			} else {
				doc.URL = fmt.Sprintf("%s/page_%d", rag.WatchedURL, i+1)
			}
			fmt.Printf("Assigned URL to document: %s\n", doc.URL)
		}
		validDocuments = append(validDocuments, doc)
	}

	// Get existing document URLs and content hashes
	existingURLs := make(map[string]bool)
	existingContents := make(map[string]bool)

	for _, doc := range rag.Documents {
		if doc.URL != "" {
			normalizedURL := normalizeURL(doc.URL)
			existingURLs[normalizedURL] = true
			fmt.Printf("Existing URL in RAG: %s\n", normalizedURL)
		}

		if len(doc.Content) > 0 {
			contentHash := getContentHash(doc.Content)
			existingContents[contentHash] = true
		}
	}

	fmt.Printf("Found %d documents on website, checking for new content...\n", len(documents))
	fmt.Printf("There are currently %d existing documents in the RAG\n", len(rag.Documents))

	// Filter documents to keep only the new ones
	var newDocuments []*domain.Document
	for i := range validDocuments {
		doc := validDocuments[i] // doc is already a pointer
		normalizedURL := normalizeURL(doc.URL)
		contentHash := getContentHash(doc.Content)

		// Debug logging
		fmt.Printf("Checking document URL: %s (normalized: %s)\n", doc.URL, normalizedURL)
		fmt.Printf("  URL exists: %v, Content exists: %v\n", existingURLs[normalizedURL], existingContents[contentHash])

		// Check both the URL and the content
		if !existingURLs[normalizedURL] && !existingContents[contentHash] {
			fmt.Printf("New content found: %s\n", doc.URL)
			newDocuments = append(newDocuments, doc)

			// Add to the list to avoid duplicates in this session
			existingURLs[normalizedURL] = true
			existingContents[contentHash] = true
		}
	}

	// If no new documents after filtering, update the timestamp and terminate
	if len(newDocuments) == 0 {
		fmt.Printf("No new content found at '%s' after comparing with existing documents.\n", rag.WatchedURL)
		rag.LastWebWatchAt = time.Now()
		return 0, ww.ragService.UpdateRag(rag)
	}

	fmt.Printf("Found %d new documents to add to the RAG.\n", len(newDocuments))

	// Process the crawled documents directly without going through the file system
	// Create chunker service
	chunkerService := NewChunkerService(ChunkingConfig{
		ChunkSize:    rag.WebWatchOptions.ChunkSize,
		ChunkOverlap: rag.WebWatchOptions.ChunkOverlap,
	})

	var allChunks []*domain.DocumentChunk
	var processedDocs []*domain.Document

	// Process each new document directly
	for i, doc := range newDocuments {
		// Create a unique ID based on the URL
		doc.ID = fmt.Sprintf("web_%d_%s", i, normalizeURL(doc.URL))

		// Ensure the URL is preserved
		if doc.URL == "" {
			doc.URL = rag.WatchedURL + doc.Path
		}

		// Add to the list of processed documents
		processedDocs = append(processedDocs, doc)

		// Chunk the document
		chunks := chunkerService.ChunkDocument(doc)
		// Update the chunk metadata
		for i, chunk := range chunks {
			chunk.ChunkNumber = i
			chunk.TotalChunks = len(chunks)
		}
		allChunks = append(allChunks, chunks...)
	}

	// Generate embeddings for all chunks
	embeddingService := NewEmbeddingService(ww.ragService.GetOllamaClient())
	err = embeddingService.GenerateChunkEmbeddings(allChunks, rag.ModelName)
	if err != nil {
		return 0, fmt.Errorf("error generating embeddings for new documents: %w", err)
	}

	// Add the documents and chunks to the RAG
	for _, doc := range processedDocs {
		rag.AddDocument(doc)
	}

	for _, chunk := range allChunks {
		rag.AddChunk(chunk)
	}

	// Update last watched time
	rag.LastWebWatchAt = time.Now()

	// Save the updated RAG
	err = ww.ragService.UpdateRag(rag)
	if err != nil {
		return 0, fmt.Errorf("error saving updated RAG: %w", err)
	}

	// S'il y a des modifications et qu'un agent ou crew est lié
	if ww.agentService != nil && len(processedDocs) > 0 && (rag.LinkedAgentID != "" || rag.LinkedCrewID != "") {
		// Préparer le dossier de sortie
		outputDir := rag.OutputDirectory
		if outputDir == "" {
			outputDir = "agent-docs"
		}
		
		// Créer le dossier s'il n'existe pas
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			return len(processedDocs), fmt.Errorf("erreur lors de la création du dossier de sortie: %w", err)
		}
		
		// Exécuter l'agent ou le crew lié
		var result string
		if rag.LinkedAgentID != "" {
			// Exécuter l'agent
			agent, err := (*ww.agentService).LoadAgent(rag.LinkedAgentID)
			if err != nil {
				return len(processedDocs), fmt.Errorf("erreur lors du chargement de l'agent: %w", err)
			}
			
			result, err = (*ww.agentService).RunAgent(agent, rag.LinkedPrompt)
			if err != nil {
				return len(processedDocs), fmt.Errorf("erreur lors de l'exécution de l'agent: %w", err)
			}
		} else if rag.LinkedCrewID != "" {
			// Exécuter le crew
			crew, err := (*ww.agentService).LoadCrew(rag.LinkedCrewID)
			if err != nil {
				return len(processedDocs), fmt.Errorf("erreur lors du chargement du crew: %w", err)
			}
			
			results, err := (*ww.agentService).RunCrew(crew, rag.LinkedPrompt)
			if err != nil {
				return len(processedDocs), fmt.Errorf("erreur lors de l'exécution du crew: %w", err)
			}
			
			// Convertir les résultats du crew en un seul résultat
			for agentID, res := range results {
				result += fmt.Sprintf("=== Résultat de %s ===\n\n%s\n\n", agentID, res)
			}
		}
		
		// Sauvegarder les résultats dans des fichiers
		timestamp := time.Now().Format("2006-01-02_15-04-05")
		filename := filepath.Join(outputDir, fmt.Sprintf("%s_%s.md", rag.Name, timestamp))
		
		if err := ioutil.WriteFile(filename, []byte(result), 0644); err != nil {
			return len(processedDocs), fmt.Errorf("erreur lors de la sauvegarde des résultats: %w", err)
		}
		
		fmt.Printf("Résultats de l'agent/crew sauvegardés dans %s\n", filename)
	}

	return len(processedDocs), nil
}

// Function to normalize URLs (remove trailing slashes, etc.)
func normalizeURL(url string) string {
	// Remove the trailing slash if it exists
	url = strings.TrimSuffix(url, "/")
	// Convert to lowercase
	url = strings.ToLower(url)
	// Other normalizations if needed...
	return url
}

// Function to generate a simple hash of the content
func getContentHash(content string) string {
	// Simplify the content for comparison (remove spaces, etc.)
	content = strings.TrimSpace(content)
	simplified := strings.Join(strings.Fields(content), " ")

	// If the content is very short, use the entire content
	if len(simplified) < 200 {
		return simplified
	}

	// For longer content, take the beginning and the end
	// for better identification
	return simplified[:100] + "..." + simplified[len(simplified)-100:]
}

// Add this to the file file_watcher.go or implement it here if it's a new file
func createTempDirForDocuments(documents []*domain.Document) string {
	// Create a temporary directory
	tempDir, err := os.MkdirTemp("", "rlama-crawl-*")
	if err != nil {
		fmt.Printf("Error creating temporary directory: %v\n", err)
		return ""
	}

	fmt.Printf("Created temporary directory for documents: %s\n", tempDir)

	// Save each document as a file in the temporary directory
	for i, doc := range documents {
		// Default to index-based filename
		filename := fmt.Sprintf("page_%d.md", i+1)

		// Try to use Path if available (more likely to exist than URL)
		if doc.Path != "" {
			// Create a safe filename from the Path
			safePath := strings.Map(func(r rune) rune {
				if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
					return r
				}
				return '-'
			}, doc.Path)

			// Trim leading/trailing dashes
			safePath = strings.Trim(safePath, "-")
			if safePath != "" {
				filename = fmt.Sprintf("%s.md", safePath)
			}
		}

		// Full path to the file
		filePath := filepath.Join(tempDir, filename)

		// Write the document content to the file
		err := os.WriteFile(filePath, []byte(doc.Content), 0644)
		if err != nil {
			fmt.Printf("Error writing document to file %s: %v\n", filePath, err)
			continue
		}
	}

	return tempDir
}

func cleanupTempDir(path string) {
	if path != "" {
		if err := os.RemoveAll(path); err != nil {
			fmt.Printf("Warning: Failed to clean up temporary directory %s: %v\n", path, err)
		}
	}
}

// StartWebWatcherDaemon starts a background daemon to watch websites
func (ww *WebWatcher) StartWebWatcherDaemon(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			<-ticker.C
			ww.checkAllRags()
		}
	}()
}

// checkAllRags checks all RAGs with web watching enabled
func (ww *WebWatcher) checkAllRags() {
	// Get all RAGs
	rags, err := ww.ragService.ListAllRags()
	if err != nil {
		fmt.Printf("Error listing RAGs for web watching: %v\n", err)
		return
	}

	now := time.Now()

	for _, ragName := range rags {
		rag, err := ww.ragService.LoadRag(ragName)
		if err != nil {
			fmt.Printf("Error loading RAG %s: %v\n", ragName, err)
			continue
		}

		// Check if web watching is enabled and if interval has passed
		if rag.WebWatchEnabled && rag.WebWatchInterval > 0 {
			intervalDuration := time.Duration(rag.WebWatchInterval) * time.Minute
			if now.Sub(rag.LastWebWatchAt) >= intervalDuration {
				docsAdded, err := ww.CheckAndUpdateRag(rag)
				if err != nil {
					fmt.Printf("Error checking for updates in RAG %s website: %v\n", ragName, err)
				} else if docsAdded > 0 {
					fmt.Printf("Added %d new pages to RAG %s from watched website\n", docsAdded, ragName)
				}
			}
		}
	}
}
