package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/dontizi/rlama/internal/domain"
	"github.com/dontizi/rlama/internal/service"
	"github.com/spf13/cobra"
)

var (
	agentDescription string
	agentRole        string
	agentModelName   string
	agentRagName     string
	forceDeleteAgent bool
)

var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Manage AI agents",
	Long: `Create and manage AI agents that can perform tasks autonomously
or collaborate with other agents to solve complex problems.`,
}

var createAgentCmd = &cobra.Command{
	Use:   "create [agent-name]",
	Short: "Create a new agent",
	Long: `Create a new AI agent with a specific name, role, and model.
Example: rlama agent create researcher-agent --role researcher --model llama3 --rag my-docs
	
Available roles include:
  - researcher: Focuses on information retrieval and analysis
  - coder: Specializes in code generation and analysis
  - analyst: Good at processing numerical data and calculations`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Début de la création de l'agent...")
		
		// Vérifier la connexion avec timeout court
		if err := checkOllamaConnectionWithTimeout(5); err != nil {
			return fmt.Errorf("erreur de connexion à Ollama: %w", err)
		}
		
		agentName := args[0]

		// Obtenir les clients
		ollamaClient := GetOllamaClient()
		ragService := service.NewRagService(ollamaClient)
		agentService := service.NewAgentService(ollamaClient, ragService)

		// Vérifier et ajuster les valeurs par défaut si nécessaire
		if agentRole == "" {
			agentRole = "assistant" // Rôle par défaut
		}

		if agentModelName == "" {
			// Demander à l'utilisateur de sélectionner un modèle
			fmt.Println("Select a model for the agent:")
			models, err := ollamaClient.ListModels()
			if err != nil {
				return fmt.Errorf("error listing models: %w", err)
			}

			for i, model := range models {
				fmt.Printf("%d. %s\n", i+1, model)
			}

			fmt.Print("Enter model number: ")
			var choice int
			fmt.Scanln(&choice)
			if choice <= 0 || choice > len(models) {
				return fmt.Errorf("invalid model selection")
			}
			agentModelName = models[choice-1]
		}

		// Si la description n'est pas fournie, demander
		if agentDescription == "" {
			fmt.Print("Enter a description for the agent: ")
			reader := bufio.NewReader(os.Stdin)
			agentDescription, _ = reader.ReadString('\n')
			agentDescription = strings.TrimSpace(agentDescription)
		}

		// Créer l'agent
		agent, err := agentService.CreateAgent(agentName, agentDescription, agentRole, agentModelName, agentRagName)
		if err != nil {
			return fmt.Errorf("error creating agent: %w", err)
		}

		fmt.Printf("Agent '%s' created successfully with ID: %s\n", agent.Name, agent.ID)
		return nil
	},
}

var listAgentsCmd = &cobra.Command{
	Use:   "list",
	Short: "List all available agents",
	Long:  `Display a list of all agents that have been created.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Chargement de la liste des agents...")
		
		// Recupérer directement les fichiers agents plutôt que d'utiliser le service
		agentFiles, err := getAgentFilesFromDisk()
		if err != nil {
			return fmt.Errorf("erreur lors de la lecture des fichiers agents: %w", err)
		}
		
		if len(agentFiles) == 0 {
			fmt.Println("Aucun agent trouvé.")
			return nil
		}
		
		fmt.Printf("Agents disponibles (%d trouvés):\n\n", len(agentFiles))
		
		// Utiliser tabwriter pour un affichage aligné
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tNAME\tROLE\tMODEL\tRAG")
		
		for _, agent := range agentFiles {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
				agent.ID, agent.Name, agent.Role, agent.ModelName, 
				defaultString(agent.RAGName, "none"))
		}
		w.Flush()
		
		return nil
	},
}

var deleteAgentCmd = &cobra.Command{
	Use:   "delete [agent-id]",
	Short: "Delete an agent",
	Long:  `Permanently delete an agent by its ID.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		agentID := args[0]

		// Obtenir les clients
		ollamaClient := GetOllamaClient()
		ragService := service.NewRagService(ollamaClient)
		agentService := service.NewAgentService(ollamaClient, ragService)

		// Demander confirmation si --force n'est pas spécifié
		if !forceDeleteAgent {
			fmt.Printf("Are you sure you want to permanently delete agent '%s'? (y/n): ", agentID)
			var response string
			fmt.Scanln(&response)

			response = strings.ToLower(strings.TrimSpace(response))
			if response != "y" && response != "yes" {
				fmt.Println("Deletion cancelled.")
				return nil
			}
		}

		// Supprimer l'agent
		err := agentService.DeleteAgent(agentID)
		if err != nil {
			return fmt.Errorf("error deleting agent: %w", err)
		}

		fmt.Printf("Agent '%s' has been successfully deleted.\n", agentID)
		return nil
	},
}

var agentRunCmd = &cobra.Command{
	Use:   "run [agent-id] [instruction]",
	Short: "Run an agent with a given instruction",
	Long: `Run an agent with a given instruction.
Example: rlama agent run twitter-writer "Write a tweet about AI"`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Exécution de l'agent...")
		agentID := args[0]

		// Construire l'instruction à partir des arguments restants
		var instruction string
		if len(args) > 1 {
			instruction = strings.Join(args[1:], " ")
		} else {
			// Si aucune instruction n'est fournie, en demander une
			fmt.Print("Enter instruction: ")
			scanner := bufio.NewReader(os.Stdin)
			instructionInput, _ := scanner.ReadString('\n')
			instruction = strings.TrimSpace(instructionInput)
		}
		
		if instruction == "" {
			return fmt.Errorf("instruction cannot be empty")
		}

		// Vérifier la connexion avec timeout court
		if err := checkOllamaConnectionWithTimeout(5); err != nil {
			return fmt.Errorf("erreur de connexion à Ollama: %w", err)
		}

		// Créer un contexte avec timeout
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		// Canal pour recevoir le résultat
		resultChan := make(chan string, 1)
		errChan := make(chan error, 1)

		go func() {
			// Charger l'agent
			agent, err := getAgentByID(agentID)
			if err != nil {
				errChan <- fmt.Errorf("erreur lors du chargement de l'agent: %w", err)
				return
			}

			// Obtenir les clients
			ollamaClient := GetOllamaClient()
			ragService := service.NewRagService(ollamaClient)
			agentService := service.NewAgentService(ollamaClient, ragService)

			// Exécuter l'agent
			fmt.Printf("Executing agent '%s' with model '%s'...\n", agent.Name, agent.ModelName)
			result, err := agentService.RunAgent(agent, instruction)
			if err != nil {
				errChan <- fmt.Errorf("error running agent: %w", err)
				return
			}

			// Afficher le résultat
			resultChan <- result
		}()

		// Attendre le résultat ou le timeout
		select {
		case result := <-resultChan:
			fmt.Println("\n--- Result ---")
			fmt.Println(result)
			return nil
		case err := <-errChan:
			return err
		case <-ctx.Done():
			return fmt.Errorf("timeout: l'exécution de l'agent a pris trop de temps")
		}
	},
}

var crewCmd = &cobra.Command{
	Use:   "crew",
	Short: "Manage agent crews",
	Long: `Create and manage crews of agents that work together.
Crews allow multiple agents to collaborate on solving complex problems.`,
}

var createCrewCmd = &cobra.Command{
	Use:   "create [crew-name]",
	Short: "Create a new crew",
	Long: `Create a new crew of agents with a specified workflow type.
Example: rlama agent crew create research-team --workflow sequential`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		crewName := args[0]
		workflowFlag, _ := cmd.Flags().GetString("workflow")
		description, _ := cmd.Flags().GetString("description")

		// Déterminer le type de workflow
		var workflowType domain.WorkflowType
		switch strings.ToLower(workflowFlag) {
		case "sequential":
			workflowType = domain.WorkflowSequential
		case "parallel":
			workflowType = domain.WorkflowParallel
		default:
			return fmt.Errorf("invalid workflow type: %s (must be 'sequential' or 'parallel')", workflowFlag)
		}

		// Obtenir les clients
		ollamaClient := GetOllamaClient()
		ragService := service.NewRagService(ollamaClient)
		agentService := service.NewAgentService(ollamaClient, ragService)

		// Si la description n'est pas fournie, demander
		if description == "" {
			fmt.Print("Enter a description for the crew: ")
			reader := bufio.NewReader(os.Stdin)
			description, _ = reader.ReadString('\n')
			description = strings.TrimSpace(description)
		}

		// Créer le crew
		crew, err := agentService.CreateCrew(crewName, description, workflowType)
		if err != nil {
			return fmt.Errorf("error creating crew: %w", err)
		}

		fmt.Printf("Crew '%s' created successfully with ID: %s\n", crew.Name, crew.ID)
		fmt.Println("Use 'rlama agent crew add-agent' to add agents to this crew.")
		return nil
	},
}

var agentCrewAddAgentCmd = &cobra.Command{
	Use:   "add-agent [crew-name] [agent-name]",
	Short: "Add an agent to a crew",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Ajout de l'agent au crew (avec timeout)...")
		crewName := args[0]
		agentName := args[1]
		
		// Utiliser un timeout global pour l'opération complète
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		
		// Canal pour recevoir le résultat
		resultChan := make(chan error, 1)
		
		go func() {
			// Charger le crew directement depuis le disque
			crew, err := getCrewByName(crewName)
			if err != nil {
				resultChan <- fmt.Errorf("erreur lors du chargement du crew: %w", err)
				return
			}
			
			// Charger l'agent directement depuis le disque
			agent, err := getAgentByName(agentName)
			if err != nil {
				resultChan <- fmt.Errorf("erreur lors du chargement de l'agent: %w", err)
				return
			}
			
			// Ajouter l'agent au crew directement
			if !containsString(crew.Agents, agent.ID) {
				// Ajouter l'agent s'il n'est pas déjà présent
				crew.Agents = append(crew.Agents, agent.ID)
				
				// Sauvegarder le crew modifié
				basePath := filepath.Join(os.Getenv("HOME"), ".rlama", "agents")
				crewPath := filepath.Join(basePath, crew.ID+".json")
				
				// Convertir en JSON
				crewData, err := json.MarshalIndent(crew, "", "  ")
				if err != nil {
					resultChan <- fmt.Errorf("erreur lors de la sérialisation du crew: %w", err)
					return
				}
				
				// Écrire le fichier
				err = ioutil.WriteFile(crewPath, crewData, 0644)
				if err != nil {
					resultChan <- fmt.Errorf("erreur lors de l'écriture du crew: %w", err)
					return
				}
				
				resultChan <- nil
			} else {
				resultChan <- fmt.Errorf("l'agent '%s' est déjà dans le crew '%s'", agentName, crewName)
			}
		}()
		
		// Attendre le résultat ou le timeout
		select {
		case err := <-resultChan:
			if err != nil {
				return err
			}
			fmt.Printf("Agent '%s' ajouté avec succès au crew '%s'\n", agentName, crewName)
			return nil
		case <-ctx.Done():
			return fmt.Errorf("timeout: l'opération a pris trop de temps")
		}
	},
}

var addWorkflowStepCmd = &cobra.Command{
	Use:   "add-step [crew-id] [agent-id]",
	Short: "Add a workflow step to a crew",
	Long: `Add a step to a crew's workflow.
Example: rlama agent crew add-step crew_123 agent_456 --instruction "Analyze this data" --output-to-next`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Ajout d'une étape au workflow (avec timeout)...")
		crewID := args[0]
		agentID := args[1]
		instruction, _ := cmd.Flags().GetString("instruction")
		dependsOnStr, _ := cmd.Flags().GetString("depends-on")
		outputToNext, _ := cmd.Flags().GetBool("output-to-next")

		// Utiliser un timeout global pour l'opération complète
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		
		// Canal pour recevoir le résultat
		resultChan := make(chan error, 1)
		
		go func() {
			// Parse dependsOn
			var dependsOn []int
			if dependsOnStr != "" {
				parts := strings.Split(dependsOnStr, ",")
				for _, part := range parts {
					idx, err := strconv.Atoi(strings.TrimSpace(part))
					if err != nil {
						resultChan <- fmt.Errorf("invalid depends-on format: %w", err)
						return
					}
					dependsOn = append(dependsOn, idx)
				}
			}
			
			// Charger le crew directement depuis le disque
			crew, err := getCrewByID(crewID)
			if err != nil {
				resultChan <- fmt.Errorf("erreur lors du chargement du crew: %w", err)
				return
			}
			
			// Vérifier que l'agent existe
			_, err = getAgentByID(agentID)
			if err != nil {
				resultChan <- fmt.Errorf("erreur lors du chargement de l'agent: %w", err)
				return
			}
			
			// Créer et ajouter l'étape directement
			step := domain.WorkflowStep{
				AgentID:      agentID,
				Instruction:  instruction,
				DependsOn:    dependsOn,
				OutputToNext: outputToNext,
			}
			
			// Ajouter l'étape au workflow
			crew.Workflow.Steps = append(crew.Workflow.Steps, step)
			crew.UpdatedAt = time.Now()
			
			// Sauvegarder le crew
			basePath := filepath.Join(os.Getenv("HOME"), ".rlama", "agents")
			crewPath := filepath.Join(basePath, crew.ID+".json")
			
			// Convertir en JSON
			crewData, err := json.MarshalIndent(crew, "", "  ")
			if err != nil {
				resultChan <- fmt.Errorf("erreur lors de la sérialisation du crew: %w", err)
				return
			}
			
			// Écrire le fichier
			err = ioutil.WriteFile(crewPath, crewData, 0644)
			if err != nil {
				resultChan <- fmt.Errorf("erreur lors de l'écriture du crew: %w", err)
				return
			}
			
			resultChan <- nil
		}()
		
		// Attendre le résultat ou le timeout
		select {
		case err := <-resultChan:
			if err != nil {
				return err
			}
			crew, err := getCrewByName(crewID)
			if err != nil {
				return fmt.Errorf("erreur lors de la récupération du nom du crew: %w", err)
			}
			fmt.Printf("Étape ajoutée au workflow du crew '%s' à la position %d.\n", crew.Name, len(crew.Workflow.Steps)-1)
			return nil
		case <-ctx.Done():
			return fmt.Errorf("timeout: l'opération a pris trop de temps")
		}
	},
}

var runCrewCmd = &cobra.Command{
	Use:   "run [crew-id]",
	Short: "Run a crew with an instruction",
	Long: `Run a crew of agents with a given instruction.
Example: rlama agent crew run crew_123 "Research quantum computing advancements"`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Exécution du crew (avec timeout)...")
		crewID := args[0]
		
		// Construire l'instruction à partir des arguments restants
		var instruction string
		if len(args) > 1 {
			instruction = strings.Join(args[1:], " ")
		} else {
			// Si aucune instruction n'est fournie, en demander une
			fmt.Print("Enter instruction: ")
			scanner := bufio.NewReader(os.Stdin)
			instructionInput, _ := scanner.ReadString('\n')
			instruction = strings.TrimSpace(instructionInput)
		}
		
		if instruction == "" {
			return fmt.Errorf("instruction cannot be empty")
		}
		
		// Utiliser un timeout global plus long pour l'exécution du crew
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		
		// Canal pour recevoir le résultat
		resultChan := make(chan string, 1)
		errChan := make(chan error, 1)
		
		go func() {
			// Charger le crew directement
			crew, err := getCrewByID(crewID)
			if err != nil {
				errChan <- fmt.Errorf("erreur lors du chargement du crew: %w", err)
				return
			}
			
			// Vérifier qu'il y a des étapes dans le workflow
			if len(crew.Workflow.Steps) == 0 {
				errChan <- fmt.Errorf("le crew n'a pas d'étapes de workflow définies")
				return
			}
			
			// Créer des services temporaires pour l'exécution
			ollamaClient := GetOllamaClient()
			ragService := service.NewRagService(ollamaClient)
			agentService := service.NewAgentService(ollamaClient, ragService)
			
			// Exécuter le crew avec un contexte de timeout
			result, err := executeCrewWithTimeout(ctx, crew, instruction, agentService)
			if err != nil {
				errChan <- err
				return
			}
			
			resultChan <- result
		}()
		
		// Attendre le résultat ou le timeout
		select {
		case result := <-resultChan:
			fmt.Println("\n--- Résultat du Crew ---")
			fmt.Println(result)
			return nil
		case err := <-errChan:
			return err
		case <-ctx.Done():
			return fmt.Errorf("timeout: l'exécution du crew a pris trop de temps")
		}
	},
}

// Fonction auxiliaire pour exécuter un crew avec timeout
func executeCrewWithTimeout(ctx context.Context, crew *domain.Crew, instruction string, agentService *service.AgentService) (string, error) {
	// Créer un canal pour timeout interne
	timeoutCtx, cancel := context.WithTimeout(ctx, 4*time.Minute)
	defer cancel()
	
	// Canal pour résultat
	resultChan := make(chan string, 1)
	errChan := make(chan error, 1)
	
	go func() {
		fmt.Println("Exécution du workflow...")
		
		if crew.Workflow.Type == "sequential" {
			// Exécution séquentielle
			outputs := make(map[int]string)
			
			for i, step := range crew.Workflow.Steps {
				fmt.Printf("Étape %d: Exécution de l'agent '%s'...\n", i, step.AgentID)
				
				// Charger l'agent pour cette étape
				agent, err := getAgentByID(step.AgentID)
				if err != nil {
					errChan <- fmt.Errorf("erreur lors du chargement de l'agent pour l'étape %d: %w", i, err)
					return
				}
				
				// Préparer l'instruction pour cette étape
				stepInstruction := step.Instruction
				if stepInstruction == "" {
					stepInstruction = instruction
				}
				
				// Si cette étape dépend d'autres étapes, incorporer leurs sorties
				if len(step.DependsOn) > 0 {
					for _, depIndex := range step.DependsOn {
						if depOutput, ok := outputs[depIndex]; ok {
							stepInstruction += "\n\nRésultat de l'étape " + strconv.Itoa(depIndex) + ":\n" + depOutput
						}
					}
				}
				
				// Exécuter l'agent avec un timeout interne
				agentCtx, agentCancel := context.WithTimeout(timeoutCtx, 60*time.Second)
				agentResult := make(chan string, 1)
				agentErr := make(chan error, 1)
				
				go func() {
					// Simuler l'exécution d'un agent
					output, err := agentService.RunAgent(agent, stepInstruction)
					if err != nil {
						agentErr <- err
						return
					}
					agentResult <- output
				}()
				
				// Attendre le résultat de l'agent ou timeout
				var output string
				select {
				case output = <-agentResult:
					// Succès
				case err := <-agentErr:
					agentCancel()
					errChan <- fmt.Errorf("erreur lors de l'exécution de l'agent à l'étape %d: %w", i, err)
					return
				case <-agentCtx.Done():
					agentCancel()
					errChan <- fmt.Errorf("timeout lors de l'exécution de l'agent à l'étape %d", i)
					return
				}
				
				// Enregistrer le résultat pour les étapes futures
				outputs[i] = output
				
				// Si c'est la dernière étape ou si outputToNext est false, afficher le résultat
				if i == len(crew.Workflow.Steps)-1 || !step.OutputToNext {
					fmt.Printf("Résultat de l'étape %d:\n%s\n\n", i, output)
				}
			}
			
			// Retourner le résultat de la dernière étape
			lastIndex := len(crew.Workflow.Steps) - 1
			if result, ok := outputs[lastIndex]; ok {
				resultChan <- result
			} else {
				errChan <- fmt.Errorf("résultat final non disponible")
			}
		} else if crew.Workflow.Type == "parallel" {
			// Exécution parallèle (simplifiée)
			errChan <- fmt.Errorf("l'exécution parallèle n'est pas encore implémentée complètement")
			return
		} else {
			errChan <- fmt.Errorf("type de workflow non reconnu: %s", crew.Workflow.Type)
			return
		}
	}()
	
	// Attendre le résultat ou le timeout
	select {
	case result := <-resultChan:
		return result, nil
	case err := <-errChan:
		return "", err
	case <-timeoutCtx.Done():
		return "", fmt.Errorf("timeout global lors de l'exécution du crew")
	}
}

// Fonction auxiliaire pour trouver un crew par son ID
func getCrewByID(crewID string) (*domain.Crew, error) {
	basePath := filepath.Join(os.Getenv("HOME"), ".rlama", "agents")
	crewPath := filepath.Join(basePath, crewID+".json")
	
	// Vérifier si l'ID est un nom plutôt qu'un ID
	if !strings.HasPrefix(crewID, "crew_") {
		// C'est un nom, chercher l'ID correspondant
		crew, err := getCrewByName(crewID)
		if err == nil {
			return crew, nil
		}
	}
	
	// Essayer de charger directement par ID
	if _, err := os.Stat(crewPath); err == nil {
		data, err := ioutil.ReadFile(crewPath)
		if err != nil {
			return nil, err
		}
		
		crew := &domain.Crew{}
		if err := json.Unmarshal(data, crew); err != nil {
			return nil, err
		}
		
		return crew, nil
	}
	
	return nil, fmt.Errorf("crew non trouvé: %s", crewID)
}

// Fonction auxiliaire pour trouver un agent par son ID
func getAgentByID(agentID string) (*domain.Agent, error) {
	basePath := filepath.Join(os.Getenv("HOME"), ".rlama", "agents")
	agentPath := filepath.Join(basePath, agentID+".json")
	
	// Vérifier si l'ID est un nom plutôt qu'un ID
	if !strings.HasPrefix(agentID, "agent_") {
		// C'est un nom, chercher l'ID correspondant
		agent, err := getAgentByName(agentID)
		if err == nil {
			return agent, nil
		}
	}
	
	// Essayer de charger directement par ID
	if _, err := os.Stat(agentPath); err == nil {
		data, err := ioutil.ReadFile(agentPath)
		if err != nil {
			return nil, err
		}
		
		agent := &domain.Agent{}
		if err := json.Unmarshal(data, agent); err != nil {
			return nil, err
		}
		
		return agent, nil
	}
	
	return nil, fmt.Errorf("agent non trouvé: %s", agentID)
}

// Fonction auxiliaire pour obtenir les agents directement depuis le disque
func getAgentFilesFromDisk() ([]*domain.Agent, error) {
	basePath := filepath.Join(os.Getenv("HOME"), ".rlama", "agents")
	files, err := ioutil.ReadDir(basePath)
	if err != nil {
		return nil, err
	}
	
	var agents []*domain.Agent
	for _, file := range files {
		if !file.IsDir() && strings.HasPrefix(file.Name(), "agent_") && strings.HasSuffix(file.Name(), ".json") {
			data, err := ioutil.ReadFile(filepath.Join(basePath, file.Name()))
			if err != nil {
				continue // Skip files with errors
			}
			
			agent := &domain.Agent{}
			if json.Unmarshal(data, agent) == nil {
				agents = append(agents, agent)
			}
		}
	}
	
	return agents, nil
}

// Fonction utilitaire pour les valeurs par défaut des chaînes
func defaultString(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

// Fonction auxiliaire pour trouver un crew par son nom
func getCrewByName(crewName string) (*domain.Crew, error) {
	basePath := filepath.Join(os.Getenv("HOME"), ".rlama", "agents")
	files, err := ioutil.ReadDir(basePath)
	if err != nil {
		return nil, err
	}
	
	// Chercher dans tous les fichiers crew
	for _, file := range files {
		if !file.IsDir() && strings.HasPrefix(file.Name(), "crew_") && strings.HasSuffix(file.Name(), ".json") {
			data, err := ioutil.ReadFile(filepath.Join(basePath, file.Name()))
			if err != nil {
				continue // Ignorer les fichiers avec erreurs
			}
			
			crew := &domain.Crew{}
			if err := json.Unmarshal(data, crew); err != nil {
				continue // Ignorer les fichiers avec erreurs
			}
			
			if crew.Name == crewName {
				return crew, nil
			}
		}
	}
	
	return nil, fmt.Errorf("crew non trouvé: %s", crewName)
}

// Fonction auxiliaire pour trouver un agent par son nom
func getAgentByName(agentName string) (*domain.Agent, error) {
	basePath := filepath.Join(os.Getenv("HOME"), ".rlama", "agents")
	files, err := ioutil.ReadDir(basePath)
	if err != nil {
		return nil, err
	}
	
	// Chercher dans tous les fichiers agent
	for _, file := range files {
		if !file.IsDir() && strings.HasPrefix(file.Name(), "agent_") && strings.HasSuffix(file.Name(), ".json") {
			data, err := ioutil.ReadFile(filepath.Join(basePath, file.Name()))
			if err != nil {
				continue // Ignorer les fichiers avec erreurs
			}
			
			agent := &domain.Agent{}
			if err := json.Unmarshal(data, agent); err != nil {
				continue // Ignorer les fichiers avec erreurs
			}
			
			if agent.Name == agentName {
				return agent, nil
			}
		}
	}
	
	return nil, fmt.Errorf("agent non trouvé: %s", agentName)
}

// Fonction utilitaire pour vérifier si une chaîne est présente dans un tableau
func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func init() {
	rootCmd.AddCommand(agentCmd)
	
	// Sous-commandes pour les agents
	agentCmd.AddCommand(createAgentCmd)
	agentCmd.AddCommand(listAgentsCmd)
	agentCmd.AddCommand(deleteAgentCmd)
	agentCmd.AddCommand(agentRunCmd)
	
	// Flags pour createAgentCmd
	createAgentCmd.Flags().StringVar(&agentDescription, "description", "", "Description of the agent")
	createAgentCmd.Flags().StringVar(&agentRole, "role", "", "Role of the agent (researcher, coder, analyst, etc.)")
	createAgentCmd.Flags().StringVar(&agentModelName, "model", "", "Model to use for the agent")
	createAgentCmd.Flags().StringVar(&agentRagName, "rag", "", "RAG system to associate with the agent")
	createAgentCmd.Flags().Bool("allow-commands", false, "Autoriser l'agent à exécuter des commandes système")
	createAgentCmd.Flags().Bool("allow-files", false, "Autoriser l'agent à créer et modifier des fichiers")
	
	// Flags pour deleteAgentCmd
	deleteAgentCmd.Flags().BoolVar(&forceDeleteAgent, "force", false, "Delete without asking for confirmation")
	
	// Gestion des crews
	agentCmd.AddCommand(crewCmd)
	
	// Sous-commandes pour les crews
	crewCmd.AddCommand(createCrewCmd)
	crewCmd.AddCommand(agentCrewAddAgentCmd)
	crewCmd.AddCommand(addWorkflowStepCmd)
	crewCmd.AddCommand(runCrewCmd)
	
	// Flags pour createCrewCmd
	createCrewCmd.Flags().String("workflow", "sequential", "Workflow type (sequential or parallel)")
	createCrewCmd.Flags().String("description", "", "Description of the crew")
	
	// Flags pour addWorkflowStepCmd
	addWorkflowStepCmd.Flags().String("instruction", "", "Instruction for this step")
	addWorkflowStepCmd.Flags().String("depends-on", "", "Comma-separated list of step indices this step depends on")
	addWorkflowStepCmd.Flags().Bool("output-to-next", false, "Whether this step's output should be passed to the next step")
	
	// Ajouter cette ligne dans la fonction init() après l'ajout des autres sous-commandes
	crewCmd.AddCommand(wizardCmd)
} 