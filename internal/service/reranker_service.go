package service

import (
	"fmt"
	"sort"
	"strings"

	"github.com/dontizi/rlama/internal/client"
	"github.com/dontizi/rlama/internal/domain"
	"github.com/dontizi/rlama/pkg/vector"
)

// RerankerOptions defines configuration options for the reranking process
type RerankerOptions struct {
	// TopK is the number of documents to return after reranking
	TopK int

	// InitialK is the number of documents to retrieve from the initial search
	// before reranking (should be >= TopK)
	InitialK int

	// RerankerModel is the model to use for reranking
	// If empty, will default to the same model used for embedding
	RerankerModel string

	// ScoreThreshold is the minimum relevance score (0-1) for a document to be included
	// Documents with scores below this threshold will be filtered out
	ScoreThreshold float64

	// RerankerWeight defines the weight of the reranker score vs vector similarity
	// 0.0 = use only vector similarity, 1.0 = use only reranker scores
	RerankerWeight float64

	// AdaptiveFiltering when true, uses content relevance to select chunks
	// rather than a fixed top-k approach
	AdaptiveFiltering bool
}

// DefaultRerankerOptions returns the default options for reranking
func DefaultRerankerOptions() RerankerOptions {
	return RerankerOptions{
		TopK:              5,
		InitialK:          20,
		RerankerModel:     "BAAI/bge-reranker-v2-m3",
		ScoreThreshold:    0.0,
		RerankerWeight:    0.7, // 70% reranker score, 30% vector similarity
		AdaptiveFiltering: false,
	}
}

// RerankerService handles document reranking using cross-encoder models
type RerankerService struct {
	ollamaClient      *client.OllamaClient
	bgeRerankerClient *client.BGERerankerClient
}

// NewRerankerService creates a new instance of RerankerService
func NewRerankerService(ollamaClient *client.OllamaClient) *RerankerService {
	if ollamaClient == nil {
		ollamaClient = client.NewDefaultOllamaClient()
	}

	// Create the BGE reranker client with the default model
	bgeRerankerClient := client.NewBGERerankerClient("BAAI/bge-reranker-v2-m3")

	return &RerankerService{
		ollamaClient:      ollamaClient,
		bgeRerankerClient: bgeRerankerClient,
	}
}

// RankedResult represents a document with its relevance score after reranking
type RankedResult struct {
	Chunk         *domain.DocumentChunk
	VectorScore   float64
	RerankerScore float64
	FinalScore    float64
}

// Rerank takes initial retrieval results and reruns them through a cross-encoder for more accurate ranking
func (rs *RerankerService) Rerank(
	query string,
	rag *domain.RagSystem,
	initialResults []vector.SearchResult,
	options RerankerOptions,
) ([]RankedResult, error) {
	// Create an empty result if no documents were found
	if len(initialResults) == 0 {
		return []RankedResult{}, nil
	}

	// Always use BGE Reranker if available
	if rs.bgeRerankerClient != nil {
		// Use the BGE model configured in the client
		fmt.Printf("Using reranker model: %s (BGE Reranker)\n", rs.bgeRerankerClient.GetModelName())

		// Code to perform reranking with BGE
		pairs := make([][]string, 0, len(initialResults))
		resultMap := make(map[int]*domain.DocumentChunk)

		// Prepare the pairs for batch processing
		for i, result := range initialResults {
			chunk := rag.GetChunkByID(result.ID)
			if chunk == nil {
				continue
			}

			pairs = append(pairs, []string{query, chunk.Content})
			resultMap[i] = chunk
		}

		// Get scores
		scores, err := rs.bgeRerankerClient.ComputeScores(pairs, true)
		if err != nil {
			// In case of failure, return to standard model
			fmt.Printf("⚠️ BGE Reranker failure: %v. Falling back to standard model.\n", err)
		} else {
			// Process scores and return results
			rankedResults := make([]RankedResult, 0, len(scores))
			for i, score := range scores {
				if i >= len(initialResults) {
					break
				}

				chunk := resultMap[i]
				if chunk == nil {
					continue
				}

				vectorScore := initialResults[i].Score

				// Calculate final score as weighted combination of vector and reranker scores
				finalScore := (options.RerankerWeight * score) + ((1 - options.RerankerWeight) * vectorScore)

				// Add to results if above threshold
				if finalScore >= options.ScoreThreshold {
					rankedResults = append(rankedResults, RankedResult{
						Chunk:         chunk,
						VectorScore:   vectorScore,
						RerankerScore: score,
						FinalScore:    finalScore,
					})
				}
			}

			// Sort by final score (descending)
			sort.Slice(rankedResults, func(i, j int) bool {
				return rankedResults[i].FinalScore > rankedResults[j].FinalScore
			})

			// Only apply Top-K limit if we're not using adaptive filtering
			if !options.AdaptiveFiltering && options.TopK > 0 && len(rankedResults) > options.TopK {
				fmt.Printf("Limiting reranked results from %d to top %d\n", len(rankedResults), options.TopK)
				rankedResults = rankedResults[:options.TopK]
			}

			return rankedResults, nil
		}
	}

	// If BGE is not available or failed, fall back to the standard model
	// Use the model specified in options or the one from RAG
	modelName := options.RerankerModel
	if modelName == "" {
		modelName = rag.ModelName
	}

	fmt.Printf("Using reranker model: %s\n", modelName)

	// Check if the model is a BGE reranker model
	isBGEReranker := strings.Contains(strings.ToLower(modelName), "bge-reranker")

	var rankedResults []RankedResult

	if isBGEReranker {
		// Use BGE reranker for BGE models
		pairs := make([][]string, 0, len(initialResults))
		resultMap := make(map[int]*domain.DocumentChunk)

		// Prepare the pairs for batch processing
		for i, result := range initialResults {
			chunk := rag.GetChunkByID(result.ID)
			if chunk == nil {
				continue
			}

			pairs = append(pairs, []string{query, chunk.Content})
			resultMap[i] = chunk
		}

		// Get all scores at once using the BGE reranker
		scores, err := rs.bgeRerankerClient.ComputeScores(pairs, true) // normalize=true to get 0-1 scores
		if err != nil {
			return nil, fmt.Errorf("error computing BGE reranker scores: %w", err)
		}

		// Process the scores
		for i, score := range scores {
			if i >= len(initialResults) {
				break
			}

			chunk := resultMap[i]
			if chunk == nil {
				continue
			}

			vectorScore := initialResults[i].Score

			// Calculate final score as weighted combination of vector and reranker scores
			finalScore := (options.RerankerWeight * score) + ((1 - options.RerankerWeight) * vectorScore)

			// Add to results if above threshold
			if finalScore >= options.ScoreThreshold {
				rankedResults = append(rankedResults, RankedResult{
					Chunk:         chunk,
					VectorScore:   vectorScore,
					RerankerScore: score,
					FinalScore:    finalScore,
				})
			}
		}
	} else {
		// Use the existing Ollama-based reranker for other models
		// Prepare the cross-encoder prompt template
		var promptTemplate string
		if options.AdaptiveFiltering {
			// Enhanced prompt for adaptive content filtering
			promptTemplate = `You are an advanced document relevance scoring system. Your task is to determine if a document contains useful information to answer a specific query.

Query: %s

Document Content:
%s

Score guidelines:
- Score 0.0-0.2: Document is completely irrelevant to the query
- Score 0.3-0.5: Document has minimal relevance but doesn't directly answer the query
- Score 0.6-0.8: Document contains partial information that's useful for answering the query
- Score 0.9-1.0: Document contains highly relevant information that directly answers the query

Only output a single number between 0 and 1 representing your relevance assessment:
`
		} else {
			// Original prompt for standard reranking
			promptTemplate = `You are a document relevance scoring system. Rate how relevant a document is to a query on a scale from 0 to 1, where 0 is completely irrelevant and 1 is highly relevant.

Query: %s

Document:
%s

Relevance score (output only a single number between 0 and 1):
`
		}

		// Get chunks and score them
		for _, result := range initialResults {
			chunk := rag.GetChunkByID(result.ID)
			if chunk == nil {
				continue
			}

			// Prepare prompt for this chunk
			prompt := fmt.Sprintf(promptTemplate, query, chunk.Content)

			// Get reranking score from the model
			response, err := rs.ollamaClient.GenerateCompletion(modelName, prompt)
			if err != nil {
				return nil, fmt.Errorf("error generating reranking score: %w", err)
			}

			// Parse the response as a float (score)
			var score float64
			_, err = fmt.Sscanf(response, "%f", &score)
			if err != nil {
				// If parsing fails, use a default score (based on vector similarity)
				score = result.Score
			}

			// Ensure score is in range [0,1]
			if score < 0 {
				score = 0
			} else if score > 1 {
				score = 1
			}

			// Calculate final score as weighted combination of vector and reranker scores
			finalScore := (options.RerankerWeight * score) + ((1 - options.RerankerWeight) * result.Score)

			// Add to results if above threshold
			if finalScore >= options.ScoreThreshold {
				rankedResults = append(rankedResults, RankedResult{
					Chunk:         chunk,
					VectorScore:   result.Score,
					RerankerScore: score,
					FinalScore:    finalScore,
				})
			}
		}
	}

	// Sort by final score (descending)
	sort.Slice(rankedResults, func(i, j int) bool {
		return rankedResults[i].FinalScore > rankedResults[j].FinalScore
	})

	// Only apply Top-K limit if we're not using adaptive filtering
	if !options.AdaptiveFiltering && options.TopK > 0 && len(rankedResults) > options.TopK {
		fmt.Printf("Limiting reranked results from %d to top %d\n", len(rankedResults), options.TopK)
		rankedResults = rankedResults[:options.TopK]
	}

	return rankedResults, nil
}
