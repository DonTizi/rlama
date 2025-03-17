package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/dontizi/rlama/internal/service"
	"github.com/dontizi/rlama/internal/domain"
)

var (
	contextSize int
)

var runCmd = &cobra.Command{
	Use:   "run [rag-name]",
	Short: "Run a RAG system",
	Long: `Run a previously created RAG system. 
Starts an interactive session to interact with the RAG system.
Example: rlama run rag1`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ragName := args[0]

		// Get Ollama client with configured host and port
		ollamaClient := GetOllamaClient()
		if err := ollamaClient.CheckOllamaAndModel(""); err != nil {
			return err
		}

		ragService := service.NewRagService(ollamaClient)
		rag, err := ragService.LoadRag(ragName)
		if err != nil {
			return err
		}

		fmt.Printf("RAG '%s' loaded. Model: %s\n", rag.Name, rag.ModelName)
		fmt.Println("Type your question (or 'exit' to quit):")

		scanner := bufio.NewScanner(os.Stdin)
		for {
			fmt.Print("> ")
			if !scanner.Scan() {
				break
			}

			question := scanner.Text()
			if question == "exit" {
				break
			}

			if strings.TrimSpace(question) == "" {
				continue
			}

			fmt.Println("\nSearching documents for relevant information...")

			checkWatchedResources(rag, ragService)

			answer, err := ragService.Query(rag, question, contextSize)
			if err != nil {
				fmt.Printf("Error: %s\n", err)
				continue
			}

			fmt.Println("\n--- Answer ---")
			fmt.Println(answer)
			fmt.Println()
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(runCmd)
	
	// Add context size flag
	runCmd.Flags().IntVar(&contextSize, "context-size", 20, "Number of context chunks to retrieve (default: 20)")
}

func checkWatchedResources(rag *domain.RagSystem, ragService service.RagService) {
	// Check watched directory if enabled with on-use check
	if rag.WatchEnabled && rag.WatchInterval == 0 {
		fileWatcher := service.NewFileWatcher(ragService)
		docsAdded, err := fileWatcher.CheckAndUpdateRag(rag)
		if err != nil {
			fmt.Printf("Error checking watched directory: %v\n", err)
		} else if docsAdded > 0 {
			fmt.Printf("Added %d new documents from watched directory.\n", docsAdded)
		}
	}
	
	// Check watched website if enabled with on-use check
	if rag.WebWatchEnabled && rag.WebWatchInterval == 0 {
		webWatcher := service.NewWebWatcherWithoutAgent(ragService)
		pagesAdded, err := webWatcher.CheckAndUpdateRag(rag)
		if err != nil {
			fmt.Printf("Error checking watched website: %v\n", err)
		} else if pagesAdded > 0 {
			fmt.Printf("Added %d new pages from watched website.\n", pagesAdded)
		}
	}
}