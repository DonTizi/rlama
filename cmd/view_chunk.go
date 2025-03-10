package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/dontizi/rlama/internal/service"
)

var viewChunkCmd = &cobra.Command{
	Use:   "view-chunk [rag-name] [chunk-id]",
	Short: "View a specific chunk's details and content",
	Long: `Display detailed information about a specific chunk, including its full content.
Example: rlama view-chunk my-rag doc123_chunk_2

You can find chunk IDs by using the list-chunks command.`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		ragName := args[0]
		chunkID := args[1]

		// Get services
		ollamaClient := GetOllamaClient()
		ragService := service.NewRagService(ollamaClient)

		// Load the RAG
		rag, err := ragService.LoadRag(ragName)
		if err != nil {
			return err
		}

		// Find the chunk
		chunk := rag.GetChunkByID(chunkID)
		if chunk == nil {
			return fmt.Errorf("chunk with ID '%s' not found", chunkID)
		}

		// Display chunk details
		fmt.Printf("Chunk ID: %s\n", chunk.ID)
		fmt.Printf("Document: %s\n", chunk.Metadata["document_name"])
		fmt.Printf("Position: %s\n", chunk.Metadata["chunk_position"])
		fmt.Printf("Size: %d characters\n", len(chunk.Content))
		fmt.Printf("Document Path: %s\n", chunk.Metadata["document_path"])
		
		// Display other metadata
		fmt.Println("\nMetadata:")
		for k, v := range chunk.Metadata {
			if k != "document_name" && k != "chunk_position" && k != "document_path" {
				fmt.Printf("  %s: %s\n", k, v)
			}
		}
		
		// Display content with clear boundaries
		fmt.Println("\n=== CHUNK CONTENT START ===")
		fmt.Println(chunk.Content)
		fmt.Println("=== CHUNK CONTENT END ===")
		
		return nil
	},
}

func init() {
	rootCmd.AddCommand(viewChunkCmd)
} 