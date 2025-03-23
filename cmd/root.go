package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/dontizi/rlama/internal/client"
	"github.com/dontizi/rlama/internal/service"
)

const (
	Version = "0.1.34" // Current version of RLAMA
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
	dataDir     string
)

var (
	globalRagService   service.RagService
	globalAgentService *service.AgentService
)

// SetServices permet de définir les services globaux
func SetServices(ragService service.RagService, agentService *service.AgentService) {
	globalRagService = ragService
	globalAgentService = agentService
}

// GetRagService retourne le service RAG global
func GetRagService() service.RagService {
	if globalRagService == nil {
		ollamaClient := GetOllamaClient()
		globalRagService = service.NewRagService(ollamaClient)
	}
	return globalRagService
}

// GetAgentService retourne le service Agent global
func GetAgentService() *service.AgentService {
	if globalAgentService == nil {
		ollamaClient := GetOllamaClient()
		ragService := GetRagService()
		globalAgentService = service.NewAgentService(ollamaClient, ragService)
	}
	return globalAgentService
}

// Execute executes the root command
func Execute() error {
	return rootCmd.Execute()
}

// GetOllamaClient returns an Ollama client with the configured host and port
func GetOllamaClient() *client.OllamaClient {
	// Si les flags sont définis, ils ont priorité
	host := ollamaHost
	port := ollamaPort

	// Sinon, utiliser les valeurs par défaut
	if host == "" {
		host = client.DefaultOllamaHost
	}

	return client.NewOllamaClient(host, port)
}

func init() {
	// Add --version flag
	rootCmd.Flags().BoolVarP(&versionFlag, "version", "v", false, "Display RLAMA version")

	// Add Ollama configuration flags
	rootCmd.PersistentFlags().StringVar(&ollamaHost, "host", "", "Ollama host (overrides OLLAMA_HOST env var, default: localhost)")
	rootCmd.PersistentFlags().StringVar(&ollamaPort, "port", "", "Ollama port (overrides port in OLLAMA_HOST env var, default: 11434)")
	rootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "Enable verbose output")

	// New flag for data directory
	rootCmd.PersistentFlags().StringVar(&dataDir, "data-dir", "", "Custom data directory path")

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

// Ajouter une fonction pour vérifier la connexion à Ollama
func checkOllamaConnection() error {
	client := GetOllamaClient()

	// Check if Ollama is running
	running, err := client.IsOllamaRunning()
	if err != nil {
		return fmt.Errorf("erreur de connexion à Ollama: %w", err)
	}
	if !running {
		return fmt.Errorf("Ollama n'est pas en cours d'exécution")
	}

	return nil
}

// Ajouter une fonction de vérification avec timeout court
func checkOllamaConnectionWithTimeout(timeoutSeconds int) error {
	// Créer un canal pour le résultat
	resultChan := make(chan error, 1)

	// Lancer la vérification dans une goroutine
	go func() {
		resultChan <- checkOllamaConnection()
	}()

	// Attendre avec un timeout
	select {
	case err := <-resultChan:
		return err
	case <-time.After(time.Duration(timeoutSeconds) * time.Second):
		return fmt.Errorf("timeout lors de la vérification de connexion à Ollama")
	}
}
