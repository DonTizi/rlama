package repository

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dontizi/rlama/internal/domain"
	"github.com/dontizi/rlama/pkg/vector"
	"github.com/dontizi/rlama/internal/config"
)

// RagRepository manages the persistence of RAG systems
type RagRepository struct {
	basePath string
}

// NewRagRepository creates a new instance of RagRepository
func NewRagRepository() *RagRepository {
	basePath := config.GetDataDir()
	
	// Create the folder if it doesn't exist
	os.MkdirAll(basePath, 0755)
	
	return &RagRepository{
		basePath: basePath,
	}
}

// getRagPath returns the complete path for a given RAG
func (r *RagRepository) getRagPath(ragName string) string {
	return filepath.Join(r.basePath, ragName)
}

// getRagInfoPath returns the path of the RAG information file
func (r *RagRepository) getRagInfoPath(ragName string) string {
	return filepath.Join(r.getRagPath(ragName), "info.json")
}

// getRagVectorStorePath returns the path of the vector storage file
func (r *RagRepository) getRagVectorStorePath(ragName string) string {
	return filepath.Join(r.getRagPath(ragName), "vectors.json")
}

// Exists checks if a RAG exists
func (r *RagRepository) Exists(ragName string) bool {
	_, err := os.Stat(r.getRagInfoPath(ragName))
	return err == nil
}

// Save saves a RAG system
func (r *RagRepository) Save(rag *domain.RagSystem) error {
	ragPath := r.getRagPath(rag.Name)
	
	// Create the folder for this RAG
	err := os.MkdirAll(ragPath, 0755)
	if err != nil {
		return fmt.Errorf("unable to create folder for RAG: %w", err)
	}
	
	// Save RAG information
	ragInfo := *rag // Copy to avoid modifying the original
	
	// Serialize and save the info.json file
	infoJSON, err := json.MarshalIndent(ragInfo, "", "  ")
	if err != nil {
		return fmt.Errorf("unable to serialize RAG information: %w", err)
	}
	
	err = os.WriteFile(r.getRagInfoPath(rag.Name), infoJSON, 0644)
	if err != nil {
		return fmt.Errorf("unable to save RAG information: %w", err)
	}
	
	// Save the Vector Store
	err = rag.HybridStore.Save(r.getRagVectorStorePath(rag.Name))
	if err != nil {
		return fmt.Errorf("unable to save Vector Store: %w", err)
	}
	
	return nil
}

// Load loads a RAG system
func (r *RagRepository) Load(ragName string) (*domain.RagSystem, error) {
	// Check if the RAG exists
	if !r.Exists(ragName) {
		return nil, fmt.Errorf("RAG '%s' does not exist", ragName)
	}
	
	// Load RAG information
	infoBytes, err := os.ReadFile(r.getRagInfoPath(ragName))
	if err != nil {
		return nil, fmt.Errorf("unable to read RAG information: %w", err)
	}
	
	var ragInfo domain.RagSystem
	err = json.Unmarshal(infoBytes, &ragInfo)
	if err != nil {
		return nil, fmt.Errorf("unable to deserialize RAG information: %w", err)
	}
	
	// Create a new Vector Store and load it from the file
	hybridStore, _ := vector.NewEnhancedHybridStore(":memory:", 1536)
	ragInfo.HybridStore = hybridStore
	err = ragInfo.HybridStore.Load(r.getRagVectorStorePath(ragName))
	if err != nil {
		return nil, fmt.Errorf("unable to load Vector Store: %w", err)
	}
	
	return &ragInfo, nil
}

// ListAll returns the list of all available RAG systems
func (r *RagRepository) ListAll() ([]string, error) {
	// Check if the base folder exists
	_, err := os.Stat(r.basePath)
	if os.IsNotExist(err) {
		return []string{}, nil // No RAGs available
	}
	
	// Read the folder contents
	entries, err := os.ReadDir(r.basePath)
	if err != nil {
		return nil, fmt.Errorf("unable to read RAGs folder: %w", err)
	}
	
	var ragNames []string
	for _, entry := range entries {
		if entry.IsDir() {
			// Check if it's a valid RAG folder (contains info.json)
			infoPath := filepath.Join(r.basePath, entry.Name(), "info.json")
			if _, err := os.Stat(infoPath); err == nil {
				ragNames = append(ragNames, entry.Name())
			}
		}
	}
	
	return ragNames, nil
}

// Delete deletes a RAG system
func (r *RagRepository) Delete(ragName string) error {
	// Check if the RAG exists
	if !r.Exists(ragName) {
		return fmt.Errorf("RAG system '%s' does not exist", ragName)
	}
	
	// Delete the complete RAG folder
	ragPath := r.getRagPath(ragName)
	err := os.RemoveAll(ragPath)
	if err != nil {
		return fmt.Errorf("error while deleting RAG system '%s': %w", ragName, err)
	}
	
	return nil
} 