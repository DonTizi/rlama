package service

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
	"io/ioutil"
	"encoding/json"

	"github.com/dontizi/rlama/internal/domain"
)

// FileWatcher is responsible for watching directories for file changes
type FileWatcher struct {
	ragService RagService
}

// NewFileWatcher creates a new file watcher service
func NewFileWatcher(ragService RagService) *FileWatcher {
	return &FileWatcher{
		ragService: ragService,
	}
}

// CheckAndUpdateRag checks for new files in the watched directory and updates the RAG
func (fw *FileWatcher) CheckAndUpdateRag(rag *domain.RagSystem) (int, error) {
	if !rag.WatchEnabled || rag.WatchedDir == "" {
		return 0, nil // Watching not enabled
	}

	// Check if the directory exists
	dirInfo, err := os.Stat(rag.WatchedDir)
	if os.IsNotExist(err) {
		return 0, fmt.Errorf("watched directory '%s' does not exist", rag.WatchedDir)
	} else if err != nil {
		return 0, fmt.Errorf("error accessing watched directory: %w", err)
	}

	if !dirInfo.IsDir() {
		return 0, fmt.Errorf("'%s' is not a directory", rag.WatchedDir)
	}

	// Get the last modified time of the directory
	lastModified := getLastModifiedTime(rag.WatchedDir)
	
	// If the directory hasn't been modified since last check, no need to proceed
	if !lastModified.After(rag.LastWatchedAt) && !rag.LastWatchedAt.IsZero() {
		return 0, nil
	}

	// Convert watch options to document loader options
	loaderOptions := DocumentLoaderOptions{
		ExcludeDirs:  rag.WatchOptions.ExcludeDirs,
		ExcludeExts:  rag.WatchOptions.ExcludeExts,
		ProcessExts:  rag.WatchOptions.ProcessExts,
		ChunkSize:    rag.WatchOptions.ChunkSize,
		ChunkOverlap: rag.WatchOptions.ChunkOverlap,
	}

	// Get existing document paths to avoid re-processing
	existingPaths := make(map[string]bool)
	for _, doc := range rag.Documents {
		existingPaths[doc.Path] = true
	}

	// Create a document loader
	docLoader := NewDocumentLoader()
	
	// Load all documents from the directory
	allDocs, err := docLoader.LoadDocumentsFromFolderWithOptions(rag.WatchedDir, loaderOptions)
	if err != nil {
		return 0, fmt.Errorf("error loading documents from watched directory: %w", err)
	}

	// Filter out existing documents
	var newDocs []*domain.Document
	for _, doc := range allDocs {
		if !existingPaths[doc.Path] {
			newDocs = append(newDocs, doc)
		}
	}

	if len(newDocs) == 0 {
		// Update last watched time even if no new documents
		rag.LastWatchedAt = time.Now()
		err = fw.ragService.UpdateRag(rag)
		return 0, err
	}

	// Create chunker service with options from the RAG
	chunkerService := NewChunkerService(ChunkingConfig{
		ChunkSize:    loaderOptions.ChunkSize,
		ChunkOverlap: loaderOptions.ChunkOverlap,
	})

	// Process each new document - chunk and prepare for embeddings
	var allChunks []*domain.DocumentChunk
	for _, doc := range newDocs {
		// Chunk the document
		chunks := chunkerService.ChunkDocument(doc)
		
		// Update total chunks in metadata
		for i, chunk := range chunks {
			chunk.ChunkNumber = i
			chunk.TotalChunks = len(chunks)
		}
		
		allChunks = append(allChunks, chunks...)
	}

	// Generate embeddings for all chunks
	embeddingService := NewEmbeddingService(fw.ragService.GetOllamaClient())
	err = embeddingService.GenerateChunkEmbeddings(allChunks, rag.ModelName)
	if err != nil {
		return 0, fmt.Errorf("error generating embeddings for new documents: %w", err)
	}

	// Add documents and chunks to the RAG
	for _, doc := range newDocs {
		rag.AddDocument(doc)
	}
	
	for _, chunk := range allChunks {
		rag.AddChunk(chunk)
	}

	// Update last watched time
	rag.LastWatchedAt = time.Now()
	
	// Save the updated RAG
	err = fw.ragService.UpdateRag(rag)
	if err != nil {
		return 0, fmt.Errorf("error saving updated RAG: %w", err)
	}

	return len(newDocs), nil
}

// getLastModifiedTime gets the latest modification time in a directory
func getLastModifiedTime(dirPath string) time.Time {
	var lastModTime time.Time

	// Walk through the directory and find the latest modification time
	filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}
		
		if info.ModTime().After(lastModTime) {
			lastModTime = info.ModTime()
		}
		
		return nil
	})

	return lastModTime
}

// StartWatcherDaemon starts a background daemon to watch directories
// for all RAGs that have watching enabled with intervals
func (fw *FileWatcher) StartWatcherDaemon(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			<-ticker.C
			fw.checkAllRags()
		}
	}()
}

// checkAllRags checks all RAGs with watching enabled
func (fw *FileWatcher) checkAllRags() {
	// Get all RAGs
	rags, err := fw.ragService.ListAllRags()
	if err != nil {
		fmt.Printf("Error listing RAGs for file watching: %v\n", err)
		return
	}

	now := time.Now()
	
	for _, ragName := range rags {
		rag, err := fw.ragService.LoadRag(ragName)
		if err != nil {
			fmt.Printf("Error loading RAG %s: %v\n", ragName, err)
			continue
		}

		// Check if watching is enabled and if interval has passed
		if rag.WatchEnabled && rag.WatchInterval > 0 {
			intervalDuration := time.Duration(rag.WatchInterval) * time.Minute
			if now.Sub(rag.LastWatchedAt) >= intervalDuration {
				docsAdded, err := fw.CheckAndUpdateRag(rag)
				if err != nil {
					fmt.Printf("Error checking for updates in RAG %s: %v\n", ragName, err)
				} else if docsAdded > 0 {
					fmt.Printf("Added %d new documents to RAG %s from watched directory\n", docsAdded, ragName)
				}
			}
		}
	}
}

// GetLinkedCrews retrieves the crews linked to a specific RAG
func (fw *FileWatcher) GetLinkedCrews(ragName string) ([]LinkedCrew, error) {
	// Charger le RAG pour vérifier s'il a des crews liés
	rag, err := fw.ragService.LoadRag(ragName)
	if err != nil {
		return nil, fmt.Errorf("erreur lors du chargement du RAG: %w", err)
	}
	
	// Vérifier si le RAG a un crew lié
	if rag.LinkedCrewID == "" {
		return nil, nil // Pas de crews liés
	}
	
	fmt.Printf("Recherche du crew avec ID: %s\n", rag.LinkedCrewID)
	
	// Chercher d'abord le fichier par ID du crew
	basePath := filepath.Join(os.Getenv("HOME"), ".rlama", "agents")
	
	// Méthode 1: Chercher dans le dossier agents par ID
	crewFilePath := filepath.Join(basePath, rag.LinkedCrewID)
	
	if _, err := os.Stat(crewFilePath); os.IsNotExist(err) {
		// Méthode 2: Essayer avec extension .json
		crewFilePath = filepath.Join(basePath, rag.LinkedCrewID+".json")
		
		if _, err := os.Stat(crewFilePath); os.IsNotExist(err) {
			// Méthode 3: Chercher dans le sous-dossier crews par ID
			crewFilePath = filepath.Join(basePath, "crews", rag.LinkedCrewID)
			
			if _, err := os.Stat(crewFilePath); os.IsNotExist(err) {
				// Méthode 4: Essayer avec extension .json dans le sous-dossier
				crewFilePath = filepath.Join(basePath, "crews", rag.LinkedCrewID+".json")
				
				if _, err := os.Stat(crewFilePath); os.IsNotExist(err) {
					// Dernière tentative: parcourir le dossier pour trouver le fichier
					fmt.Printf("Tentative de recherche manuelle des fichiers crew...\n")
					files, _ := ioutil.ReadDir(basePath)
					
					fmt.Printf("Fichiers trouvés dans %s:\n", basePath)
					for _, file := range files {
						fmt.Printf("- %s\n", file.Name())
					}
					
					return nil, nil // Le fichier n'existe pas
				}
			}
		}
	}
	
	fmt.Printf("Fichier crew trouvé: %s\n", crewFilePath)
	
	// Lire le fichier crew
	data, err := ioutil.ReadFile(crewFilePath)
	if err != nil {
		return nil, fmt.Errorf("erreur lors de la lecture du fichier crew: %w", err)
	}
	
	// Désérialiser le crew
	crew := &domain.Crew{}
	if err := json.Unmarshal(data, crew); err != nil {
		return nil, fmt.Errorf("erreur lors de la désérialisation du crew: %w", err)
	}
	
	fmt.Printf("Crew '%s' trouvé avec ID: %s\n", crew.Name, crew.ID)
	
	// Créer la structure LinkedCrew
	return []LinkedCrew{
		{
			CrewID:             crew.ID,
			InstructionTemplate: rag.LinkedPrompt, // Utiliser le prompt stocké dans le RAG
		},
	}, nil
}

// LinkedCrew represents a crew linked to a RAG
type LinkedCrew struct {
	CrewID             string
	InstructionTemplate string
} 