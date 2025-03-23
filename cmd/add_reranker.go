package cmd

import (
	"fmt"

	"github.com/dontizi/rlama/internal/service"
	"github.com/spf13/cobra"
)

var (
	rerankerModel     string
	rerankerWeight    float64
	rerankerThreshold float64
	rerankerTopK      int
	disableReranker   bool
)

var addRerankerCmd = &cobra.Command{
	Use:   "add-reranker [rag-name]",
	Short: "Configure reranking for a RAG system",
	Long: `Configure reranking settings for a RAG system (note: reranking is enabled by default).
Example: rlama add-reranker my-rag --model reranker-model

Reranking improves retrieval accuracy by applying a second-stage ranking to initial search results.
This uses a cross-encoder approach to evaluate the relevance of each document to the query.

Use --disable flag to turn off reranking if needed.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ragName := args[0]

		// Get Ollama client from root command
		ollamaClient := GetOllamaClient()

		// Create RAG service
		ragService := service.NewRagService(ollamaClient)

		// Load the RAG
		rag, err := ragService.LoadRag(ragName)
		if err != nil {
			return fmt.Errorf("error loading RAG: %w", err)
		}

		if disableReranker {
			// Disable reranking
			rag.RerankerEnabled = false
			fmt.Printf("Reranking disabled for RAG '%s'\n", ragName)
		} else {
			// Enable reranking with specified settings
			rag.RerankerEnabled = true

			if rerankerModel != "" {
				rag.RerankerModel = rerankerModel
			} else if rag.RerankerModel == "" {
				// If not set, use the same model as the RAG
				rag.RerankerModel = rag.ModelName
			}

			// Set weight if specified
			if cmd.Flags().Changed("weight") {
				rag.RerankerWeight = rerankerWeight
			} else if rag.RerankerWeight == 0 {
				// Set default if not already set
				rag.RerankerWeight = 0.7
			}

			// Set threshold if specified
			if cmd.Flags().Changed("threshold") {
				rag.RerankerThreshold = rerankerThreshold
			}

			// Set max results to return if specified
			if cmd.Flags().Changed("topk") {
				rag.RerankerTopK = rerankerTopK
			} else if rag.RerankerTopK == 0 {
				// Set default if not already set
				rag.RerankerTopK = 5
			}

			fmt.Printf("Reranking enabled for RAG '%s'\n", ragName)
			fmt.Printf("  Model: %s\n", rag.RerankerModel)
			fmt.Printf("  Weight: %.2f\n", rag.RerankerWeight)
			fmt.Printf("  Threshold: %.2f\n", rag.RerankerThreshold)
			fmt.Printf("  Max results: %d\n", rag.RerankerTopK)
		}

		// Update the RAG
		err = ragService.UpdateRag(rag)
		if err != nil {
			return fmt.Errorf("error updating RAG: %w", err)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(addRerankerCmd)

	// Configure reranking options
	addRerankerCmd.Flags().StringVar(&rerankerModel, "model", "", "Model to use for reranking (defaults to RAG model if not specified)")
	addRerankerCmd.Flags().Float64Var(&rerankerWeight, "weight", 0.7, "Weight for reranker scores vs vector scores (0-1)")
	addRerankerCmd.Flags().Float64Var(&rerankerThreshold, "threshold", 0.0, "Minimum score threshold for reranked results")
	addRerankerCmd.Flags().IntVar(&rerankerTopK, "topk", 5, "Maximum number of results to return after reranking")
	addRerankerCmd.Flags().BoolVar(&disableReranker, "disable", false, "Disable reranking for this RAG")
}
