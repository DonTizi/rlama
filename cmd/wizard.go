package cmd

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/dontizi/rlama/internal/crawler"
	"github.com/dontizi/rlama/internal/domain"
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

// Renamed to avoid conflict with snowflake_wizard.go

var localWizardCmd = &cobra.Command{
	Use:   "wizard",
	Short: "Interactive wizard to create a local RAG",
	Long: `Start an interactive wizard that guides you through creating a RAG system.
This makes it easy to set up a new RAG without remembering all command options.`,
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
			fmt.Println("❌ Failed to list Ollama models.")
			if stderr.Len() > 0 {
				fmt.Printf("Error details: %s\n", stderr.String())
			}
			fmt.Println("Make sure Ollama is installed and running.")
			fmt.Println("Continuing without model list. You'll need to enter a model name manually.")
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
		var crawlSingleURL bool
		var crawlURLsList []string

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

			// Ask if the user wants to use the sitemap
			useSitemapPrompt := &survey.Confirm{
				Message: "Use sitemap.xml if available (recommended for better coverage)?",
				Default: true,
			}
			err = survey.AskOne(useSitemapPrompt, &useSitemap)
			if err != nil {
				return err
			}

			// Display crawling configuration
			fmt.Println("\nCrawling configuration:")
			fmt.Printf("- URL: %s\n", websiteURL)
			fmt.Printf("- Max depth: %d\n", maxDepth)
			fmt.Printf("- Concurrency: %d\n", concurrency)
			if len(excludePaths) > 0 {
				fmt.Printf("- Excluded paths: %s\n", strings.Join(excludePaths, ", "))
			}
			fmt.Printf("- Use sitemap: %t\n", useSitemap)
			fmt.Printf("- Single URL mode: %t\n", crawlSingleURL)
			if len(crawlURLsList) > 0 {
				fmt.Printf("- Specific URLs: %s\n", strings.Join(crawlURLsList, ", "))
			}

			// Ask if the user wants to use single URL mode
			useSingleURLPrompt := &survey.Confirm{
				Message: "Use single URL mode (only crawl the specified URL without following links)?",
				Default: false,
			}
			err = survey.AskOne(useSingleURLPrompt, &crawlSingleURL)
			if err != nil {
				return err
			}

			// Ask for a list of specific URLs to crawl
			if !crawlSingleURL {
				fmt.Print("Enter specific URLs to crawl (comma-separated, leave empty to crawl all): ")
				urlsListStr, _ := reader.ReadString('\n')
				urlsListStr = strings.TrimSpace(urlsListStr)
				if urlsListStr != "" {
					urlsList := strings.Split(urlsListStr, ",")
					for i := range urlsList {
						urlsList[i] = strings.TrimSpace(urlsList[i])
					}
					crawlURLsList = urlsList
				}
			}
		} else {
			// Option local folder (existing code)
			useWebCrawler = false

			fmt.Print("\nEnter path to document folder: ")
			folderPath, _ = reader.ReadString('\n')
			folderPath = strings.TrimSpace(folderPath)
			if folderPath == "" {
				return fmt.Errorf("folder path cannot be empty")
			}
		}

		// Step 4: Chunking options
		fmt.Println("\nStep 4: Chunking options")

		// Add chunking strategy selection
		fmt.Println("\nChunking strategies:")
		fmt.Println("  auto     - Automatically selects the best strategy for each document")
		fmt.Println("  fixed    - Splits text into fixed-size chunks")
		fmt.Println("  semantic - Respects natural boundaries like paragraphs")
		fmt.Println("  hybrid   - Adapts strategy based on document type")
		fmt.Println("  hierarchical - Creates two-level structure for long documents")

		fmt.Print("Chunking strategy [auto]: ")
		chunkingStrategyStr, _ := reader.ReadString('\n')
		chunkingStrategyStr = strings.TrimSpace(chunkingStrategyStr)
		chunkingStrategy := "auto"
		if chunkingStrategyStr != "" {
			chunkingStrategy = chunkingStrategyStr
		}

		fmt.Print("Chunk size [1000]: ")
		chunkSizeStr, _ := reader.ReadString('\n')
		chunkSizeStr = strings.TrimSpace(chunkSizeStr)
		chunkSize := 1000
		if chunkSizeStr != "" {
			fmt.Sscanf(chunkSizeStr, "%d", &chunkSize)
		}

		fmt.Print("Chunk overlap [200]: ")
		overlapStr, _ := reader.ReadString('\n')
		overlapStr = strings.TrimSpace(overlapStr)
		overlap := 200
		if overlapStr != "" {
			fmt.Sscanf(overlapStr, "%d", &overlap)
		}

		// Step 5: File filtering (optional)
		fmt.Println("\nStep 5: File filtering (optional)")

		fmt.Print("Exclude directories (comma-separated): ")
		excludeDirsStr, _ := reader.ReadString('\n')
		excludeDirsStr = strings.TrimSpace(excludeDirsStr)
		var excludeDirs []string
		if excludeDirsStr != "" {
			excludeDirs = strings.Split(excludeDirsStr, ",")
			for i := range excludeDirs {
				excludeDirs[i] = strings.TrimSpace(excludeDirs[i])
			}
		}

		fmt.Print("Exclude extensions (comma-separated): ")
		excludeExtsStr, _ := reader.ReadString('\n')
		excludeExtsStr = strings.TrimSpace(excludeExtsStr)
		var excludeExts []string
		if excludeExtsStr != "" {
			excludeExts = strings.Split(excludeExtsStr, ",")
			for i := range excludeExts {
				excludeExts[i] = strings.TrimSpace(excludeExts[i])
			}
		}

		fmt.Print("Process only these extensions (comma-separated): ")
		processExtsStr, _ := reader.ReadString('\n')
		processExtsStr = strings.TrimSpace(processExtsStr)
		var processExts []string
		if processExtsStr != "" {
			processExts = strings.Split(processExtsStr, ",")
			for i := range processExts {
				processExts[i] = strings.TrimSpace(processExts[i])
			}
		}

		// Step 6: Confirmation and creation
		fmt.Println("\nStep 6: Review and create")
		fmt.Println("RAG configuration:")
		fmt.Printf("- Name: %s\n", ragName)
		fmt.Printf("- Model: %s\n", modelName)

		if useWebCrawler {
			fmt.Printf("- Source: Website - %s\n", websiteURL)
			fmt.Printf("- Crawl depth: %d\n", maxDepth)
			fmt.Printf("- Concurrency: %d\n", concurrency)
			fmt.Printf("- Use sitemap: %t\n", useSitemap)
			fmt.Printf("- Single URL mode: %t\n", crawlSingleURL)
			if len(excludePaths) > 0 {
				fmt.Printf("- Exclude paths: %s\n", strings.Join(excludePaths, ", "))
			}
			if len(crawlURLsList) > 0 {
				fmt.Printf("- Specific URLs: %s\n", strings.Join(crawlURLsList, ", "))
			}
		} else {
			fmt.Printf("- Source: Local folder - %s\n", folderPath)
			if len(excludeDirs) > 0 {
				fmt.Printf("- Exclude directories: %s\n", strings.Join(excludeDirs, ", "))
			}
			if len(excludeExts) > 0 {
				fmt.Printf("- Exclude extensions: %s\n", strings.Join(excludeExts, ", "))
			}
			if len(processExts) > 0 {
				fmt.Printf("- Process only: %s\n", strings.Join(processExts, ", "))
			}
		}

		fmt.Printf("- Chunk size: %d\n", chunkSize)
		fmt.Printf("- Chunk overlap: %d\n", overlap)
		fmt.Printf("- Chunking strategy: %s\n", chunkingStrategy)

		fmt.Print("\nCreate RAG with these settings? (y/n): ")
		confirm, _ := reader.ReadString('\n')
		confirm = strings.ToLower(strings.TrimSpace(confirm))

		if confirm != "y" && confirm != "yes" {
			fmt.Println("RAG creation cancelled.")
			return nil
		}

		// Create the RAG
		fmt.Println("\nCreating RAG...")

		// Get the configured Ollama client
		ollamaClient := GetOllamaClient()

		// Check that the model is available before continuing
		// This step is important to avoid errors later
		fmt.Printf("Checking if model '%s' is available...\n", modelName)
		err = ollamaClient.CheckOllamaAndModel(modelName)
		if err != nil {
			return fmt.Errorf("model '%s' is not available: %w", modelName, err)
		}

		// Use RagService to create the RAG
		ragService := service.NewRagService(ollamaClient)

		if useWebCrawler {
			// Use the crawler
			fmt.Printf("\nCrawling website '%s'...\n", websiteURL)

			// Create the crawler
			webCrawler, err := crawler.NewWebCrawler(websiteURL, maxDepth, concurrency, excludePaths)
			if err != nil {
				return fmt.Errorf("error initializing web crawler: %w", err)
			}

			// Set the single URL option
			webCrawler.SetSingleURLMode(crawlSingleURL)

			// Set the sitemap option
			webCrawler.SetUseSitemap(useSitemap)

			// Set specific URLs list if provided
			if len(crawlURLsList) > 0 {
				webCrawler.SetURLsList(crawlURLsList)
				fmt.Printf("Using list of %d specific URLs for crawling\n", len(crawlURLsList))
			}

			// Start the crawling
			documents, err := webCrawler.CrawlWebsite()
			if err != nil {
				return fmt.Errorf("error crawling website: %w", err)
			}

			if len(documents) == 0 {
				return fmt.Errorf("no content found when crawling %s", websiteURL)
			}

			fmt.Printf("Retrieved %d pages from website. Processing content...\n", len(documents))

			// Convert documents to pointers before calling createTempDirForDocuments
			var docPointers []*domain.Document
			for i := range documents {
				docPointers = append(docPointers, &documents[i])
			}

			// Create a temporary directory for the documents
			tempDir := createTempDirForDocuments(docPointers)
			if tempDir != "" {
				defer cleanupTempDir(tempDir)
			}

			// Options for the document loader
			loaderOptions := service.DocumentLoaderOptions{
				ChunkSize:        chunkSize,
				ChunkOverlap:     overlap,
				ChunkingStrategy: chunkingStrategy,
				EnableReranker:   true,
			}

			// Create the RAG
			err = ragService.CreateRagWithOptions(modelName, ragName, tempDir, loaderOptions)
			if err != nil {
				return err
			}
		} else {
			// Use the local folder (existing code)
			loaderOptions := service.DocumentLoaderOptions{
				ExcludeDirs:      excludeDirs,
				ExcludeExts:      excludeExts,
				ProcessExts:      processExts,
				ChunkSize:        chunkSize,
				ChunkOverlap:     overlap,
				ChunkingStrategy: chunkingStrategy,
				EnableReranker:   true,
			}

			err = ragService.CreateRagWithOptions(modelName, ragName, folderPath, loaderOptions)
			if err != nil {
				return err
			}
		}

		fmt.Println("\n🎉 RAG created successfully! 🎉")
		fmt.Printf("\nYou can now use your RAG with: rlama run %s\n", ragName)

		// Ask if the user wants to set up directory watching
		if !useWebCrawler {
			// For local folder RAGs, offer directory watching
			setupDirWatchPrompt := &survey.Confirm{
				Message: "Do you want to set up directory watching to automatically update the RAG when files change?",
				Default: false,
			}

			var setupDirWatch bool
			if err := survey.AskOne(setupDirWatchPrompt, &setupDirWatch); err == nil && setupDirWatch {
				// Ask for the interval
				var watchInterval int
				intervalPrompt := &survey.Input{
					Message: "Check interval in minutes (0 = only check when RAG is used):",
					Default: "60",
				}

				var intervalStr string
				if err := survey.AskOne(intervalPrompt, &intervalStr); err == nil {
					fmt.Sscanf(intervalStr, "%d", &watchInterval)

					// Set up directory watching with appropriate options
					watchLoaderOptions := service.DocumentLoaderOptions{
						ExcludeDirs:      excludeDirs,
						ExcludeExts:      excludeExts,
						ProcessExts:      processExts,
						ChunkSize:        chunkSize,
						ChunkOverlap:     overlap,
						ChunkingStrategy: chunkingStrategy,
					}

					// Set up directory watching
					err := ragService.SetupDirectoryWatching(ragName, folderPath, watchInterval, watchLoaderOptions)
					if err != nil {
						fmt.Printf("Warning: Failed to set up directory watching: %v\n", err)
					} else {
						if watchInterval == 0 {
							fmt.Printf("Directory watching set up for '%s'. Directory '%s' will be checked each time the RAG is used.\n",
								ragName, folderPath)
						} else {
							fmt.Printf("Directory watching set up for '%s'. Directory '%s' will be checked every %d minutes.\n",
								ragName, folderPath, watchInterval)
						}
					}
				}
			}
		} else {
			// For web crawler RAGs, offer web watching
			setupWebWatchPrompt := &survey.Confirm{
				Message: "Do you want to set up website watching to automatically update the RAG when website content changes?",
				Default: false,
			}

			var setupWebWatch bool
			if err := survey.AskOne(setupWebWatchPrompt, &setupWebWatch); err == nil && setupWebWatch {
				// Ask for the interval
				var watchInterval int
				intervalPrompt := &survey.Input{
					Message: "Check interval in minutes (0 = only check when RAG is used):",
					Default: "60",
				}

				var intervalStr string
				if err := survey.AskOne(intervalPrompt, &intervalStr); err == nil {
					fmt.Sscanf(intervalStr, "%d", &watchInterval)

					// Set up web watching options
					webWatchOptions := domain.WebWatchOptions{
						MaxDepth:     maxDepth,
						Concurrency:  concurrency,
						ExcludePaths: excludePaths,
						ChunkSize:    chunkSize,
						ChunkOverlap: overlap,
					}

					// Set up web watching
					err := ragService.SetupWebWatching(ragName, websiteURL, watchInterval, webWatchOptions)
					if err != nil {
						fmt.Printf("Warning: Failed to set up website watching: %v\n", err)
					} else {
						if watchInterval == 0 {
							fmt.Printf("Website watching set up for '%s'. Website '%s' will be checked each time the RAG is used.\n",
								ragName, websiteURL)
						} else {
							fmt.Printf("Website watching set up for '%s'. Website '%s' will be checked every %d minutes.\n",
								ragName, websiteURL, watchInterval)
						}
					}
				}
			}
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(localWizardCmd)
}

func ExecuteWizard(out, errOut io.Writer) error {
	cmd := NewWizardCommand()
	cmd.SetOut(out)
	cmd.SetErr(errOut)
	return cmd.Execute()
}

func NewWizardCommand() *cobra.Command {
	return localWizardCmd
}
