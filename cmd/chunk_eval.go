package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/dontizi/rlama/internal/domain"
	"github.com/dontizi/rlama/internal/service"
	"github.com/spf13/cobra"
)

var (
	targetFile      string
	outputDetailed  bool
	compareAll      bool
	customChunkSize int
	customOverlap   int
	customStrategy  string
)

// chunkEvalCmd represents the command to evaluate chunking strategies
var chunkEvalCmd = &cobra.Command{
	Use:   "chunk-eval",
	Short: "Evaluate and optimize chunking strategies for different documents",
	Long: `Evaluate and compare different chunking strategies for a given document.
This command allows you to:
- Test a specific chunking configuration on a document
- Automatically compare multiple strategies to find the best one
- Get detailed metrics on chunking quality

Examples:
  rlama chunk-eval --file=document.md
  rlama chunk-eval --file=code.go --strategy=semantic --size=1000 --overlap=100
  rlama chunk-eval --file=document.txt --compare-all --detailed`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Check if file exists
		if targetFile == "" {
			return fmt.Errorf("please specify a file with --file")
		}

		fileInfo, err := os.Stat(targetFile)
		if err != nil {
			return fmt.Errorf("error accessing file: %w", err)
		}

		if fileInfo.IsDir() {
			return fmt.Errorf("the specified path is a directory, not a file")
		}

		// Load file content
		content, err := os.ReadFile(targetFile)
		if err != nil {
			return fmt.Errorf("error reading file: %w", err)
		}

		// Create document from file
		doc := &domain.Document{
			ID:      filepath.Base(targetFile),
			Name:    filepath.Base(targetFile),
			Path:    targetFile,
			Content: string(content),
		}

		// Create evaluator
		chunkerService := service.NewChunkerService(service.DefaultChunkingConfig())
		evaluator := service.NewChunkingEvaluator(chunkerService)

		fmt.Printf("Analyzing document: %s (%d characters)\n", doc.Name, len(doc.Content))

		// If --compare-all is specified, compare all strategies
		if compareAll {
			fmt.Println("\nComparing all available chunking strategies...")
			startTime := time.Now()

			results := evaluator.CompareChunkingStrategies(doc)

			fmt.Printf("\nAnalysis completed in %.2f seconds\n", time.Since(startTime).Seconds())

			// Display top 5 strategies
			fmt.Println("\n=== Top 5 strategies for this document ===")
			fmt.Println("Rank | Strategy        | Size   | Overlap | Score  | Chunks | Time (ms)")
			fmt.Println("-----|----------------|--------|---------|--------|--------|----------")

			limit := 5
			if len(results) < limit {
				limit = len(results)
			}

			for i := 0; i < limit; i++ {
				fmt.Printf("%4d | %-15s | %6d | %7d | %.4f | %6d | %6d\n",
					i+1,
					results[i].Strategy,
					results[i].ChunkSize,
					results[i].ChunkOverlap,
					results[i].SemanticCoherenceScore,
					results[i].TotalChunks,
					results[i].ProcessingTimeMs)
			}

			// Show details of the best strategy
			if len(results) > 0 && outputDetailed {
				fmt.Println("\nDetails of the best strategy:")
				evaluator.PrintEvaluationResults(results[0])
			}

			// Recommended configuration
			if len(results) > 0 {
				best := results[0]
				fmt.Printf("\nRecommended configuration for this document:\n")
				fmt.Printf("  --strategy=%s --size=%d --overlap=%d\n",
					best.Strategy, best.ChunkSize, best.ChunkOverlap)
			}

			return nil
		}

		// Otherwise, evaluate a specific configuration
		config := service.DefaultChunkingConfig()

		// Use custom parameters if specified
		if customStrategy != "" {
			config.ChunkingStrategy = customStrategy
		}

		if customChunkSize > 0 {
			config.ChunkSize = customChunkSize
		}

		if customOverlap >= 0 {
			config.ChunkOverlap = customOverlap
		}

		fmt.Printf("\nEvaluating strategy: %s (size: %d, overlap: %d)\n",
			config.ChunkingStrategy, config.ChunkSize, config.ChunkOverlap)

		// Evaluate the strategy
		metrics := evaluator.EvaluateChunkingStrategy(doc, config)

		// Display results
		if outputDetailed {
			evaluator.PrintEvaluationResults(metrics)
		} else {
			// Simplified display
			fmt.Println("\n=== Evaluation Results ===")
			fmt.Printf("Coherence score: %.4f\n", metrics.SemanticCoherenceScore)
			fmt.Printf("Number of chunks: %d\n", metrics.TotalChunks)
			fmt.Printf("Average chunk size: %.2f characters\n", metrics.AverageChunkSize)
			fmt.Printf("Split sentences: %d (%.1f%%)\n",
				metrics.ChunksWithSplitSentences,
				float64(metrics.ChunksWithSplitSentences)/float64(metrics.TotalChunks)*100)
			fmt.Printf("Split paragraphs: %d (%.1f%%)\n",
				metrics.ChunksWithSplitParagraphs,
				float64(metrics.ChunksWithSplitParagraphs)/float64(metrics.TotalChunks)*100)
			fmt.Printf("Redundancy rate: %.1f%%\n", metrics.RedundancyRate*100)
		}

		// Suggest other strategies if score is low
		if metrics.SemanticCoherenceScore < 0.7 {
			fmt.Println("\nThe coherence score is relatively low. Try comparing other strategies with:")
			fmt.Printf("  rlama chunk-eval --file=%s --compare-all\n", targetFile)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(chunkEvalCmd)

	// Flags
	chunkEvalCmd.Flags().StringVar(&targetFile, "file", "", "Path to the file to analyze")
	chunkEvalCmd.Flags().BoolVar(&outputDetailed, "detailed", false, "Show detailed results")
	chunkEvalCmd.Flags().BoolVar(&compareAll, "compare-all", false, "Compare all available strategies")
	chunkEvalCmd.Flags().IntVar(&customChunkSize, "size", 0, "Custom chunk size")
	chunkEvalCmd.Flags().IntVar(&customOverlap, "overlap", -1, "Custom overlap")
	chunkEvalCmd.Flags().StringVar(&customStrategy, "strategy", "",
		"Chunking strategy to use (fixed, semantic, hybrid, hierarchical)")
}
