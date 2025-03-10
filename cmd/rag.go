package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/dontizi/rlama/internal/service"
	"github.com/dontizi/rlama/internal/client"
)

var ragCmd = &cobra.Command{
	Use:   "rag [model] [rag-name] [folder-path]",
	Short: "Create a new RAG system",
	Long: `Create a new RAG system by indexing all documents in the specified folder.
Example: rlama rag llama3.2 rag1 ./documents

The folder will be created if it doesn't exist yet.
Supported formats include: .txt, .md, .html, .json, .csv, and various source code files.`,
	Args: cobra.ExactArgs(3),
	RunE: runRagCmd,
}

var smallModelOptimized bool

func init() {
	rootCmd.AddCommand(ragCmd)
	
	// Add to existing flags for the rag command
	ragCmd.Flags().BoolVar(&smallModelOptimized, "optimize-small", false, 
		"Optimize RAG for small models (use smaller chunks, more overlap)")
	
	// If your rag command has subcommands like create/init, add it to that specific subcommand
	// For example:
	// ragCreateCmd.Flags().BoolVar(&smallModelOptimized, "optimize-small", false, 
	//	"Optimize RAG for small models (use smaller chunks, more overlap)")
}

func runRagCmd(cmd *cobra.Command, args []string) error {
	// Get existing arguments
	if len(args) < 3 {
		return fmt.Errorf("expected model, name and documents folder")
	}
	
	modelName := args[0]
	ragName := args[1]
	documentsPath := args[2]
	
	fmt.Printf("Creating RAG '%s' with model '%s' from folder '%s'...\n", 
		ragName, modelName, documentsPath)
	
	// IMPORTANT: Detect small model BEFORE creating chunker
	isSmallModel := false
	
	// Check flag first
	if smallModelOptimized {
		isSmallModel = true
		fmt.Println("Small model optimizations enabled via flag")
	} else {
		// Auto-detect based on model name
		ollamaClient := client.NewOllamaClient("http://localhost:11434", "")
		modelSize, err := ollamaClient.GetModelSize(modelName)
		if err == nil && modelSize <= 7 {
			isSmallModel = true
			fmt.Println("Small model detected automatically:", modelName)
		}
	}
	
	// Set chunking parameters based on model size
	chunkSize := 1500  // Default for large models
	chunkOverlap := 150
	
	if isSmallModel {
		chunkSize = 800      // Smaller chunks for small models
		chunkOverlap = 200   // More overlap
		fmt.Printf("Using small model optimizations: chunk size=%d, overlap=%d\n", 
			chunkSize, chunkOverlap)
	}
	
	// Get Ollama client with configured host and port
	ollamaClient := GetOllamaClient()
	if err := ollamaClient.CheckOllamaAndModel(modelName); err != nil {
		return err
	}

	ragService := service.NewRagService(ollamaClient)
	err := ragService.CreateRag(modelName, ragName, documentsPath, chunkSize, chunkOverlap)
	if err != nil {
		// Improve error messages related to Ollama
		if strings.Contains(err.Error(), "connection refused") {
			return fmt.Errorf("⚠️ Unable to connect to Ollama.\n"+
				"Make sure Ollama is installed and running.\n")
		}
		return err
	}

	fmt.Printf("RAG '%s' created successfully.\n", ragName)
	return nil
}