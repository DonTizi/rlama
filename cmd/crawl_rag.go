package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dontizi/rlama/internal/crawler"
	"github.com/dontizi/rlama/internal/domain"
	"github.com/dontizi/rlama/internal/service"
	"github.com/spf13/cobra"
)

var (
	crawlMaxDepth         int
	crawlConcurrency      int
	crawlExcludePaths     []string
	crawlUseSitemap       bool
	crawlSingleURL        bool
	crawlURLsList         []string
	crawlChunkSize        int
	crawlChunkOverlap     int
	crawlChunkingStrategy string
)

var crawlRagCmd = &cobra.Command{
	Use:   "crawl-rag [model] [rag-name] [website-url]",
	Short: "Create a new RAG system from a website",
	Long: `Create a new RAG system by crawling a website and indexing its content.
Example: rlama crawl-rag llama3 mysite-rag https://example.com

The crawler will try to use the sitemap.xml if available for comprehensive coverage.
It will also follow links on the pages up to the specified depth.

You can exclude certain paths and control other crawling parameters:
  rlama crawl-rag llama3 my-docs https://docs.example.com --max-depth=2
  rlama crawl-rag llama3 blog-rag https://blog.example.com --exclude-path=/archive,/tags
  rlama crawl-rag llama3 site-rag https://site.com --use-sitemap=false  # Disable sitemap search`,
	Args: cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		modelName := args[0]
		ragName := args[1]
		websiteURL := args[2]

		// Get Ollama client with configured host and port
		ollamaClient := GetOllamaClient()
		if err := ollamaClient.CheckOllamaAndModel(modelName); err != nil {
			return err
		}

		// Create new crawler
		webCrawler, err := crawler.NewWebCrawler(websiteURL, crawlMaxDepth, crawlConcurrency, crawlExcludePaths)
		if err != nil {
			return fmt.Errorf("error initializing web crawler: %w", err)
		}

		// Define crawling options
		webCrawler.SetUseSitemap(crawlUseSitemap)
		webCrawler.SetSingleURLMode(crawlSingleURL)

		// If specific URL list, define it
		if len(crawlURLsList) > 0 {
			webCrawler.SetURLsList(crawlURLsList)
		}

		// Afficher le mode de crawling
		if len(crawlURLsList) > 0 {
			fmt.Printf("URLs list mode: crawling %d specific URLs\n", len(crawlURLsList))
		} else if crawlSingleURL {
			fmt.Println("Single URL mode: only the specified URL will be crawled (no links will be followed)")
		} else if crawlUseSitemap {
			fmt.Println("Sitemap mode enabled: will try to use sitemap.xml for comprehensive coverage")
		} else {
			fmt.Println("Standard crawling mode: will follow links to the specified depth")
		}

		// Display a message to indicate that the process has started
		fmt.Printf("Creating RAG '%s' with model '%s' by crawling website '%s'...\n",
			ragName, modelName, websiteURL)
		fmt.Printf("Max crawl depth: %d, Concurrency: %d\n", crawlMaxDepth, crawlConcurrency)

		// Start crawling
		documents, err := webCrawler.CrawlWebsite()
		if err != nil {
			return fmt.Errorf("error crawling website: %w", err)
		}

		if len(documents) == 0 {
			return fmt.Errorf("no content found when crawling %s", websiteURL)
		}

		fmt.Printf("Retrieved %d pages from website. Processing content...\n", len(documents))

		// Convertir []domain.Document en []*domain.Document
		var docPointers []*domain.Document
		for i := range documents {
			docPointers = append(docPointers, &documents[i])
		}

		// Create RAG service
		ragService := service.NewRagService(ollamaClient)

		// Set chunking options
		loaderOptions := service.DocumentLoaderOptions{
			ChunkSize:        crawlChunkSize,
			ChunkOverlap:     crawlChunkOverlap,
			ChunkingStrategy: crawlChunkingStrategy,
		}

		// Create temporary directory to store crawled content
		tempDir := createTempDirForDocuments(docPointers)
		if tempDir != "" {
			// Comment this line to prevent deletion
			// defer cleanupTempDir(tempDir)

			// Add this to clearly display the path
			fmt.Printf("\n📁 The markdown files are located in: %s\n", tempDir)
		}

		// Create RAG system
		err = ragService.CreateRagWithOptions(modelName, ragName, tempDir, loaderOptions)
		if err != nil {
			if strings.Contains(err.Error(), "connection refused") {
				return fmt.Errorf("⚠️ Unable to connect to Ollama.\n" +
					"Make sure Ollama is installed and running.\n")
			}
			return err
		}

		fmt.Printf("RAG '%s' created successfully with content from %s.\n", ragName, websiteURL)

		return nil
	},
}

func init() {
	rootCmd.AddCommand(crawlRagCmd)

	// Add local flags
	crawlRagCmd.Flags().IntVar(&crawlMaxDepth, "max-depth", 2, "Maximum crawl depth")
	crawlRagCmd.Flags().IntVar(&crawlConcurrency, "concurrency", 5, "Number of concurrent crawlers")
	crawlRagCmd.Flags().StringSliceVar(&crawlExcludePaths, "exclude-path", nil, "Paths to exclude from crawling (comma-separated)")
	crawlRagCmd.Flags().IntVar(&crawlChunkSize, "chunk-size", 1000, "Character count per chunk (default: 1000)")
	crawlRagCmd.Flags().IntVar(&crawlChunkOverlap, "chunk-overlap", 200, "Overlap between chunks in characters (default: 200)")
	crawlRagCmd.Flags().StringVar(&crawlChunkingStrategy, "chunking-strategy", "hybrid", "Chunking strategy to use (options: \"fixed\", \"semantic\", \"hybrid\", \"hierarchical\", \"auto\"). The \"auto\" strategy will analyze each document and apply the optimal strategy automatically.")
	crawlRagCmd.Flags().BoolVar(&crawlUseSitemap, "use-sitemap", true, "Use sitemap.xml if available for comprehensive coverage")
	crawlRagCmd.Flags().BoolVar(&crawlSingleURL, "single-url", false, "Process only the specified URL without following links")
	crawlRagCmd.Flags().StringSliceVar(&crawlURLsList, "urls-list", nil, "Provide a comma-separated list of specific URLs to crawl")
}

// Helper function to create a temporary directory and save crawled documents as files
func createTempDirForDocuments(documents []*domain.Document) string {
	// Create a temporary directory
	tempDir, err := os.MkdirTemp("", "rlama-crawl-*")
	if err != nil {
		fmt.Printf("Error creating temporary directory: %v\n", err)
		return ""
	}

	fmt.Printf("Created temporary directory for documents: %s\n", tempDir)

	// Save each document as a file in the temporary directory
	for i, doc := range documents {
		// Default to index-based filename
		filename := fmt.Sprintf("page_%d.md", i+1)

		// Try to use Path if available (more likely to exist than URL)
		if doc.Path != "" {
			// Create a safe filename from the Path
			safePath := strings.Map(func(r rune) rune {
				if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
					return r
				}
				return '-'
			}, doc.Path)

			// Trim leading/trailing dashes
			safePath = strings.Trim(safePath, "-")
			if safePath != "" {
				filename = fmt.Sprintf("%s.md", safePath)
			}
		}

		// Full path to the file
		filePath := filepath.Join(tempDir, filename)

		// Write the document content to the file
		err := os.WriteFile(filePath, []byte(doc.Content), 0644)
		if err != nil {
			fmt.Printf("Error writing document to file %s: %v\n", filePath, err)
			continue
		}
	}

	return tempDir
}

func cleanupTempDir(path string) {
	if path != "" {
		if err := os.RemoveAll(path); err != nil {
			fmt.Printf("Warning: Failed to clean up temporary directory %s: %v\n", path, err)
		}
	}
}
