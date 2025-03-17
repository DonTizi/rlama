package cmd

import (
	"fmt"
	"strconv"
	"os"
	"io/ioutil"
	"encoding/json"
	"strings"
	"bytes"

	"github.com/spf13/cobra"
	"github.com/dontizi/rlama/internal/domain"
	"github.com/dontizi/rlama/internal/service"
)

var (
	webWatchMaxDepth     int
	webWatchConcurrency  int
	webWatchExcludePaths []string
	webWatchChunkSize    int
	webWatchChunkOverlap int
)

var webWatchCmd = &cobra.Command{
	Use:   "web-watch [rag-name] [website-url] [interval]",
	Short: "Set up website watching for a RAG system",
	Long: `Configure a RAG system to automatically watch a website for new content and add it to the RAG.
The interval is specified in minutes. Use 0 to only check when the RAG is used.

Example: rlama web-watch my-docs https://docs.example.com 60
This will check the website every 60 minutes for new content.

Use rlama web-watch-off [rag-name] to disable watching.`,
	Args: cobra.RangeArgs(2, 3),
	RunE: func(cmd *cobra.Command, args []string) error {
		ragName := args[0]
		websiteURL := args[1]
		
		// Default interval is 0 (only check when RAG is used)
		interval := 0
		
		// If an interval is provided, parse it
		if len(args) > 2 {
			var err error
			interval, err = strconv.Atoi(args[2])
			if err != nil {
				return fmt.Errorf("invalid interval: %s", args[2])
			}
			
			if interval < 0 {
				return fmt.Errorf("interval must be non-negative")
			}
		}
		
		// Get Ollama client from root command
		ollamaClient := GetOllamaClient()
		
		// Create RAG service
		ragService := service.NewRagService(ollamaClient)
		
		// Set up web watch options
		webWatchOptions := domain.WebWatchOptions{
			MaxDepth:     webWatchMaxDepth,
			Concurrency:  webWatchConcurrency,
			ExcludePaths: webWatchExcludePaths,
			ChunkSize:    webWatchChunkSize,
			ChunkOverlap: webWatchChunkOverlap,
		}
		
		// Set up website watching
		err := ragService.SetupWebWatching(ragName, websiteURL, interval, webWatchOptions)
		if err != nil {
			return err
		}
		
		// Provide feedback based on the interval
		if interval == 0 {
			fmt.Printf("Website watching set up for RAG '%s'. Website '%s' will be checked each time the RAG is used.\n", 
				ragName, websiteURL)
		} else {
			fmt.Printf("Website watching set up for RAG '%s'. Website '%s' will be checked every %d minutes.\n", 
				ragName, websiteURL, interval)
		}
		
		// Perform an initial check
		docsAdded, err := ragService.CheckWatchedWebsite(ragName)
		if err != nil {
			return fmt.Errorf("error during initial website check: %w", err)
		}
		
		if docsAdded > 0 {
			fmt.Printf("Added %d new pages from '%s'.\n", docsAdded, websiteURL)
		} else {
			fmt.Printf("No new content found at '%s'.\n", websiteURL)
		}
		
		return nil
	},
}

var webWatchOffCmd = &cobra.Command{
	Use:   "web-watch-off [rag-name]",
	Short: "Disable website watching for a RAG system",
	Long:  `Disable automatic website watching for a RAG system.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ragName := args[0]
		
		// Get Ollama client from root command
		ollamaClient := GetOllamaClient()
		
		// Create RAG service
		ragService := service.NewRagService(ollamaClient)
		
		// Disable website watching
		err := ragService.DisableWebWatching(ragName)
		if err != nil {
			return err
		}
		
		fmt.Printf("Website watching disabled for RAG '%s'.\n", ragName)
		return nil
	},
}

var checkWebWatchedCmd = &cobra.Command{
	Use:   "check-web-watched [rag-name]",
	Short: "Check a RAG's watched website for new content",
	Long:  `Manually check a RAG's watched website for new content and add it to the RAG.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ragName := args[0]
		
		// Get Ollama client from root command
		ollamaClient := GetOllamaClient()
		
		// Create RAG service
		ragService := service.NewRagService(ollamaClient)
		
		// Check the watched website
		pagesAdded, err := ragService.CheckWatchedWebsite(ragName)
		if err != nil {
			return err
		}
		
		if pagesAdded > 0 {
			fmt.Printf("Added %d new pages to RAG '%s'.\n", pagesAdded, ragName)
		} else {
			fmt.Printf("No new content found for RAG '%s'.\n", ragName)
		}
		
		return nil
	},
}

var processFileCmd = &cobra.Command{
	Use:   "process-file [rag-name] [file-path]",
	Short: "Traite un fichier comme s'il s'agissait d'une notification de surveillance web",
	Long: `Cette commande permet de simuler la réception d'une notification de surveillance web
en traitant directement un fichier local. Utile pour tester les workflows sans attendre
une vraie notification.`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		ragName := args[0]
		filePath := args[1]
		
		fmt.Printf("Traitement du fichier %s comme source pour le RAG %s...\n", filePath, ragName)
		
		// Vérifier que le fichier existe
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			return fmt.Errorf("le fichier %s n'existe pas", filePath)
		}
		
		// Lire le contenu du fichier
		fileData, err := ioutil.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("erreur lors de la lecture du fichier: %w", err)
		}
		
		// Obtenir les services nécessaires
		ollamaClient := GetOllamaClient()
		ragService := service.NewRagService(ollamaClient)
		agentService := service.NewAgentService(ollamaClient, ragService)
		fileWatcher := service.NewFileWatcher(ragService)
		
		// Analyser le contenu comme une notification GitHub
		var release struct {
			TagName string `json:"tag_name"`
			Name    string `json:"name"`
			Body    string `json:"body"`
		}
		
		if err := json.Unmarshal(fileData, &release); err != nil {
			return fmt.Errorf("erreur lors de l'analyse du fichier JSON: %w", err)
		}
		
		// Construire le texte à traiter
		releaseText := fmt.Sprintf("Nouvelle version: %s (%s)\n\n%s", 
			release.Name, release.TagName, release.Body)
		
		fmt.Println("Contenu de la release détecté:")
		fmt.Println("---------------------------------")
		fmt.Println(releaseText)
		fmt.Println("---------------------------------")
		
		// Vérifier si un crew est lié à ce RAG
		linkedCrews, err := fileWatcher.GetLinkedCrews(ragName)
		if err != nil {
			return fmt.Errorf("erreur lors de la récupération des crews liés: %w", err)
		}
		
		if len(linkedCrews) == 0 {
			return fmt.Errorf("aucun crew n'est lié au RAG %s", ragName)
		}
		
		// Traiter chaque crew lié
		for _, linked := range linkedCrews {
			fmt.Printf("Exécution du crew lié '%s' avec le modèle d'instruction: %s\n", 
				linked.CrewID, linked.InstructionTemplate)
			
			// Charger le crew
			crew, err := agentService.LoadCrew(linked.CrewID)
			if err != nil {
				fmt.Printf("Erreur lors du chargement du crew %s: %v\n", linked.CrewID, err)
				continue
			}
			
			// Remplacer les variables dans le modèle d'instruction
			instruction := strings.Replace(linked.InstructionTemplate, 
				"{RELEASE_DESCRIPTION}", releaseText, -1)
			instruction = strings.Replace(instruction, 
				"{INPUT_TEXT}", releaseText, -1)
			
			// Exécuter le crew avec l'instruction générée
			fmt.Printf("Exécution du crew '%s'...\n", crew.Name)
			results, err := agentService.RunCrew(crew, instruction)
			if err != nil {
				fmt.Printf("Erreur lors de l'exécution du crew: %v\n", err)
				continue
			}
			
			// Afficher les résultats
			fmt.Println("Résultats:")
			for agentID, result := range results {
				// Trouver le nom de l'agent pour une meilleure lisibilité
				agent, _ := agentService.LoadAgent(agentID)
				agentName := agentID
				if agent != nil {
					agentName = agent.Name
				}
				
				fmt.Printf("- Agent '%s':\n", agentName)
				
				// Essayer de formater le JSON pour une meilleure lisibilité
				var prettyJSON bytes.Buffer
				if err := json.Indent(&prettyJSON, []byte(result), "  ", "  "); err == nil {
					fmt.Printf("  %s\n", prettyJSON.String())
				} else {
					// Si le formatage JSON échoue, afficher le texte brut
					fmt.Printf("  %s\n", result)
				}
			}
		}
		
		fmt.Println("Traitement terminé!")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(webWatchCmd)
	rootCmd.AddCommand(webWatchOffCmd)
	rootCmd.AddCommand(checkWebWatchedCmd)
	rootCmd.AddCommand(processFileCmd)
	
	// Add web watching specific flags
	webWatchCmd.Flags().IntVar(&webWatchMaxDepth, "max-depth", 2, "Maximum crawl depth (default: 2)")
	webWatchCmd.Flags().IntVar(&webWatchConcurrency, "concurrency", 5, "Number of concurrent crawlers (default: 5)")
	webWatchCmd.Flags().StringSliceVar(&webWatchExcludePaths, "exclude-path", nil, "Paths to exclude from crawling (comma-separated)")
	webWatchCmd.Flags().IntVar(&webWatchChunkSize, "chunk-size", 1000, "Character count per chunk (default: 1000)")
	webWatchCmd.Flags().IntVar(&webWatchChunkOverlap, "chunk-overlap", 200, "Overlap between chunks in characters (default: 200)")
	
	// Nouvelle commande pour lier un agent à un watcher
	var linkAgentCmd = &cobra.Command{
		Use:   "link-agent [rag-name] [agent-id] [prompt]",
		Short: "Lie un agent à un RAG surveillé",
		Args:  cobra.ExactArgs(3),
		Run: func(cmd *cobra.Command, args []string) {
			// Utiliser les services globaux
			ragService := GetRagService()
			agentService := GetAgentService()
			
			ragName := args[0]
			agentID := args[1]
			prompt := args[2]
			
			outputDir, _ := cmd.Flags().GetString("output-dir")
			
			rag, err := ragService.LoadRag(ragName)
			if err != nil {
				fmt.Printf("Erreur lors du chargement du RAG: %v\n", err)
				os.Exit(1)
			}
			
			// Vérifier que le RAG a bien un watcher configuré
			if !rag.WebWatchEnabled && !rag.WatchEnabled {
				fmt.Println("Ce RAG n'a pas de surveillance configurée. Configurez-en une d'abord avec 'web-watch' ou 'dir-watch'.")
				os.Exit(1)
			}
			
			// Vérifier que l'agent existe
			agent, err := agentService.LoadAgent(agentID)
			if err != nil {
				fmt.Printf("Erreur lors du chargement de l'agent: %v\n", err)
				os.Exit(1)
			}
			
			// Configurer le lien
			rag.LinkedAgentID = agentID
			rag.LinkedCrewID = ""  // Désactiver un crew s'il était lié
			rag.LinkedPrompt = prompt
			rag.OutputDirectory = outputDir
			
			// Sauvegarder les modifications
			if err := ragService.UpdateRag(rag); err != nil {
				fmt.Printf("Erreur lors de la mise à jour du RAG: %v\n", err)
				os.Exit(1)
			}
			
			fmt.Printf("Agent '%s' lié au RAG '%s'. Il sera exécuté automatiquement lors des changements.\n", agent.Name, rag.Name)
		},
	}
	
	linkAgentCmd.Flags().String("output-dir", "agent-docs", "Dossier où seront sauvegardés les résultats")
	webWatchCmd.AddCommand(linkAgentCmd)
	
	// Nouvelle commande pour lier un crew à un watcher
	var linkCrewCmd = &cobra.Command{
		Use:   "link-crew [rag-name] [crew-id] [prompt]",
		Short: "Lie un crew à un RAG surveillé",
		Args:  cobra.ExactArgs(3),
		Run: func(cmd *cobra.Command, args []string) {
			// Utiliser les services globaux
			ragService := GetRagService()
			agentService := GetAgentService()
			
			ragName := args[0]
			crewID := args[1]
			prompt := args[2]
			
			outputDir, _ := cmd.Flags().GetString("output-dir")
			
			rag, err := ragService.LoadRag(ragName)
			if err != nil {
				fmt.Printf("Erreur lors du chargement du RAG: %v\n", err)
				os.Exit(1)
			}
			
			// Vérifier que le RAG a bien un watcher configuré
			if !rag.WebWatchEnabled && !rag.WatchEnabled {
				fmt.Println("Ce RAG n'a pas de surveillance configurée. Configurez-en une d'abord avec 'web-watch' ou 'dir-watch'.")
				os.Exit(1)
			}
			
			// Vérifier que le crew existe
			crew, err := agentService.LoadCrew(crewID)
			if err != nil {
				fmt.Printf("Erreur lors du chargement du crew: %v\n", err)
				os.Exit(1)
			}
			
			// Configurer le lien
			rag.LinkedAgentID = ""  // Désactiver un agent s'il était lié
			rag.LinkedCrewID = crewID
			rag.LinkedPrompt = prompt
			rag.OutputDirectory = outputDir
			
			// Sauvegarder les modifications
			if err := ragService.UpdateRag(rag); err != nil {
				fmt.Printf("Erreur lors de la mise à jour du RAG: %v\n", err)
				os.Exit(1)
			}
			
			fmt.Printf("Crew '%s' lié au RAG '%s'. Il sera exécuté automatiquement lors des changements.\n", crew.Name, rag.Name)
		},
	}
	
	linkCrewCmd.Flags().String("output-dir", "agent-docs", "Dossier où seront sauvegardés les résultats")
	webWatchCmd.AddCommand(linkCrewCmd)
	webWatchCmd.AddCommand(processFileCmd)
} 