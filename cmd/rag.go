package cmd

import (
	"fmt"
	"strings"

	"github.com/dontizi/rlama/internal/client"
	"github.com/dontizi/rlama/internal/service"
	"github.com/spf13/cobra"
)

var (
	excludeDirs        []string
	excludeExts        []string
	processExts        []string
	chunkSize          int
	chunkOverlap       int
	chunkingStrategy   string
	profileName        string
	ragDisableReranker bool
	ragRerankerModel   string
	ragRerankerWeight  float64
	testService        interface{} // Pour les tests
)

var ragCmd = &cobra.Command{
	Use:   "rag [model] [rag-name] [folder-path]",
	Short: "Create a new RAG system",
	Long: `Create a new RAG system by indexing all documents in the specified folder.
Example: rlama rag llama3.2 rag1 ./documents

The folder will be created if it doesn't exist yet.
Supported formats include: .txt, .md, .html, .json, .csv, and various source code files.

You can exclude directories or file types:
  rlama rag llama3 myproject ./code --excludedir=node_modules,dist,.git
  rlama rag llama3 mydocs ./docs --excludeext=.log,.tmp
  rlama rag llama3 specific ./mixed --processext=.md,.py,.js

Hugging Face Models:
  You can use Hugging Face GGUF models with the format:
  rlama rag hf.co/username/repository my-rag ./docs
  
  Specify quantization with:
  rlama rag hf.co/username/repository:Q4_K_M my-rag ./docs
  
OpenAI Models:
  You can use OpenAI models by setting the OPENAI_API_KEY environment variable:
  export OPENAI_API_KEY="your-api-key"
  
  Then use any OpenAI model:
  rlama rag gpt-4-turbo my-openai-rag ./docs
  rlama rag gpt-3.5-turbo my-openai-rag ./docs`,
	Args: cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		modelName := args[0]
		ragName := args[1]
		folderPath := args[2]

		// Get Ollama client with configured host and port
		ollamaClient := GetOllamaClient()

		// Check if this is an OpenAI model
		isOpenAIModel := client.IsOpenAIModel(modelName)

		if isOpenAIModel {
			// For OpenAI models, check the specified profile or API key
			var openaiClient *client.OpenAIClient
			var err error

			if profileName != "" {
				openaiClient, err = client.NewOpenAIClientWithProfile(profileName)
				if err != nil {
					return err
				}
				fmt.Printf("Using OpenAI model '%s' with profile '%s' for inference.\n",
					modelName, profileName)
			} else {
				openaiClient = client.NewOpenAIClient()
				if err := openaiClient.CheckOpenAIAndModel(modelName); err != nil {
					return err
				}
				fmt.Printf("Using OpenAI model '%s' for inference. No profile specified, using environment variable.\n",
					modelName)
			}
		} else if client.IsHuggingFaceModel(modelName) {
			// Check if this is a Hugging Face model
			isHfModel := client.IsHuggingFaceModel(modelName)

			if isHfModel {
				// Extract quantization if specified
				hfModelName := client.GetHuggingFaceModelName(modelName)
				quantization := client.GetQuantizationFromModelRef(modelName)

				// Pull the model from Hugging Face
				fmt.Printf("Pulling Hugging Face model %s...\n", hfModelName)
				if err := ollamaClient.PullHuggingFaceModel(hfModelName, quantization); err != nil {
					return fmt.Errorf("error pulling Hugging Face model: %w", err)
				}
			} else {
				// Regular Ollama model check
				if err := ollamaClient.CheckOllamaAndModel(modelName); err != nil {
					return err
				}
			}
		} else {
			// Regular Ollama model check
			if err := ollamaClient.CheckOllamaAndModel(modelName); err != nil {
				return err
			}
		}

		// Display a message to indicate that the process has started
		fmt.Printf("Creating RAG '%s' with model '%s' from folder '%s'...\n",
			ragName, modelName, folderPath)

		// Set up loader options based on flags
		loaderOptions := service.DocumentLoaderOptions{
			ExcludeDirs:      excludeDirs,
			ExcludeExts:      excludeExts,
			ProcessExts:      processExts,
			ChunkSize:        chunkSize,
			ChunkOverlap:     chunkOverlap,
			ChunkingStrategy: chunkingStrategy,
			APIProfileName:   profileName,
			EnableReranker:   !ragDisableReranker,
			RerankerModel:    ragRerankerModel,
			RerankerWeight:   ragRerankerWeight,
		}

		ragService := service.NewRagService(ollamaClient)
		err := ragService.CreateRagWithOptions(modelName, ragName, folderPath, loaderOptions)
		if err != nil {
			// Improve error messages related to Ollama
			if strings.Contains(err.Error(), "connection refused") {
				return fmt.Errorf("⚠️ Unable to connect to Ollama.\n" +
					"Make sure Ollama is installed and running.\n")
			}
			return err
		}

		fmt.Printf("RAG '%s' created successfully.\n", ragName)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(ragCmd)

	// Add exclusion and processing flags
	ragCmd.Flags().StringSliceVar(&excludeDirs, "exclude-dir", []string{}, "Directories to exclude (comma-separated)")
	ragCmd.Flags().StringSliceVar(&excludeExts, "exclude-ext", []string{}, "File extensions to exclude (comma-separated)")
	ragCmd.Flags().StringSliceVar(&processExts, "process-ext", []string{}, "File extensions to process (others will be ignored)")

	// Add flags for chunking options
	ragCmd.Flags().IntVar(&chunkSize, "chunk-size", 1000, "Character count per chunk")
	ragCmd.Flags().IntVar(&chunkOverlap, "chunk-overlap", 200, "Overlap between chunks in characters")
	ragCmd.Flags().StringVar(&chunkingStrategy, "chunking", "hybrid", "Chunking strategy (options: fixed, semantic, hybrid, hierarchical)")

	// Add reranking options - now with a flag to disable it instead
	ragCmd.Flags().BoolVar(&ragDisableReranker, "disable-reranker", false, "Disable reranking (enabled by default)")
	ragCmd.Flags().StringVar(&ragRerankerModel, "reranker-model", "", "Model to use for reranking (defaults to main model)")
	ragCmd.Flags().Float64Var(&ragRerankerWeight, "reranker-weight", 0.7, "Weight for reranker scores vs vector scores (0-1)")

	// Add profile option
	ragCmd.Flags().StringVar(&profileName, "profile", "", "API profile name for OpenAI models")

	// Add logic to use the test service if available
	if testService != nil {
		// Use the test service
	}
}

// NewRagCommand returns the rag command
func NewRagCommand() *cobra.Command {
	return ragCmd
}

// InjectTestService injects a test service
func InjectTestService(service interface{}) {
	testService = service
}
