package cmd

import (
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/spf13/cobra"
	
	"github.com/dontizi/rlama/internal/client"
	"github.com/dontizi/rlama/internal/service"
)

const (
	Version = "0.1.31" // Current version of RLAMA
)

var rootCmd = &cobra.Command{
	Use:   "rlama",
	Short: "RLAMA is a CLI tool for creating and using RAG systems with Ollama",
	Long: `RLAMA (Retrieval-Augmented Language Model Adapter) is a command-line tool 
that simplifies the creation and use of RAG (Retrieval-Augmented Generation) systems 
based on Ollama models.

Main commands:
  rag [model] [rag-name] [folder-path]    Create a new RAG system
  run [rag-name]                          Run an existing RAG system
  list                                    List all available RAG systems
  delete [rag-name]                       Delete a RAG system
  update                                  Check and install RLAMA updates

Environment variables:
  OLLAMA_HOST                            Specifies default Ollama host:port (overridden by --host and --port flags)`,
}

// Variables to store command flags
var (
	versionFlag bool
	ollamaHost  string
	ollamaPort  string
	verbose     bool
)

// Execute executes the root command
func Execute() error {
	return rootCmd.Execute()
}

// GetOllamaClient returns an Ollama client configured with host and port from command flags
func GetOllamaClient() *client.OllamaClient {
	// Windows may use different environment variable handling
	if runtime.GOOS == "windows" {
		// Ensure Ollama can be found if running via WSL
		if ollamaHost == "" && ollamaPort == "" && os.Getenv("OLLAMA_HOST") == "" {
			// Check if WSL access is needed and Ollama is not running natively
			// This is just an example approach - actual implementation would need to check
			// if Ollama is accessible on localhost first
		}
	}
	
	return client.NewOllamaClient(ollamaHost, ollamaPort)
}

func init() {
	// Add --version flag
	rootCmd.Flags().BoolVarP(&versionFlag, "version", "v", false, "Display RLAMA version")
	
	// Add Ollama configuration flags
	rootCmd.PersistentFlags().StringVar(&ollamaHost, "host", "", "Ollama host (overrides OLLAMA_HOST env var, default: localhost)")
	rootCmd.PersistentFlags().StringVar(&ollamaPort, "port", "", "Ollama port (overrides port in OLLAMA_HOST env var, default: 11434)")
	rootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "Enable verbose output")
	
	// Override the Run function to handle the --version flag
	rootCmd.Run = func(cmd *cobra.Command, args []string) {
		if versionFlag {
			fmt.Printf("RLAMA version %s\n", Version)
			return
		}
		
		// If no arguments are provided and --version is not used, display help
		if len(args) == 0 {
			cmd.Help()
		}
	}
	
	// Start the watcher daemon
	go startFileWatcherDaemon()
}

// Add this function to start the watcher daemon
func startFileWatcherDaemon() {
	// Wait a bit for application initialization
	time.Sleep(2 * time.Second)
	
	// Create the services
	ollamaClient := GetOllamaClient()
	ragService := service.NewRagService(ollamaClient)
	fileWatcher := service.NewFileWatcher(ragService)
	
	// Start the daemon with a 1-minute check interval for its internal operations
	// Actual RAG check intervals are controlled by each RAG's configuration
	fileWatcher.StartWatcherDaemon(1 * time.Minute)
} 
