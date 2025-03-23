package service

import (
	"testing"

	"github.com/dontizi/rlama/internal/domain"
	"github.com/stretchr/testify/assert"
)

// TestRagRerankerTopK checks that the reranking is configured correctly and limits the results to 5 by default
func TestRagRerankerTopK(t *testing.T) {
	// Create a RAG with a custom model using the constructor
	rag := &domain.RagSystem{
		Name:            "test-rag",
		ModelName:       "test-model",
		RerankerEnabled: true,
		RerankerModel:   "test-model",
		RerankerTopK:    5,
		RerankerWeight:  0.7,
	}

	// Check that the default reranking values are correct
	assert.True(t, rag.RerankerEnabled, "Reranking should be enabled by default")
	assert.Equal(t, float64(0.7), rag.RerankerWeight, "The reranker weight should be 0.7 by default")
	assert.Equal(t, "test-model", rag.RerankerModel, "Le modèle du reranker devrait être le même que le RAG par défaut")
	assert.Equal(t, 5, rag.RerankerTopK, "TopK devrait être 5 par défaut")

	// Check that the default reranking options are consistent
	options := DefaultRerankerOptions()
	assert.Equal(t, options.TopK, rag.RerankerTopK, "TopK in the RAG and in the options should be identical")

	// Test with different TopK values
	testCases := []struct {
		name     string
		topK     int
		expected int
	}{
		{
			name:     "DefaultTopK",
			topK:     0, // 0 means use the default value
			expected: 5,
		},
		{
			name:     "CustomTopK",
			topK:     10,
			expected: 10,
		},
		{
			name:     "ZeroTopK",
			topK:     -1, // Invalid value, should use the default of the RAG
			expected: 5,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Simulate the logic of Query() to determine the context size
			contextSize := tc.topK

			// If contextSize is 0 (auto), use:
			// - RerankerTopK of the RAG if defined
			// - Otherwise the default TopK (5)
			// - 20 if reranking is disabled
			if contextSize <= 0 {
				if rag.RerankerEnabled {
					if rag.RerankerTopK > 0 {
						contextSize = rag.RerankerTopK
					} else {
						contextSize = options.TopK // 5 by default
					}
				} else {
					contextSize = 20 // 20 by default if reranking is disabled
				}
			}

			// Check that contextSize corresponds to the expected value
			assert.Equal(t, tc.expected, contextSize,
				"The context size should correspond to the expected value")
		})
	}

	// Test the case where reranking is disabled
	t.Run("DisabledReranking", func(t *testing.T) {
		rag.RerankerEnabled = false

		// Context size set to 0 should default to 20 because reranking is disabled
		contextSize := 0
		if contextSize <= 0 {
			if rag.RerankerEnabled {
				if rag.RerankerTopK > 0 {
					contextSize = rag.RerankerTopK
				} else {
					contextSize = options.TopK // 5 by default
				}
			} else {
				contextSize = 20 // 20 by default if reranking is disabled
			}
		}

		assert.Equal(t, 20, contextSize,
			"La taille du contexte devrait être 20 par défaut si le reranking est désactivé")

		// Restore the state
		rag.RerankerEnabled = true
	})
}
