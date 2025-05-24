package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/dontizi/rlama/internal/client"
	"github.com/dontizi/rlama/internal/service"
	"github.com/spf13/cobra"
)

const (
	// Version is the current version of RLAMA
	Version = "0.1.37"
	// Default Ollama connection settings
	defaultHost = "localhost"
	defaultPort = "11434"
)

// Global flags
var (
	modelName   string
	verbose     bool
	ollamaHost  string
	ollamaPort  string
	dataDir     string
	versionFlag bool
	numThread   int
)

// GlobalServices holds all global service instances
type GlobalServices struct {
	RagService   service.RagService
	OllamaClient *client.OllamaClient
}

// Services is the global instance of GlobalServices
var Services = &GlobalServices{}

var rootCmd = &cobra.Command{
	Use:   "rlama",
	Short: "RLAMA - Retrieval Local Assistant with Memory Augmentation",
	Long: `RLAMA is a CLI tool for local RAG (Retrieval-Augmented Generation) 
that helps you build and query knowledge bases from your local documents.

Main commands:
  rag [model] [rag-name] [folder-path]    Create a new RAG system
  run [rag-name]                          Run an existing RAG system
  agent [rag-name]                        Run an AI agent with optional RAG
  list                                    List all available RAG systems
  delete [rag-name]                       Delete a RAG system
  update                                  Check and install RLAMA updates`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Initialize services before any command runs
		initServices()
	},
	Run: func(cmd *cobra.Command, args []string) {
		if versionFlag {
			fmt.Printf("RLAMA version %s\n", Version)
			os.Exit(0)
		}
		cmd.Help()
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	// Add global flags
	rootCmd.PersistentFlags().StringVarP(&modelName, "model", "m", "qwen3:8b", "Model to use for LLM operations")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")
	rootCmd.PersistentFlags().StringVar(&ollamaHost, "host", "", "Ollama host (overrides OLLAMA_HOST env var)")
	rootCmd.PersistentFlags().StringVar(&ollamaPort, "port", "", "Ollama port (overrides port in OLLAMA_HOST env var)")
	rootCmd.PersistentFlags().StringVar(&dataDir, "data-dir", "", "Directory for storing RLAMA data")
	rootCmd.PersistentFlags().IntVar(&numThread, "num-thread", 0, "Number of threads for Ollama to use (0 = use Ollama default)")
	rootCmd.Flags().BoolVar(&versionFlag, "version", false, "Display RLAMA version")

	// Set default data directory if not specified
	if dataDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			fmt.Printf("Warning: Could not determine home directory: %v\n", err)
			dataDir = ".rlama"
		} else {
			dataDir = filepath.Join(homeDir, ".rlama")
		}
	}
}

// GetOllamaClient returns an Ollama client configured with host and port from command flags
func GetOllamaClient() *client.OllamaClient {
	// If services are already initialized, return the existing client
	if Services.OllamaClient != nil {
		return Services.OllamaClient
	}

	// Otherwise, initialize a new client
	if ollamaHost == "" {
		ollamaHost = os.Getenv("OLLAMA_HOST")
		if ollamaHost == "" {
			ollamaHost = defaultHost
		}
	}
	if ollamaPort == "" {
		ollamaPort = defaultPort
	}

	// Windows may use different environment variable handling
	if runtime.GOOS == "windows" {
		// Ensure Ollama can be found if running via WSL
		if ollamaHost == "" && ollamaPort == "" && os.Getenv("OLLAMA_HOST") == "" {
			// Check if WSL access is needed and Ollama is not running natively
			// This is just a placeholder - actual implementation would need to check
			// if Ollama is accessible on localhost first
			ollamaHost = defaultHost
			ollamaPort = defaultPort
		}
	}

	// Create and store the client in Services
	Services.OllamaClient = client.NewOllamaClient(ollamaHost, ollamaPort, numThread)
	return Services.OllamaClient
}

func initServices() {
	// Get or create Ollama client
	Services.OllamaClient = GetOllamaClient()
	if err := Services.OllamaClient.CheckLLMAndModel(modelName); err != nil {
		fmt.Printf("Warning: Failed to verify model %s: %v\n", modelName, err)
		fmt.Println("Make sure Ollama is running and the model is installed.")
	}

	// Initialize RAG service
	Services.RagService = service.NewRagService(Services.OllamaClient)

	if verbose {
		fmt.Printf("Initialized services:\n")
		fmt.Printf("- Ollama client: %s:%s\n", ollamaHost, ollamaPort)
		fmt.Printf("- Data directory: %s\n", dataDir)
		fmt.Printf("- Default model: %s\n", modelName)
	}
}
