package service

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/dontizi/rlama/internal/domain"
)

// ChunkingEvaluationMetrics contains evaluation metrics for a chunking strategy
type ChunkingEvaluationMetrics struct {
	// Basic metrics
	TotalChunks      int     // Total number of chunks produced
	AverageChunkSize float64 // Average chunk size in characters
	SizeStdDeviation float64 // Standard deviation of chunk sizes
	MaxChunkSize     int     // Size of the largest chunk
	MinChunkSize     int     // Size of the smallest chunk

	// Coherence metrics
	ChunksWithSplitSentences  int     // Number of chunks that split sentences
	ChunksWithSplitParagraphs int     // Number of chunks that split paragraphs
	SemanticCoherenceScore    float64 // Estimated semantic coherence score

	// Performance metrics
	ProcessingTimeMs int64 // Processing time in ms

	// Distribution metrics
	ContentCoverage float64 // % of original content covered by chunks
	RedundancyRate  float64 // Redundancy rate due to overlap

	// Strategy info
	Strategy     string // Strategy used
	ChunkSize    int    // Configured chunk size
	ChunkOverlap int    // Configured overlap
}

// ChunkingEvaluator evaluates different chunking strategies
type ChunkingEvaluator struct {
	chunkerService *ChunkerService
	// References for semantic evaluation
	sentenceEndings  []string
	paragraphMarkers []string
}

// NewChunkingEvaluator creates a new chunking evaluator
func NewChunkingEvaluator(chunkerService *ChunkerService) *ChunkingEvaluator {
	return &ChunkingEvaluator{
		chunkerService:   chunkerService,
		sentenceEndings:  []string{".", "!", "?", "\n"},
		paragraphMarkers: []string{"\n\n", "\r\n\r\n"},
	}
}

// EvaluateChunkingStrategy evaluates a chunking strategy with the given parameters
func (ce *ChunkingEvaluator) EvaluateChunkingStrategy(doc *domain.Document, config ChunkingConfig) ChunkingEvaluationMetrics {
	startTime := time.Now()

	// Create a temporary chunker service with the configuration to test
	tempChunker := NewChunkerService(config)

	// Generate chunks with the strategy to evaluate
	chunks := tempChunker.ChunkDocument(doc)

	// Calculate basic metrics
	metrics := ChunkingEvaluationMetrics{
		TotalChunks:      len(chunks),
		Strategy:         config.ChunkingStrategy,
		ChunkSize:        config.ChunkSize,
		ChunkOverlap:     config.ChunkOverlap,
		ProcessingTimeMs: time.Since(startTime).Milliseconds(),
	}

	if len(chunks) == 0 {
		return metrics
	}

	// Calculate chunk sizes
	totalSize := 0
	metrics.MaxChunkSize = 0
	metrics.MinChunkSize = math.MaxInt32

	sizes := make([]int, len(chunks))

	for i, chunk := range chunks {
		size := len(chunk.Content)
		sizes[i] = size
		totalSize += size

		if size > metrics.MaxChunkSize {
			metrics.MaxChunkSize = size
		}
		if size < metrics.MinChunkSize {
			metrics.MinChunkSize = size
		}
	}

	// Mean and standard deviation
	metrics.AverageChunkSize = float64(totalSize) / float64(len(chunks))

	// Calculate standard deviation
	sumSquaredDiff := 0.0
	for _, size := range sizes {
		diff := float64(size) - metrics.AverageChunkSize
		sumSquaredDiff += diff * diff
	}
	metrics.SizeStdDeviation = math.Sqrt(sumSquaredDiff / float64(len(chunks)))

	// Evaluate coverage and redundancy
	docLength := len(doc.Content)
	coveredChars := 0

	// Track covered characters
	covered := make([]bool, docLength)
	for _, chunk := range chunks {
		for i := chunk.StartPos; i < chunk.EndPos && i < docLength; i++ {
			if !covered[i] {
				covered[i] = true
				coveredChars++
			}
		}
	}

	metrics.ContentCoverage = float64(coveredChars) / float64(docLength)

	// Calculate redundancy rate
	totalChunkChars := totalSize
	nonRedundantChars := coveredChars
	if nonRedundantChars > 0 {
		metrics.RedundancyRate = float64(totalChunkChars-nonRedundantChars) / float64(totalChunkChars)
	}

	// Check for split sentences and paragraphs
	metrics.ChunksWithSplitSentences = ce.countChunksWithSplitSentences(chunks, doc.Content)
	metrics.ChunksWithSplitParagraphs = ce.countChunksWithSplitParagraphs(chunks, doc.Content)

	// Calculate an approximate semantic coherence score based on the metrics above
	// Higher score = better estimated coherence
	metrics.SemanticCoherenceScore = ce.calculateSemanticCoherenceScore(metrics, len(chunks))

	return metrics
}

// countChunksWithSplitSentences counts chunks that split a sentence
func (ce *ChunkingEvaluator) countChunksWithSplitSentences(chunks []*domain.DocumentChunk, originalContent string) int {
	count := 0
	for _, chunk := range chunks {
		// Check the beginning of the chunk
		if chunk.StartPos > 0 {
			// Check if the previous character is a sentence ending marker
			// Make sure that StartPos-1 is within the valid range
			if chunk.StartPos-1 >= 0 && chunk.StartPos-1 < len(originalContent) {
				prevChar := originalContent[chunk.StartPos-1]
				if !strings.ContainsRune(".!?\n", rune(prevChar)) {
					// Check if we're in the middle of a sentence
					count++
					continue
				}
			}
		}

		// Check the end of the chunk
		if chunk.EndPos < len(originalContent) && len(chunk.Content) > 0 {
			lastChar := chunk.Content[len(chunk.Content)-1]
			nextChar := originalContent[chunk.EndPos]

			// If the last character is not a sentence ending and the next is not a sentence beginning
			if !strings.ContainsRune(".!?\n", rune(lastChar)) && nextChar != ' ' && nextChar != '\n' {
				count++
			}
		}
	}
	return count
}

// countChunksWithSplitParagraphs counts chunks that split a paragraph
func (ce *ChunkingEvaluator) countChunksWithSplitParagraphs(chunks []*domain.DocumentChunk, originalContent string) int {
	count := 0
	for _, chunk := range chunks {
		// For simplicity, we consider a paragraph is split if:
		// 1. The chunk doesn't start after a paragraph marker
		// 2. The chunk doesn't end before a paragraph marker

		startsWithParagraph := false
		endsWithParagraph := false

		// Check the beginning
		if chunk.StartPos == 0 {
			startsWithParagraph = true
		} else {
			// Check if there's a paragraph marker before
			for _, marker := range ce.paragraphMarkers {
				if chunk.StartPos >= len(marker) &&
					originalContent[chunk.StartPos-len(marker):chunk.StartPos] == marker {
					startsWithParagraph = true
					break
				}
			}
		}

		// Check the end
		if chunk.EndPos == len(originalContent) {
			endsWithParagraph = true
		} else {
			// Check if there's a paragraph marker after
			for _, marker := range ce.paragraphMarkers {
				if chunk.EndPos+len(marker) <= len(originalContent) &&
					originalContent[chunk.EndPos:chunk.EndPos+len(marker)] == marker {
					endsWithParagraph = true
					break
				}
			}
		}

		if !startsWithParagraph || !endsWithParagraph {
			count++
		}
	}
	return count
}

// calculateSemanticCoherenceScore calculates an estimated semantic coherence score
func (ce *ChunkingEvaluator) calculateSemanticCoherenceScore(metrics ChunkingEvaluationMetrics, totalChunks int) float64 {
	if totalChunks == 0 {
		return 0
	}

	// Factors penalizing semantic coherence
	sentenceSplitPenalty := float64(metrics.ChunksWithSplitSentences) / float64(totalChunks)
	paragraphSplitPenalty := float64(metrics.ChunksWithSplitParagraphs) / float64(totalChunks)

	// Size factor: penalize highly variable sizes
	sizeConsistencyFactor := 0.0
	if metrics.AverageChunkSize > 0 {
		sizeConsistencyFactor = metrics.SizeStdDeviation / metrics.AverageChunkSize
	}

	// Calculate score (inverted so that higher is better)
	// Lower values = fewer split sentences/paragraphs and more consistency
	coherenceScore := 1.0 - (0.4*sentenceSplitPenalty + 0.4*paragraphSplitPenalty + 0.2*sizeConsistencyFactor)

	// Ensure the score is between 0 and 1
	return math.Max(0, math.Min(1, coherenceScore))
}

// CompareChunkingStrategies runs a comparative evaluation of different chunking
// configurations and returns the results sorted by relevance for this document
func (ce *ChunkingEvaluator) CompareChunkingStrategies(doc *domain.Document) []ChunkingEvaluationMetrics {
	var results []ChunkingEvaluationMetrics

	// Define the different strategies and configurations to test
	strategies := []string{"fixed", "semantic", "hybrid", "hierarchical"}
	chunkSizes := []int{500, 1000, 1500, 2000}
	overlapRates := []float64{0.05, 0.1, 0.2} // as percentage of chunk size

	fmt.Printf("Evaluating %d chunking strategies for document '%s' (%d characters)...\n",
		len(strategies)*len(chunkSizes)*len(overlapRates), doc.Name, len(doc.Content))

	for _, strategy := range strategies {
		for _, chunkSize := range chunkSizes {
			for _, overlapRate := range overlapRates {
				// Calculate overlap in characters
				overlap := int(float64(chunkSize) * overlapRate)

				config := ChunkingConfig{
					ChunkSize:        chunkSize,
					ChunkOverlap:     overlap,
					ChunkingStrategy: strategy,
					IncludeMetadata:  true,
				}

				// Evaluate this configuration
				metrics := ce.EvaluateChunkingStrategy(doc, config)
				results = append(results, metrics)

				fmt.Printf("  Strategy: %-12s | Size: %4d | Overlap: %3d | Score: %.4f | Chunks: %3d\n",
					strategy, chunkSize, overlap, metrics.SemanticCoherenceScore, metrics.TotalChunks)
			}
		}
	}

	// Sort results by semantic coherence score (from highest to lowest)
	// Use a simple bubble sort for readability
	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].SemanticCoherenceScore > results[i].SemanticCoherenceScore {
				results[i], results[j] = results[j], results[i]
			}
		}
	}

	return results
}

// GetOptimalChunkingConfig returns the optimal chunking configuration for the given document
func (ce *ChunkingEvaluator) GetOptimalChunkingConfig(doc *domain.Document) ChunkingConfig {
	results := ce.CompareChunkingStrategies(doc)

	if len(results) == 0 {
		// If no results, return default configuration
		return DefaultChunkingConfig()
	}

	// Take the best configuration (first after sorting)
	bestResult := results[0]

	return ChunkingConfig{
		ChunkSize:        bestResult.ChunkSize,
		ChunkOverlap:     bestResult.ChunkOverlap,
		ChunkingStrategy: bestResult.Strategy,
		IncludeMetadata:  true,
	}
}

// PrintEvaluationResults displays evaluation results in a readable format
func (ce *ChunkingEvaluator) PrintEvaluationResults(metrics ChunkingEvaluationMetrics) {
	fmt.Println("\n=== Chunking Strategy Evaluation Results ===")
	fmt.Printf("Strategy: %s\n", metrics.Strategy)
	fmt.Printf("Configuration: Size=%d, Overlap=%d\n", metrics.ChunkSize, metrics.ChunkOverlap)
	fmt.Println("\n--- Basic Metrics ---")
	fmt.Printf("Number of chunks: %d\n", metrics.TotalChunks)
	fmt.Printf("Average chunk size: %.2f characters\n", metrics.AverageChunkSize)
	fmt.Printf("Standard deviation: %.2f\n", metrics.SizeStdDeviation)
	fmt.Printf("Min/max size: %d/%d characters\n", metrics.MinChunkSize, metrics.MaxChunkSize)

	fmt.Println("\n--- Coherence Metrics ---")
	fmt.Printf("Split sentences: %d chunks (%.1f%%)\n",
		metrics.ChunksWithSplitSentences,
		float64(metrics.ChunksWithSplitSentences)/float64(metrics.TotalChunks)*100)
	fmt.Printf("Split paragraphs: %d chunks (%.1f%%)\n",
		metrics.ChunksWithSplitParagraphs,
		float64(metrics.ChunksWithSplitParagraphs)/float64(metrics.TotalChunks)*100)
	fmt.Printf("Semantic coherence score: %.4f\n", metrics.SemanticCoherenceScore)

	fmt.Println("\n--- Coverage Metrics ---")
	fmt.Printf("Content coverage: %.1f%%\n", metrics.ContentCoverage*100)
	fmt.Printf("Redundancy rate: %.1f%%\n", metrics.RedundancyRate*100)

	fmt.Println("\n--- Performance ---")
	fmt.Printf("Processing time: %d ms\n", metrics.ProcessingTimeMs)
	fmt.Println("=====================================================")
}
