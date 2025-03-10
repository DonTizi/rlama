package domain

import (
	"time"

	"github.com/dontizi/rlama/pkg/vector"
)

// RagSystem représente un système RAG complet
type RagSystem struct {
	Name        string        `json:"name"`
	ModelName   string        `json:"model_name"`
	CreatedAt   time.Time     `json:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at"`
	Description string        `json:"description"`
	VectorStore *vector.Store
	Documents   []*Document     `json:"documents"`
	Chunks      []*DocumentChunk `json:"chunks"`
	MaxRetrievedChunks int // If > 0, limits number of chunks retrieved
}

// NewRagSystem crée une nouvelle instance de RagSystem
func NewRagSystem(name, modelName string) *RagSystem {
	now := time.Now()
	return &RagSystem{
		Name:        name,
		ModelName:   modelName,
		CreatedAt:   now,
		UpdatedAt:   now,
		VectorStore: vector.NewStore(),
		Documents:   []*Document{},
		Chunks:      []*DocumentChunk{},
	}
}

// AddDocument ajoute un document au système RAG
func (r *RagSystem) AddDocument(doc *Document) {
	r.Documents = append(r.Documents, doc)
	if doc.Embedding != nil {
		r.VectorStore.Add(doc.ID, doc.Embedding)
	}
	r.UpdatedAt = time.Now()
}

// GetDocumentByID récupère un document par son ID
func (r *RagSystem) GetDocumentByID(id string) *Document {
	for _, doc := range r.Documents {
		if doc.ID == id {
			return doc
		}
	}
	return nil
}

// RemoveDocument removes a document from the RAG system by its ID
func (r *RagSystem) RemoveDocument(id string) bool {
	// Find the document index
	var index = -1
	for i, doc := range r.Documents {
		if doc.ID == id {
			index = i
			break
		}
	}

	if index == -1 {
		return false
	}

	// Remove from the Documents slice
	r.Documents = append(r.Documents[:index], r.Documents[index+1:]...)
	
	// Remove from the VectorStore
	r.VectorStore.Remove(id)
	
	r.UpdatedAt = time.Now()
	return true
}

// AddChunk adds a chunk to the RAG system
func (r *RagSystem) AddChunk(chunk *DocumentChunk) {
	r.Chunks = append(r.Chunks, chunk)
	if chunk.Embedding != nil {
		r.VectorStore.Add(chunk.ID, chunk.Embedding)
	}
	r.UpdatedAt = time.Now()
}

// GetChunkByID retrieves a chunk by its ID
func (r *RagSystem) GetChunkByID(id string) *DocumentChunk {
	for _, chunk := range r.Chunks {
		if chunk.ID == id {
			return chunk
		}
	}
	return nil
} 