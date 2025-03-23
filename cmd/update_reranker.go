package cmd

import (
	"fmt"

	"github.com/dontizi/rlama/internal/client"
	"github.com/dontizi/rlama/internal/service"
	"github.com/spf13/cobra"
)

var updateRerankerCmd = &cobra.Command{
	Use:   "update-reranker [rag-name]",
	Short: "Updates the reranking model of an existing RAG",
	Long:  `Configures an existing RAG to use the default BGE Reranker model.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		updateReranker(args[0])
	},
}

func init() {
	rootCmd.AddCommand(updateRerankerCmd)
}

func updateReranker(ragName string) {
	// Load the RAG service
	ollamaClient := client.NewDefaultOllamaClient()
	ragService := service.NewRagService(ollamaClient)

	// Update the reranking model
	err := ragService.UpdateRerankerModel(ragName, "BAAI/bge-reranker-v2-m3")
	if err != nil {
		fmt.Printf("Error updating the reranking model: %v\n", err)
		return
	}

	fmt.Printf("✅ RAG '%s' updated to use the BGE Reranker model.\n", ragName)
}
