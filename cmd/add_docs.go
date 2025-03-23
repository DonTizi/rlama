package cmd

import (
	"fmt"

	"github.com/dontizi/rlama/internal/service"
	"github.com/spf13/cobra"
)

var (
	addDocsExcludeDirs      []string
	addDocsExcludeExts      []string
	addDocsProcessExts      []string
	addDocsChunkSize        int
	addDocsChunkOverlap     int
	addDocsChunkingStrategy string
	addDocsDisableReranker  bool
	addDocsRerankerModel    string
	addDocsRerankerWeight   float64
)

var addDocsCmd = &cobra.Command{
	Use:   "add-docs [rag-name] [folder-path]",
	Short: "Add documents to an existing RAG system",
	Long: `Add documents from a folder to an existing RAG system.
Example: rlama add-docs my-docs ./new-documents
	
This will load documents from the specified folder, generate embeddings,
and add them to the existing RAG system.`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		ragName := args[0]
		folderPath := args[1]

		// Get Ollama client from root command
		ollamaClient := GetOllamaClient()

		// Create necessary services
		ragService := service.NewRagService(ollamaClient)

		// Set up loader options based on flags
		loaderOptions := service.DocumentLoaderOptions{
			ExcludeDirs:      addDocsExcludeDirs,
			ExcludeExts:      addDocsExcludeExts,
			ProcessExts:      addDocsProcessExts,
			ChunkSize:        addDocsChunkSize,
			ChunkOverlap:     addDocsChunkOverlap,
			ChunkingStrategy: addDocsChunkingStrategy,
			EnableReranker:   !addDocsDisableReranker,
			RerankerModel:    addDocsRerankerModel,
			RerankerWeight:   addDocsRerankerWeight,
		}

		// Pass the options to the service
		err := ragService.AddDocsWithOptions(ragName, folderPath, loaderOptions)
		if err != nil {
			return err
		}

		fmt.Printf("Documents from '%s' added to RAG '%s' successfully.\n", folderPath, ragName)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(addDocsCmd)

	// Add exclusion and processing flags
	addDocsCmd.Flags().StringSliceVar(&addDocsExcludeDirs, "exclude-dir", []string{}, "Directories to exclude (comma-separated)")
	addDocsCmd.Flags().StringSliceVar(&addDocsExcludeExts, "exclude-ext", []string{}, "File extensions to exclude (comma-separated)")
	addDocsCmd.Flags().StringSliceVar(&addDocsProcessExts, "process-ext", []string{}, "Only process these file extensions (comma-separated)")

	// Add chunking options
	addDocsCmd.Flags().IntVar(&addDocsChunkSize, "chunk-size", 1000, "Character count per chunk")
	addDocsCmd.Flags().IntVar(&addDocsChunkOverlap, "chunk-overlap", 200, "Overlap between chunks in characters")
	addDocsCmd.Flags().StringVar(&addDocsChunkingStrategy, "chunking-strategy", "hybrid",
		"Chunking strategy to use (options: \"fixed\", \"semantic\", \"hybrid\", \"hierarchical\", \"auto\"). "+
			"The \"auto\" strategy will analyze each document and apply the optimal strategy automatically.")

	// Add reranking options
	addDocsCmd.Flags().BoolVar(&addDocsDisableReranker, "disable-reranker", false, "Disable reranking for this RAG")
	addDocsCmd.Flags().StringVar(&addDocsRerankerModel, "reranker-model", "", "Model to use for reranking (defaults to RAG model)")
	addDocsCmd.Flags().Float64Var(&addDocsRerankerWeight, "reranker-weight", 0.7, "Weight for reranker scores vs vector scores (0-1)")
}
