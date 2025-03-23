package tests

import (
	"testing"

	"github.com/dontizi/rlama/internal/repository"
	"github.com/dontizi/rlama/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestWizard(t *testing.T) {
	// Create and configure mocks
	mockOllama := new(MockOllamaClient)
	mockEmbedding := new(MockEmbeddingService)

	modelName := "test-model"
	ragName := "test-rag"

	// Configure mock expectations - chaque méthode n'est appelée qu'une fois
	mockOllama.On("CheckOllamaAndModel", modelName).Return(nil)
	mockOllama.On("GenerateEmbedding", modelName, mock.AnythingOfType("string")).Return([]float32{0.1, 0.2}, nil).Once()
	mockEmbedding.On("GenerateChunkEmbeddings", mock.AnythingOfType("[]*domain.DocumentChunk"), modelName).Return(nil).Once()
	mockEmbedding.On("GenerateQueryEmbedding", mock.AnythingOfType("string"), modelName).Return([]float32{0.1, 0.2}, nil).Once()

	// Create test service
	ragService := &TestRagService{
		mockOllama:    mockOllama,
		mockEmbedding: mockEmbedding,
		ragRepository: repository.NewRagRepository(),
	}

	// Test cases
	t.Run("CreateRAG", func(t *testing.T) {
		err := ragService.CreateRagWithOptions(modelName, ragName, "testdata", service.DocumentLoaderOptions{})
		assert.NoError(t, err)
	})

	t.Run("LoadRAG", func(t *testing.T) {
		rag, err := ragService.LoadRag(ragName)
		assert.NoError(t, err)
		assert.NotNil(t, rag)
	})

	t.Run("DeleteRAG", func(t *testing.T) {
		err := ragService.DeleteRag(ragName)
		assert.NoError(t, err)
	})

	// Verify all mock expectations were met
	mockOllama.AssertExpectations(t)
	mockEmbedding.AssertExpectations(t)
}
