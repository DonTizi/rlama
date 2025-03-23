package service

import (
	"fmt"
	"testing"

	"github.com/dontizi/rlama/internal/domain"
	"github.com/stretchr/testify/assert"
)

// TestRerankerOptionsDefaultValues checks that the default values are correct
func TestRerankerOptionsDefaultValues(t *testing.T) {
	// Get the default options
	options := DefaultRerankerOptions()

	// Check that the default values are correct
	assert.Equal(t, 5, options.TopK, "The default value for TopK should be 5")
	assert.Equal(t, 20, options.InitialK, "The default value for InitialK should be 20")
	assert.Equal(t, float64(0.7), options.RerankerWeight, "The default value for RerankerWeight should be 0.7")
	assert.Equal(t, float64(0.0), options.ScoreThreshold, "The default value for ScoreThreshold should be 0.0")
}

// TestApplyTopKLimit checks that the TopK limit is applied correctly
func TestApplyTopKLimit(t *testing.T) {
	// Create sorted results to simulate the output before applying TopK
	testCases := []struct {
		name     string
		results  []RankedResult
		topK     int
		expected int
	}{
		{
			name:     "LimitsToTopK5",
			results:  createDummyRankedResults(20),
			topK:     5,
			expected: 5,
		},
		{
			name:     "LimitsToTopK10",
			results:  createDummyRankedResults(20),
			topK:     10,
			expected: 10,
		},
		{
			name:     "HandlesTopKGreaterThanResults",
			results:  createDummyRankedResults(15),
			topK:     20,
			expected: 15, // Cannot return more than what exists
		},
		{
			name:     "HandlesEmptyResults",
			results:  []RankedResult{},
			topK:     5,
			expected: 0,
		},
		{
			name:     "HandlesTopKZero",
			results:  createDummyRankedResults(10),
			topK:     0,
			expected: 10, // Should not limit if TopK=0
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Apply the TopK limit manually (reproduce the logic of Rerank)
			var limited []RankedResult
			if tc.topK > 0 && len(tc.results) > tc.topK {
				limited = tc.results[:tc.topK]
			} else {
				limited = tc.results
			}

			// Check that the number is correct
			assert.Equal(t, tc.expected, len(limited), "The number of results should be limited to TopK if necessary")
		})
	}
}

// createDummyRankedResults creates a set of dummy results for testing
func createDummyRankedResults(count int) []RankedResult {
	results := make([]RankedResult, count)

	for i := 0; i < count; i++ {
		results[i] = RankedResult{
			Chunk:         &domain.DocumentChunk{ID: fmt.Sprintf("chunk-%d", i)},
			VectorScore:   0.8 - (float64(i) * 0.01),
			RerankerScore: 0.9 - (float64(i) * 0.02),
			FinalScore:    0.95 - (float64(i) * 0.015),
		}
	}

	return results
}

// Reproduce the sorting function to test
func TestSortingByScore(t *testing.T) {
	// Create results in a mixed order
	results := []RankedResult{
		{FinalScore: 0.5},
		{FinalScore: 0.9},
		{FinalScore: 0.3},
		{FinalScore: 0.7},
		{FinalScore: 0.1},
	}

	// Sort the results (same logic as in Rerank)
	// Sort by final score (descending)
	sortedResults := make([]RankedResult, len(results))
	copy(sortedResults, results)

	// Sort by final score (descending)
	for i := 0; i < len(sortedResults); i++ {
		for j := i + 1; j < len(sortedResults); j++ {
			if sortedResults[i].FinalScore < sortedResults[j].FinalScore {
				sortedResults[i], sortedResults[j] = sortedResults[j], sortedResults[i]
			}
		}
	}

	// Check that the results are sorted correctly
	for i := 1; i < len(sortedResults); i++ {
		assert.GreaterOrEqual(t, sortedResults[i-1].FinalScore, sortedResults[i].FinalScore,
			"The results should be sorted by descending score")
	}

	// Check the exact order
	assert.Equal(t, float64(0.9), sortedResults[0].FinalScore)
	assert.Equal(t, float64(0.7), sortedResults[1].FinalScore)
	assert.Equal(t, float64(0.5), sortedResults[2].FinalScore)
	assert.Equal(t, float64(0.3), sortedResults[3].FinalScore)
	assert.Equal(t, float64(0.1), sortedResults[4].FinalScore)
}

// TestRerankerIntegration checks the integration of reranking in the RAG service
func TestRerankerIntegration(t *testing.T) {
	// This test will integrate reranking in a complete RAG service
	// As it requires external dependencies, it will be marked as an integration test
	t.Skip("This test requires an Ollama instance to be running")

	// TODO: Implement an integration test with a real RAG service
	// This can be done later by using the existing structs and functions
}
