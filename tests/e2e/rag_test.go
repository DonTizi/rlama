package tests

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dontizi/rlama/internal/repository"
	"github.com/dontizi/rlama/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestRagOperations(t *testing.T) {
	// Create a temporary test directory
	tempDir, err := os.MkdirTemp("", "rag-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test files
	docsDir := filepath.Join(tempDir, "docs")
	os.MkdirAll(docsDir, 0755)
	err = os.WriteFile(filepath.Join(docsDir, "test.txt"), []byte("Test content"), 0644)
	assert.NoError(t, err)

	// Create mocks
	mockOllama := new(MockOllamaClient)
	mockEmbedding := new(MockEmbeddingService)

	// Configure mocks
	modelName := "bge-large"
	mockOllama.On("CheckOllamaAndModel", modelName).Return(nil)
	mockOllama.On("GenerateEmbedding", modelName, mock.AnythingOfType("string")).Return([]float32{0.1, 0.2}, nil).Once()
	mockEmbedding.On("GenerateChunkEmbeddings", mock.AnythingOfType("[]*domain.DocumentChunk"), modelName).Return(nil).Once()
	mockEmbedding.On("GenerateQueryEmbedding", mock.AnythingOfType("string"), modelName).Return([]float32{0.1, 0.2}, nil).Once()

	// Create test service
	testService := &TestRagService{
		mockOllama:    mockOllama,
		mockEmbedding: mockEmbedding,
		ragRepository: repository.NewRagRepository(),
	}

	// Run tests
	t.Run("CreateRAG", func(t *testing.T) {
		err := testService.CreateRagWithOptions(modelName, "test-rag", docsDir, service.DocumentLoaderOptions{})
		assert.NoError(t, err)
	})

	// Verify expectations
	mockOllama.AssertExpectations(t)
	mockEmbedding.AssertExpectations(t)
}
