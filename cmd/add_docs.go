package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/dontizi/rlama/internal/service"
	"github.com/dontizi/rlama/internal/domain"
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
		documentLoader := service.NewDocumentLoader()
		embeddingService := service.NewEmbeddingService(ollamaClient)

		// Load the RAG
		rag, err := ragService.LoadRag(ragName)
		if err != nil {
			return err
		}

		// Check if Ollama is available with the model
		if err := ollamaClient.CheckOllamaAndModel(rag.ModelName); err != nil {
			return err
		}

		// Load documents from the folder
		fmt.Printf("Loading documents from '%s'...\n", folderPath)
		docs, err := documentLoader.LoadDocumentsFromFolder(folderPath)
		if err != nil {
			return fmt.Errorf("error loading documents: %w", err)
		}

		if len(docs) == 0 {
			return fmt.Errorf("no valid documents found in folder %s", folderPath)
		}

		fmt.Printf("Successfully loaded %d documents. Generating embeddings...\n", len(docs))

		// Create chunker service
		chunkerService := service.NewChunkerService(1500, 150)

		// Process each document - chunk and generate embeddings
		var allChunks []*domain.DocumentChunk
		for _, doc := range docs {
			// Chunk the document
			chunks := chunkerService.ChunkDocument(doc)
			
			// Update total chunks in metadata
			for _, chunk := range chunks {
				chunk.UpdateTotalChunks(len(chunks))
			}
			
			allChunks = append(allChunks, chunks...)
		}

		fmt.Printf("Generated %d chunks from %d documents. Generating embeddings...\n", 
			len(allChunks), len(docs))

		// Generate embeddings for all chunks
		err = embeddingService.GenerateChunkEmbeddings(allChunks, rag.ModelName)
		if err != nil {
			return fmt.Errorf("error generating embeddings: %w", err)
		}

		// Track how many new chunks were added
		chunksAdded := 0
		existingChunks := make(map[string]bool)

		// Create a map of existing chunk IDs for quick lookup
		for _, chunk := range rag.Chunks {
			existingChunks[chunk.ID] = true
		}

		// Add chunks to the RAG if they don't already exist
		for _, chunk := range allChunks {
			if !existingChunks[chunk.ID] {
				rag.AddChunk(chunk)
				chunksAdded++
			}
		}

		// Save the RAG
		err = ragService.UpdateRag(rag)
		if err != nil {
			return fmt.Errorf("error saving the RAG: %w", err)
		}

		fmt.Printf("Successfully added %d new chunks to RAG '%s'.\n", chunksAdded, ragName)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(addDocsCmd)
}