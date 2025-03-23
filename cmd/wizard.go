package cmd

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/dontizi/rlama/internal/crawler"
	"github.com/dontizi/rlama/internal/service"
	"github.com/spf13/cobra"
)

// Structure to parse the JSON output of Ollama list
type OllamaModel struct {
	Name       string `json:"name"`
	Size       int64  `json:"size"`
	ModifiedAt string `json:"modified_at"`
	Digest     string `json:"digest"`
}

var (
	// Variables for the local wizard
	localWizardModel        string
	localWizardName         string
	localWizardPath         string
	localWizardChunkSize    int
	localWizardChunkOverlap int
	localWizardExcludeDirs  []string
	localWizardExcludeExts  []string
	localWizardProcessExts  []string
)

var localWizardCmd = &cobra.Command{
	Use:   "wizard",
	Short: "Local RAG creation wizard",
	Long:  `Interactive wizard to create and configure a local RAG system.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Print("\n🧙 Welcome to the RLAMA Local RAG Wizard! 🧙\n\n")

		reader := bufio.NewReader(os.Stdin)

		// Étape 1: Nom du RAG
		fmt.Print("Enter a name for your RAG: ")
		ragName, _ := reader.ReadString('\n')
		ragName = strings.TrimSpace(ragName)
		if ragName == "" {
			return fmt.Errorf("RAG name cannot be empty")
		}

		// Declare modelName at the function level so it's available everywhere
		var modelName string

		// Step 2: Model selection
		fmt.Println("\nStep 2: Select a model")

		// Get the list of available Ollama models via the ollama list command
		fmt.Println("Retrieving available Ollama models...")

		// First try with ollama list without --json for better compatibility
		// and capture stderr for debugging
		var stdout, stderr bytes.Buffer
		ollamaCmd := exec.Command("ollama", "list")
		ollamaCmd.Stdout = &stdout
		ollamaCmd.Stderr = &stderr

		// Configuration for the command execution
		ollamaHost := os.Getenv("OLLAMA_HOST")
		if cmd.Flag("host").Changed {
			ollamaHost = cmd.Flag("host").Value.String()
		}

		if ollamaHost != "" {
			// Set the OLLAMA_HOST environment variable for the command
			ollamaCmd.Env = append(os.Environ(), fmt.Sprintf("OLLAMA_HOST=%s", ollamaHost))
		}

		// Execute the command
		err := ollamaCmd.Run()
		if err != nil {
			return err
		}

		// Parse the output of ollama list (text format)
		modelsOutput := stdout.String()
		var modelNames []string

		if modelsOutput != "" {
			// Typical format:
			// NAME             ID            SIZE    MODIFIED
			// llama3           xxx...xxx     4.7 GB  X days ago

			// Skip the first line (headers)
			lines := strings.Split(modelsOutput, "\n")
			for i, line := range lines {
				if i == 0 || strings.TrimSpace(line) == "" {
					continue
				}

				fields := strings.Fields(line)
				if len(fields) >= 1 {
					modelNames = append(modelNames, fields[0])
				}
			}

			// Display models in our format
			if len(modelNames) > 0 {
				fmt.Println("\nAvailable models:")
				for i, name := range modelNames {
					fmt.Printf("  %d. %s\n", i+1, name)
				}

				// Allow the user to choose a model
				fmt.Print("\nChoose a model (number) or enter model name: ")
				modelChoice, _ := reader.ReadString('\n')
				modelChoice = strings.TrimSpace(modelChoice)

				// Check if the user entered a number
				var modelNumber int
				modelName = "" // Initialize here too

				if _, err := fmt.Sscanf(modelChoice, "%d", &modelNumber); err == nil {
					// The user entered a number
					if modelNumber > 0 && modelNumber <= len(modelNames) {
						modelName = modelNames[modelNumber-1]
					} else {
						fmt.Println("Invalid selection. Please enter a valid model name manually.")
					}
				} else {
					// The user entered a name directly
					modelName = modelChoice
				}
			}
		}

		// If no model was selected, ask manually
		if modelName == "" {
			fmt.Print("Enter model name [llama3]: ")
			inputName, _ := reader.ReadString('\n')
			inputName = strings.TrimSpace(inputName)
			if inputName == "" {
				modelName = "llama3"
			} else {
				modelName = inputName
			}
		}

		// New Step 3: Choose between local documents or website
		fmt.Println("\nStep 3: Choose document source")
		fmt.Println("1. Local document folder")
		fmt.Println("2. Crawl a website")
		fmt.Print("\nSelect option (1/2): ")
		sourceChoice, _ := reader.ReadString('\n')
		sourceChoice = strings.TrimSpace(sourceChoice)

		var folderPath string
		var websiteURL string
		var maxDepth, concurrency int
		var excludePaths []string
		var useWebCrawler bool
		var useSitemap bool

		if sourceChoice == "2" {
			// Website crawler option
			useWebCrawler = true

			// Ask for the website URL
			fmt.Print("\nEnter website URL to crawl: ")
			websiteURL, _ = reader.ReadString('\n')
			websiteURL = strings.TrimSpace(websiteURL)
			if websiteURL == "" {
				return fmt.Errorf("website URL cannot be empty")
			}

			// Maximum crawl depth
			fmt.Print("Maximum crawl depth [2]: ")
			depthStr, _ := reader.ReadString('\n')
			depthStr = strings.TrimSpace(depthStr)
			maxDepth = 2 // default value
			if depthStr != "" {
				fmt.Sscanf(depthStr, "%d", &maxDepth)
			}

			// Concurrency
			fmt.Print("Number of concurrent crawlers [5]: ")
			concurrencyStr, _ := reader.ReadString('\n')
			concurrencyStr = strings.TrimSpace(concurrencyStr)
			concurrency = 5 // default value
			if concurrencyStr != "" {
				fmt.Sscanf(concurrencyStr, "%d", &concurrency)
			}

			// Paths to exclude
			fmt.Print("Paths to exclude (comma-separated): ")
			excludePathsStr, _ := reader.ReadString('\n')
			excludePathsStr = strings.TrimSpace(excludePathsStr)
			if excludePathsStr != "" {
				excludePaths = strings.Split(excludePathsStr, ",")
				for i := range excludePaths {
					excludePaths[i] = strings.TrimSpace(excludePaths[i])
				}
			}

			// Ask about sitemap
			fmt.Print("Use sitemap.xml if available? (y/N): ")
			sitemapStr, _ := reader.ReadString('\n')
			sitemapStr = strings.ToLower(strings.TrimSpace(sitemapStr))
			useSitemap = sitemapStr == "y" || sitemapStr == "yes"

		} else {
			// Local folder option
			fmt.Print("\nEnter the path to your documents folder: ")
			folderPath, _ = reader.ReadString('\n')
			folderPath = strings.TrimSpace(folderPath)
			if folderPath == "" {
				return fmt.Errorf("folder path cannot be empty")
			}

			// Validate folder exists
			if _, err := os.Stat(folderPath); os.IsNotExist(err) {
				return fmt.Errorf("folder does not exist: %s", folderPath)
			}
		}

		// Step 4: Configure chunking
		fmt.Println("\nStep 4: Configure text chunking")
		fmt.Print("Chunk size (tokens) [512]: ")
		chunkSizeStr, _ := reader.ReadString('\n')
		chunkSizeStr = strings.TrimSpace(chunkSizeStr)
		chunkSize := 512 // default value
		if chunkSizeStr != "" {
			fmt.Sscanf(chunkSizeStr, "%d", &chunkSize)
		}

		fmt.Print("Chunk overlap (tokens) [50]: ")
		overlapStr, _ := reader.ReadString('\n')
		overlapStr = strings.TrimSpace(overlapStr)
		overlap := 50 // default value
		if overlapStr != "" {
			fmt.Sscanf(overlapStr, "%d", &overlap)
		}

		// Create the RAG system
		fmt.Println("\nCreating RAG system...")
		options := service.DocumentLoaderOptions{
			ChunkSize:    chunkSize,
			ChunkOverlap: overlap,
			ExcludeDirs:  excludePaths,
		}

		// Create the RAG service
		ollamaClient := GetOllamaClient()
		ragService := service.NewRagService(ollamaClient)

		// Create the RAG system
		if useWebCrawler {
			// Create a web crawler
			crawler, err := crawler.NewWebCrawler(websiteURL, maxDepth, concurrency, excludePaths)
			if err != nil {
				return fmt.Errorf("error creating web crawler: %w", err)
			}
			crawler.SetUseSitemap(useSitemap)

			// Crawl the website and get documents
			docs, err := crawler.CrawlWebsite()
			if err != nil {
				return fmt.Errorf("error crawling website: %w", err)
			}

			// Create a temporary directory for the crawled documents
			tempDir, err := os.MkdirTemp("", "rlama-web-*")
			if err != nil {
				return fmt.Errorf("error creating temporary directory: %w", err)
			}
			defer os.RemoveAll(tempDir)

			// Save the crawled documents to the temporary directory
			for i, doc := range docs {
				filePath := filepath.Join(tempDir, fmt.Sprintf("page_%d.md", i))
				err = os.WriteFile(filePath, []byte(doc.Content), 0644)
				if err != nil {
					return fmt.Errorf("error saving document: %w", err)
				}
			}

			// Create the RAG system with the crawled documents
			err = ragService.CreateRagWithOptions(modelName, ragName, tempDir, options)
			if err != nil {
				return fmt.Errorf("error creating RAG system: %w", err)
			}
		} else {
			// Create the RAG system with local documents
			err = ragService.CreateRagWithOptions(modelName, ragName, folderPath, options)
			if err != nil {
				return fmt.Errorf("error creating RAG system: %w", err)
			}
		}

		fmt.Printf("\n✨ RAG system '%s' has been created successfully! ✨\n", ragName)
		fmt.Println("\nYou can now use it with the following command:")
		fmt.Printf("  rlama run %s\n\n", ragName)

		return nil
	},
}

func init() {
	rootCmd.AddCommand(localWizardCmd)

	// Add flags for the local wizard
	localWizardCmd.Flags().StringVar(&localWizardModel, "model", "", "Model to use")
	localWizardCmd.Flags().StringVar(&localWizardName, "name", "", "Name of the RAG system")
	localWizardCmd.Flags().StringVar(&localWizardPath, "path", "", "Path to the documents folder")
	localWizardCmd.Flags().IntVar(&localWizardChunkSize, "chunk-size", 512, "Size of text chunks in tokens")
	localWizardCmd.Flags().IntVar(&localWizardChunkOverlap, "chunk-overlap", 50, "Overlap between chunks in tokens")
	localWizardCmd.Flags().StringSliceVar(&localWizardExcludeDirs, "exclude-dirs", []string{}, "Directories to exclude")
	localWizardCmd.Flags().StringSliceVar(&localWizardExcludeExts, "exclude-exts", []string{}, "File extensions to exclude")
	localWizardCmd.Flags().StringSliceVar(&localWizardProcessExts, "process-exts", []string{}, "File extensions to process")
}

func ExecuteWizard(out, errOut io.Writer) error {
	return localWizardCmd.Execute()
}

func NewWizardCommand() *cobra.Command {
	return localWizardCmd
}
