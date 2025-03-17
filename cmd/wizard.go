package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/dontizi/rlama/internal/domain"
	"github.com/dontizi/rlama/internal/service"
	"github.com/spf13/cobra"
	"github.com/fatih/color"
)

var (
	autoRun    bool
	inputFile  string
	watchDir   string
)

var wizardCmd = &cobra.Command{
	Use:   "wizard",
	Short: "Crew creation wizard",
	Long:  `Interactive wizard to create and configure crews and their workflows.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		reader := bufio.NewReader(os.Stdin)
		
		// Terminal colors
		titleColor := color.New(color.FgCyan, color.Bold)
		promptColor := color.New(color.FgGreen)
		successColor := color.New(color.FgGreen, color.Bold)
		infoColor := color.New(color.FgYellow)
		
		titleColor.Println("\n=== RLAMA Crew Creation Wizard ===")
		fmt.Println()
		infoColor.Println("This wizard will guide you through creating a complete crew.")
		infoColor.Println("A crew is a group of agents organized in a workflow to accomplish complex tasks.")
		fmt.Println()

		// Create base crew
		crew, err := createCrewWizard(reader, promptColor)
		if err != nil {
			return err
		}

		// Add agents
		err = addAgentsWizard(reader, crew, promptColor, successColor)
		if err != nil {
			return err
		}

		// Configure workflow
		err = configureWorkflowWizard(reader, crew, promptColor, infoColor)
		if err != nil {
			return err
		}

		// Configure automation if requested
		if autoRun || watchDir != "" {
			err = configureAutomation(crew, watchDir, inputFile)
			if err != nil {
				return err
			}
		}

		// Save crew
		err = saveCrew(crew)
		if err != nil {
			return err
		}

		successColor.Printf("\n✅ Crew '%s' created successfully (ID: %s)!\n", crew.Name, crew.ID)
		
		if inputFile != "" {
			promptColor.Printf("\nProcessing input file: %s\n", inputFile)
			err = processInputFile(crew, inputFile)
			if err != nil {
				return err
			}
		}

		infoColor.Println("\nYou can run this crew with the command:")
		fmt.Printf("  rlama agent crew run %s \"your instruction\"\n", strings.ReplaceAll(crew.Name, " ", "-"))

		return nil
	},
}

func createCrewWizard(reader *bufio.Reader, promptColor *color.Color) (*domain.Crew, error) {
	promptColor.Print("Crew name (no spaces): ")
	crewName, err := reader.ReadString('\n')
	if err != nil {
		return nil, err
	}
	crewName = strings.TrimSpace(crewName)
	crewName = strings.ReplaceAll(crewName, " ", "-")

	promptColor.Print("Crew description (optional): ")
	description, err := reader.ReadString('\n')
	if err != nil {
		return nil, err
	}
	description = strings.TrimSpace(description)

	var workflowType domain.WorkflowType
	for {
		promptColor.Print("Workflow type (sequential/parallel) [sequential]: ")
		input, err := reader.ReadString('\n')
		if err != nil {
			return nil, err
		}
		
		inputStr := strings.TrimSpace(input)
		if inputStr == "" {
			workflowType = domain.WorkflowSequential
			break
		}
		
		switch inputStr {
		case "sequential":
			workflowType = domain.WorkflowSequential
			break
		case "parallel":
			workflowType = domain.WorkflowParallel
			break
		default:
			color.Red("❌ Invalid workflow type. Please enter 'sequential' or 'parallel'.")
			continue
		}
		break
	}

	// Create crew with current timestamp
	now := time.Now()
	crewID := fmt.Sprintf("crew_%d", now.UnixNano())
	
	crew := &domain.Crew{
		ID:          crewID,
		Name:        crewName,
		Description: description,
		CreatedAt:   now,
		UpdatedAt:   now,
		Agents:      []string{},
		Workflow: domain.Workflow{
			Type:  workflowType,
			Steps: []domain.WorkflowStep{},
		},
	}

	return crew, nil
}

// addAgentsWizard guide l'utilisateur à travers l'ajout d'agents au crew
func addAgentsWizard(reader *bufio.Reader, crew *domain.Crew, promptColor, successColor *color.Color) error {
	promptColor.Println("\n=== Add agents to crew ===")
	
	// Charger tous les agents disponibles
	availableAgents, err := getAgentFilesFromDisk()
	if err != nil {
		return fmt.Errorf("erreur lors du chargement des agents: %w", err)
	}
	
	if len(availableAgents) == 0 {
		return fmt.Errorf("aucun agent trouvé. Veuillez d'abord créer des agents avec 'rlama agent create'")
	}

	// Afficher les agents disponibles
	promptColor.Println("\nAvailable agents:")
	for i, agent := range availableAgents {
		fmt.Printf("%d. %s (%s)\n", i+1, agent.Name, agent.ID)
		if agent.Description != "" {
			fmt.Printf("   Description: %s\n", agent.Description)
		}
	}

	// Sélectionner les agents à ajouter
	promptColor.Println("\nEnter the numbers of agents to add, separated by commas (ex: 1,3,5)")
	promptColor.Print("Agents to add: ")
	input, err := reader.ReadString('\n')
	if err != nil {
		return err
	}
	
	selectedIndexes := strings.Split(strings.TrimSpace(input), ",")
	
	// Ajouter les agents sélectionnés
	for _, indexStr := range selectedIndexes {
		indexStr = strings.TrimSpace(indexStr)
		if indexStr == "" {
			continue
		}
		
		index, err := strconv.Atoi(indexStr)
		if err != nil {
			fmt.Printf("❌ Invalid number: %s, ignored\n", indexStr)
			continue
		}
		
		if index < 1 || index > len(availableAgents) {
			fmt.Printf("❌ Out of range: %d, ignored\n", index)
			continue
		}
		
		agent := availableAgents[index-1]
		crew.AddAgent(agent.ID)
		successColor.Printf("✅ Agent added: %s\n", agent.Name)
	}
	
	if len(crew.Agents) == 0 {
		return fmt.Errorf("no agents added to crew. At least one agent is required")
	}

	return nil
}

// configureWorkflowWizard guide l'utilisateur à travers la configuration du workflow
func configureWorkflowWizard(reader *bufio.Reader, crew *domain.Crew, promptColor, infoColor *color.Color) error {
	promptColor.Println("\n=== Configure workflow ===")
	
	// Si le workflow est séquentiel, demander des détails pour chaque étape
	if crew.Workflow.Type == "sequential" {
		promptColor.Println("\nYou are configuring a sequential workflow where agents execute one after another.")
		
		// Charger les agents ajoutés pour référence
		agentMap := make(map[string]*domain.Agent)
		for _, agentID := range crew.Agents {
			agent, err := getAgentByID(agentID)
			if err != nil {
				return err
			}
			agentMap[agentID] = agent
		}
		
		// Afficher les agents disponibles pour le workflow
		promptColor.Println("\nAvailable agents for the workflow:")
		for i, agentID := range crew.Agents {
			agent := agentMap[agentID]
			fmt.Printf("%d. %s (%s)\n", i+1, agent.Name, agent.ID)
		}
		
		// Configurer chaque étape du workflow
		for {
			promptColor.Println("\n--- New workflow step ---")
			
			// Sélectionner un agent pour cette étape
			var agentIndex int
			for {
				promptColor.Print("Number of the agent for this step: ")
				input, err := reader.ReadString('\n')
				if err != nil {
					return err
				}
				
				index, err := strconv.Atoi(strings.TrimSpace(input))
				if err != nil || index < 1 || index > len(crew.Agents) {
					promptColor.Println("❌ Invalid number. Please enter a number from the list.")
					continue
				}
				
				agentIndex = index - 1
				break
			}
			
			// Obtenir les instructions pour cette étape
			promptColor.Print("Instructions for this agent (empty = use main instruction): ")
			instructionInput, err := reader.ReadString('\n')
			if err != nil {
				return err
			}
			instruction := strings.TrimSpace(instructionInput)
			
			// Si ce n'est pas la première étape, demander les dépendances
			var dependsOn []int
			if len(crew.Workflow.Steps) > 0 {
				promptColor.Println("Previous steps:")
				for i, step := range crew.Workflow.Steps {
					agent := agentMap[step.AgentID]
					fmt.Printf("%d. %s\n", i, agent.Name)
				}
				
				promptColor.Print("Dependencies (numbers of steps separated by commas, empty = none): ")
				depsInput, err := reader.ReadString('\n')
				if err != nil {
					return err
				}
				
				if strings.TrimSpace(depsInput) != "" {
					depIndexes := strings.Split(strings.TrimSpace(depsInput), ",")
					for _, idxStr := range depIndexes {
						idx, err := strconv.Atoi(strings.TrimSpace(idxStr))
						if err == nil && idx >= 0 && idx < len(crew.Workflow.Steps) {
							dependsOn = append(dependsOn, idx)
						}
					}
				}
			}
			
			// Demander si la sortie doit être passée à l'étape suivante
			var outputToNext bool
			promptColor.Print("Pass output to next step? (y/n) [y]: ")
			outputInput, err := reader.ReadString('\n')
			if err != nil {
				return err
			}
			
			outputStr := strings.TrimSpace(outputInput)
			if outputStr == "" || strings.ToLower(outputStr) == "y" || strings.ToLower(outputStr) == "yes" {
				outputToNext = true
			}
			
			// Ajouter l'étape
			step := domain.WorkflowStep{
				AgentID:      crew.Agents[agentIndex],
				Instruction:  instruction,
				DependsOn:    dependsOn,
				OutputToNext: outputToNext,
			}
			
			crew.Workflow.Steps = append(crew.Workflow.Steps, step)
			
			// Demander s'il faut ajouter une autre étape
			promptColor.Print("\nAdd another step? (y/n) [n]: ")
			continueInput, err := reader.ReadString('\n')
			if err != nil {
				return err
			}
			
			continueStr := strings.TrimSpace(continueInput)
			if continueStr != "y" && continueStr != "yes" {
				break
			}
		}
	} else if crew.Workflow.Type == "parallel" {
		// Workflow parallèle
		promptColor.Println("\nYou are configuring a parallel workflow where all agents execute simultaneously.")
		
		// Charger les agents pour référence
		agentMap := make(map[string]*domain.Agent)
		for _, agentID := range crew.Agents {
			agent, err := getAgentByID(agentID)
			if err != nil {
				return err
			}
			agentMap[agentID] = agent
		}
		
		// Ajouter automatiquement tous les agents au workflow
		for _, agentID := range crew.Agents {
			agent := agentMap[agentID]
			
			// Pour chaque agent, demander des instructions spécifiques
			promptColor.Printf("\nInstructions for the agent %s (empty = use main instruction): ", agent.Name)
			instructionInput, err := reader.ReadString('\n')
			if err != nil {
				return err
			}
			instruction := strings.TrimSpace(instructionInput)
			
			// Ajouter l'étape
			step := domain.WorkflowStep{
				AgentID:     agentID,
				Instruction: instruction,
				DependsOn:   []int{}, // Pas de dépendances dans un workflow parallèle
			}
			
			crew.Workflow.Steps = append(crew.Workflow.Steps, step)
		}
	}
	
	return nil
}

// saveCrew sauvegarde le crew dans un fichier JSON
func saveCrew(crew *domain.Crew) error {
	// Préparer le dossier agents
	basePath := filepath.Join(os.Getenv("HOME"), ".rlama", "agents")
	os.MkdirAll(basePath, 0755)
	
	// Chemin du fichier
	crewPath := filepath.Join(basePath, crew.ID+".json")
	
	// Convertir en JSON
	crewData, err := json.MarshalIndent(crew, "", "  ")
	if err != nil {
		return fmt.Errorf("erreur lors de la sérialisation du crew: %w", err)
	}
	
	// Écrire le fichier
	err = ioutil.WriteFile(crewPath, crewData, 0644)
	if err != nil {
		return fmt.Errorf("erreur lors de l'écriture du crew: %w", err)
	}
	
	return nil
}

func configureAutomation(crew *domain.Crew, watchDir, inputFile string) error {
	if watchDir != "" {
		// Configure file watching in RAG settings
		crew.RAGName = filepath.Base(watchDir)
		crew.InstructionTemplate = "{INPUT_TEXT}" // Default template
	}
	
	return nil
}

func processInputFile(crew *domain.Crew, inputFile string) error {
	// Read and process the input file
	data, err := ioutil.ReadFile(inputFile)
	if err != nil {
		return fmt.Errorf("error reading input file: %w", err)
	}
	
	// Get services from root command
	ollamaClient := GetOllamaClient()
	ragService := service.NewRagService(ollamaClient)
	agentService := service.NewAgentService(ollamaClient, ragService)
	
	// Execute the crew with the file content
	results, err := agentService.RunCrew(crew, string(data))
	if err != nil {
		return fmt.Errorf("error executing crew: %w", err)
	}
	
	// Display results
	color.Green("\nExecution Results:")
	for agentID, result := range results {
		agent, _ := agentService.LoadAgent(agentID)
		agentName := agentID
		if agent != nil {
			agentName = agent.Name
		}
		
		color.Cyan("\n=== %s ===", agentName)
		fmt.Println(result)
	}
	
	return nil
}

func init() {
	wizardCmd.Flags().BoolVar(&autoRun, "auto-run", false, "Enable automatic execution on file changes")
	wizardCmd.Flags().StringVar(&watchDir, "watch-dir", "", "Directory to watch for new files")
	wizardCmd.Flags().StringVar(&inputFile, "input-file", "", "Process a specific input file")
	agentCmd.AddCommand(wizardCmd) 
} 