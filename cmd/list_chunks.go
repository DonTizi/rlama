package cmd

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/dontizi/rlama/internal/service"
)

var (
	showContent    bool
	filterDocument string
)

var listChunksCmd = &cobra.Command{
	Use:   "list-chunks [rag-name]",
	Short: "List all chunks in a RAG system",
	Long: `List all chunks in a RAG system, showing their IDs, document sources, and size.
Example: rlama list-chunks my-rag

Use --show-content to display the full content of each chunk.
Use --document="name" to filter chunks from a specific document.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ragName := args[0]

		// Get services
		ollamaClient := GetOllamaClient()
		ragService := service.NewRagService(ollamaClient)

		// Load the RAG
		rag, err := ragService.LoadRag(ragName)
		if err != nil {
			return err
		}

		// Create tabwriter for nicely formatted output
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintf(w, "Chunk ID\tDocument\tChunk Position\tSize (chars)\n")
		fmt.Fprintf(w, "--------\t--------\t-------------\t------------\n")

		// Track totals
		totalChunks := 0
		totalSize := 0
		displayedChunks := 0

		// Process all chunks
		for _, chunk := range rag.Chunks {
			// Apply document filter if set
			if filterDocument != "" && !strings.Contains(chunk.Metadata["document_name"], filterDocument) {
				continue
			}

			totalChunks++
			totalSize += len(chunk.Content)
			displayedChunks++

			// Print chunk info
			fmt.Fprintf(w, "%s\t%s\t%s\t%d\n",
				chunk.ID,
				chunk.Metadata["document_name"],
				chunk.Metadata["chunk_position"],
				len(chunk.Content))

			// Show content if requested
			if showContent {
				w.Flush() // Flush the table before showing content
				fmt.Println("\n--- Chunk Content ---")
				fmt.Println(chunk.Content)
				fmt.Println("---------------------\n")
			}
		}

		w.Flush()

		// Show summary
		fmt.Printf("\nTotal: %d chunks, avg size: %d chars", 
			totalChunks, totalSize/max(totalChunks, 1))
		
		if filterDocument != "" {
			fmt.Printf(" (filtered to show %d chunks from documents matching '%s')\n",
				displayedChunks, filterDocument)
		} else {
			fmt.Println()
		}

		return nil
	},
}

// Helper function for average calculation
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func init() {
	rootCmd.AddCommand(listChunksCmd)
	
	// Add flags
	listChunksCmd.Flags().BoolVar(&showContent, "show-content", false, 
		"Show the full content of each chunk")
	listChunksCmd.Flags().StringVar(&filterDocument, "document", "", 
		"Filter chunks by document name (substring match)")
} 