package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/dontizi/rlama/internal/client"
	"github.com/dontizi/rlama/internal/domain"
)

// AgentService gère les opérations liées aux agents
type AgentService struct {
	ollamaClient *client.OllamaClient
	ragService   RagService
	basePath     string
	agents       map[string]*domain.Agent
	crews        map[string]*domain.Crew
	toolRegistry map[string]ToolFunction
	mutex        sync.RWMutex
}

// ToolFunction représente une fonction qui peut être exécutée par un outil
type ToolFunction func(agent *domain.Agent, input string) (string, error)

// NewAgentService crée un nouveau service pour les agents
func NewAgentService(ollamaClient *client.OllamaClient, ragService RagService) *AgentService {
	if ollamaClient == nil {
		ollamaClient = client.NewDefaultOllamaClient()
	}
	
	// Chemin de base pour stocker les données des agents
	basePath := filepath.Join(os.Getenv("HOME"), ".rlama", "agents")
	if _, err := os.Stat(basePath); os.IsNotExist(err) {
		os.MkdirAll(basePath, 0755)
	}
	
	return &AgentService{
		ollamaClient: ollamaClient,
		ragService:   ragService,
		basePath:     basePath,
		agents:       make(map[string]*domain.Agent),
		crews:        make(map[string]*domain.Crew),
		toolRegistry: registerDefaultTools(),
		mutex:        sync.RWMutex{},
	}
}

// registerDefaultTools enregistre les outils par défaut disponibles
func registerDefaultTools() map[string]ToolFunction {
	tools := make(map[string]ToolFunction)
	
	// Outil de recherche RAG
	tools["rag_search"] = func(agent *domain.Agent, input string) (string, error) {
		if agent.RAGName == "" {
			return "", fmt.Errorf("agent does not have an associated RAG")
		}
		
		// Cette fonction serait implémentée pour interroger le RAG
		// Ceci est une version simplifiée
		return fmt.Sprintf("Search results for '%s' in RAG '%s'", input, agent.RAGName), nil
	}
	
	// Calculatrice simple
	tools["calculator"] = func(agent *domain.Agent, input string) (string, error) {
		// Implémentation simplifiée - dans une vraie application, utilisez une 
		// bibliothèque d'évaluation d'expressions mathématiques
		return fmt.Sprintf("Calculated result for '%s'", input), nil
	}
	
	// Nouvel outil pour exécuter des commandes
	tools["execute_command"] = func(agent *domain.Agent, input string) (string, error) {
		// Vérifier que l'agent a la permission d'exécuter des commandes
		if !agent.AllowSystemCommands {
			return "", fmt.Errorf("cet agent n'a pas la permission d'exécuter des commandes système")
		}
		
		// Extraire la commande à exécuter
		command := strings.TrimSpace(input)
		
		// Sécurité: limiter les commandes potentiellement dangereuses
		if strings.Contains(command, "rm -rf") || strings.Contains(command, "sudo") {
			return "", fmt.Errorf("commande non autorisée pour des raisons de sécurité")
		}
		
		// Préparation de la commande
		parts := strings.Fields(command)
		if len(parts) == 0 {
			return "", fmt.Errorf("commande vide")
		}
		
		cmd := exec.Command(parts[0], parts[1:]...)
		
		// Exécution de la commande
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		
		err := cmd.Run()
		if err != nil {
			return "", fmt.Errorf("erreur lors de l'exécution de la commande: %v\n%s", err, stderr.String())
		}
		
		return stdout.String(), nil
	}
	
	// Nouvel outil pour écrire dans un fichier
	tools["write_file"] = func(agent *domain.Agent, input string) (string, error) {
		// Vérifier que l'agent a la permission d'écrire des fichiers
		if !agent.AllowFileOperations {
			return "", fmt.Errorf("cet agent n'a pas la permission d'écrire des fichiers")
		}
		
		// Format attendu: première ligne = chemin du fichier, reste = contenu
		lines := strings.SplitN(input, "\n", 2)
		if len(lines) < 2 {
			return "", fmt.Errorf("format incorrect: la première ligne doit être le chemin du fichier, les lignes suivantes le contenu")
		}
		
		filePath := strings.TrimSpace(lines[0])
		content := lines[1]
		
		// Sécurité: limiter les chemins pour éviter l'écriture en dehors de certains dossiers
		allowedPaths := []string{"agent-docs", "output", "./agent-docs", "./output"}
		allowed := false
		
		for _, path := range allowedPaths {
			if strings.HasPrefix(filePath, path) {
				allowed = true
				break
			}
		}
		
		if !allowed {
			return "", fmt.Errorf("chemin non autorisé: les fichiers doivent être dans l'un des dossiers suivants: %v", allowedPaths)
		}
		
		// Créer le dossier parent si nécessaire
		dir := filepath.Dir(filePath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return "", fmt.Errorf("erreur lors de la création du dossier: %v", err)
		}
		
		// Écrire le fichier
		if err := ioutil.WriteFile(filePath, []byte(content), 0644); err != nil {
			return "", fmt.Errorf("erreur lors de l'écriture du fichier: %v", err)
		}
		
		return fmt.Sprintf("Fichier écrit avec succès: %s", filePath), nil
	}
	
	return tools
}

// RegisterTool enregistre un nouvel outil personnalisé
func (as *AgentService) RegisterTool(name string, function ToolFunction) {
	as.mutex.Lock()
	defer as.mutex.Unlock()
	
	as.toolRegistry[name] = function
}

// CreateAgent crée un nouvel agent
func (as *AgentService) CreateAgent(name, description, role, modelName string, ragName string) (*domain.Agent, error) {
	// Vérifier si un agent avec ce nom existe déjà
	agents, err := as.ListAgents()
	if err == nil {
		for _, existingAgent := range agents {
			if existingAgent.Name == name {
				// Retourner l'agent existant au lieu d'en créer un nouveau
				fmt.Printf("Agent '%s' existe déjà avec ID: %s\n", name, existingAgent.ID)
				return existingAgent, nil
				// Alternative: retourner une erreur
				// return nil, fmt.Errorf("un agent avec le nom '%s' existe déjà", name)
			}
		}
	}
	
	as.mutex.Lock()
	defer as.mutex.Unlock()
	
	// Vérifier que le modèle existe
	err = as.ollamaClient.CheckOllamaAndModel(modelName)
	if err != nil {
		return nil, fmt.Errorf("error checking model: %w", err)
	}
	
	// Vérifier que le RAG existe si spécifié
	if ragName != "" {
		_, err := as.ragService.LoadRag(ragName)
		if err != nil {
			return nil, fmt.Errorf("error loading RAG: %w", err)
		}
	}
	
	agent := domain.NewAgent(name, description, role, modelName)
	agent.RAGName = ragName
	
	// Ajouter des outils par défaut en fonction du rôle
	switch strings.ToLower(role) {
	case "researcher":
		agent.AddTool(domain.Tool{
			Name:        "rag_search",
			Description: "Search information in the associated RAG system",
			Type:        domain.ToolTypeSearch,
		})
	case "coder":
		agent.AddTool(domain.Tool{
			Name:        "code_execution",
			Description: "Execute code and return the result",
			Type:        domain.ToolTypeCodeExec,
		})
	case "analyst":
		agent.AddTool(domain.Tool{
			Name:        "calculator",
			Description: "Perform mathematical calculations",
			Type:        domain.ToolTypeCalculator,
		})
	}
	
	// Enregistrer l'agent
	err = as.saveAgent(agent)
	if err != nil {
		return nil, fmt.Errorf("error saving agent: %w", err)
	}
	
	as.agents[agent.ID] = agent
	return agent, nil
}

// saveAgent sauvegarde un agent sur le disque
func (as *AgentService) saveAgent(agent *domain.Agent) error {
	agentPath := filepath.Join(as.basePath, agent.ID+".json")
	
	data, err := json.MarshalIndent(agent, "", "  ")
	if err != nil {
		return fmt.Errorf("error marshaling agent data: %w", err)
	}
	
	err = ioutil.WriteFile(agentPath, data, 0644)
	if err != nil {
		return fmt.Errorf("error writing agent file: %w", err)
	}
	
	return nil
}

// LoadAgent loads an agent by ID
func (s *AgentService) LoadAgent(agentID string) (*domain.Agent, error) {
	// Chemin du fichier agent
	basePath := filepath.Join(os.Getenv("HOME"), ".rlama", "agents")
	agentPath := filepath.Join(basePath, agentID+".json")
	
	// Vérifier si le fichier existe
	if _, err := os.Stat(agentPath); os.IsNotExist(err) {
		// Essayer de chercher par nom d'agent au lieu de l'ID
		files, err := ioutil.ReadDir(basePath)
		if err != nil {
			return nil, fmt.Errorf("error reading agents directory: %w", err)
		}
		
		// Rechercher les fichiers .json pour trouver une correspondance
		for _, file := range files {
			if filepath.Ext(file.Name()) == ".json" {
				filePath := filepath.Join(basePath, file.Name())
				data, err := ioutil.ReadFile(filePath)
				if err != nil {
					continue
				}
				
				agent := &domain.Agent{}
				if err := json.Unmarshal(data, agent); err != nil {
					continue
				}
				
				if agent.ID == agentID || agent.Name == agentID {
					fmt.Printf("Agent trouvé via recherche : %s (%s)\n", agent.Name, agent.ID)
					return agent, nil
				}
			}
		}
		
		return nil, fmt.Errorf("agent not found: %s", agentID)
	}
	
	// Lire le fichier agent
	data, err := ioutil.ReadFile(agentPath)
	if err != nil {
		return nil, fmt.Errorf("error reading agent file: %w", err)
	}
	
	// Désérialiser l'agent
	agent := &domain.Agent{}
	if err := json.Unmarshal(data, agent); err != nil {
		return nil, fmt.Errorf("error deserializing agent: %w", err)
	}
	
	return agent, nil
}

// ListAgents liste tous les agents disponibles
func (as *AgentService) ListAgents() ([]*domain.Agent, error) {
	// Créer un contexte avec timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	// Canal pour le résultat
	resultChan := make(chan []*domain.Agent, 1)
	errChan := make(chan error, 1)
	
	go func() {
		// Code original pour lister les agents
		files, err := ioutil.ReadDir(as.basePath)
		if err != nil {
			errChan <- err
			return
		}
		
		var agents []*domain.Agent
		for _, file := range files {
			if file.IsDir() || !strings.HasSuffix(file.Name(), ".json") || strings.HasPrefix(file.Name(), "crew_") {
				continue
			}
			
			agentID := strings.TrimSuffix(file.Name(), ".json")
			agent, err := as.LoadAgent(agentID)
			if err != nil {
				fmt.Printf("Warning: Could not load agent %s: %v\n", agentID, err)
				continue
			}
			
			agents = append(agents, agent)
		}
		
		resultChan <- agents
	}()
	
	// Attendre le résultat ou le timeout
	select {
	case agents := <-resultChan:
		return agents, nil
	case err := <-errChan:
		return nil, err
	case <-ctx.Done():
		return nil, fmt.Errorf("timeout: l'opération a pris trop de temps")
	}
}

// DeleteAgent supprime un agent
func (as *AgentService) DeleteAgent(agentID string) error {
	as.mutex.Lock()
	defer as.mutex.Unlock()
	
	// Supprimer du disque
	agentPath := filepath.Join(as.basePath, agentID+".json")
	err := os.Remove(agentPath)
	if err != nil {
		return fmt.Errorf("error deleting agent file: %w", err)
	}
	
	// Supprimer de la mémoire
	delete(as.agents, agentID)
	return nil
}

// GetAgentByName récupère un agent par son nom
func (as *AgentService) GetAgentByName(name string) (*domain.Agent, error) {
	agents, err := as.ListAgents()
	if err != nil {
		return nil, err
	}
	
	for _, agent := range agents {
		if agent.Name == name {
			return agent, nil
		}
	}
	
	return nil, fmt.Errorf("agent with name '%s' not found", name)
}

// RunAgent exécute un agent avec une instruction
func (as *AgentService) RunAgent(agent *domain.Agent, instruction string) (string, error) {
	// Préparer le contexte avec un timeout plus long (5 minutes)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	
	// Exécuter l'agent
	fmt.Printf("Exécution de l'agent '%s' avec le modèle '%s'...\n", agent.Name, agent.ModelName)
	
	// Mettre en forme l'instruction selon le rôle
	formattedInstruction := instruction
	if agent.Role != "" {
		formattedInstruction = fmt.Sprintf("En tant que %s, %s", agent.Role, instruction)
	}
	
	// Ajouter le contexte RAG si nécessaire
	if agent.RAGName != "" && as.ragService != nil {
		contextChunks, err := as.ragService.GetRagChunks(agent.RAGName, ChunkFilter{
			Query: instruction,
			Limit: 5,
		})
		if err == nil && len(contextChunks) > 0 {
			// Ajouter les chunks pertinents au contexte
			contextText := "Contexte pertinent :\n\n"
			for i, chunk := range contextChunks {
				documentID := "inconnu"
				if chunk.ID != "" {
					documentID = chunk.ID
				}
				contextText += fmt.Sprintf("Document %d: %s\n%s\n\n", i+1, documentID, chunk.Content)
			}
			formattedInstruction = contextText + "\n" + formattedInstruction
		}
	}
	
	// Ajouter des instructions spécifiques pour le format de réponse JSON
	formattedInstruction += "\n\nRéponds UNIQUEMENT avec un objet JSON valide qui contient ta réponse. "
	switch agent.Name {
	case "twitter-writer":
		formattedInstruction += "Format attendu: { \"tweets\": [{ \"content\": \"...\", \"hashtags\": [...] }] }"
	case "discord-writer":
		formattedInstruction += "Format attendu: { \"message\": { \"content\": \"...\", \"embeds\": [...] } }"
	case "linkedin-writer":
		formattedInstruction += "Format attendu: { \"post\": { \"title\": \"...\", \"content\": \"...\" } }"
	}
	
	// Préparer les options de génération
	options := client.GenerateOptions{
		Model:       agent.ModelName,
		Prompt:      formattedInstruction,
		System:      agent.Description,
		Format:      "json",
		Stream:      false,
		Temperature: 0.7,
		MaxTokens:   2048,
	}
	
	// Appeler le modèle
	result, err := as.ollamaClient.GenerateWithTimeout(ctx, options)
	if err != nil {
		return "", fmt.Errorf("error generating response: %w", err)
	}
	
	// Vérifier que la réponse est un JSON valide
	var jsonResponse interface{}
	if err := json.Unmarshal([]byte(result), &jsonResponse); err != nil {
		// Si ce n'est pas un JSON valide, essayer d'extraire le JSON de la réponse
		if jsonStart := strings.Index(result, "{"); jsonStart >= 0 {
			if jsonEnd := strings.LastIndex(result, "}"); jsonEnd > jsonStart {
				result = result[jsonStart : jsonEnd+1]
				if err := json.Unmarshal([]byte(result), &jsonResponse); err == nil {
					return result, nil
				}
			}
		}
		return "", fmt.Errorf("invalid JSON response: %w", err)
	}
	
	return result, nil
}

// Structure pour représenter un appel d'outil
type ToolCall struct {
	Name  string
	Input string
}

// extractToolCall extrait les informations d'appel d'outil d'une réponse
func extractToolCall(response string) ToolCall {
	var toolCall ToolCall
	
	// Trouver le bloc d'outil
	startIdx := strings.Index(response, "```tool")
	if startIdx == -1 {
		return toolCall
	}
	
	endIdx := strings.Index(response[startIdx:], "```")
	if endIdx == -1 {
		return toolCall
	}
	
	toolBlock := response[startIdx+7 : startIdx+endIdx]
	toolBlock = strings.TrimSpace(toolBlock)
	
	// Extraire le nom de l'outil et l'entrée
	parts := strings.SplitN(toolBlock, ":", 2)
	if len(parts) != 2 {
		return toolCall
	}
	
	toolCall.Name = strings.TrimSpace(parts[0])
	toolCall.Input = strings.TrimSpace(parts[1])
	
	return toolCall
}

// executeTool exécute un outil pour un agent
func (as *AgentService) executeTool(agent *domain.Agent, toolName, input string) (string, error) {
	// Essayer d'exécuter l'outil à partir du registre
	toolFunc, exists := as.toolRegistry[toolName]
	if !exists {
		return "", fmt.Errorf("tool not found: %s", toolName)
	}
	
	// Si l'outil est rag_search et qu'un RAG est associé
	if toolName == "rag_search" && agent.RAGName != "" {
		// Charger le RAG et l'utiliser directement - utiliser rag ici
		contextChunks, err := as.Search(agent.ID, input)
		if err == nil && len(contextChunks) > 0 {
			results := "Search results:\n\n"
			for _, chunk := range contextChunks {
				results += chunk.Content + "\n\n"
			}
			return results, nil
		}
	}
	
	// Exécuter l'outil avec l'agent et l'entrée
	return toolFunc(agent, input)
}

// CreateCrew crée un nouveau crew
func (as *AgentService) CreateCrew(name, description string, workflowType domain.WorkflowType) (*domain.Crew, error) {
	as.mutex.Lock()
	defer as.mutex.Unlock()
	
	crew := domain.NewCrew(name, description, workflowType)
	
	// Enregistrer le crew
	err := as.saveCrew(crew)
	if err != nil {
		return nil, fmt.Errorf("error saving crew: %w", err)
	}
	
	as.crews[crew.ID] = crew
	return crew, nil
}

// saveCrew sauvegarde un crew sur le disque
func (as *AgentService) saveCrew(crew *domain.Crew) error {
	crewPath := filepath.Join(as.basePath, crew.ID+".json")
	
	data, err := json.MarshalIndent(crew, "", "  ")
	if err != nil {
		return fmt.Errorf("error marshaling crew data: %w", err)
	}
	
	err = ioutil.WriteFile(crewPath, data, 0644)
	if err != nil {
		return fmt.Errorf("error writing crew file: %w", err)
	}
	
	return nil
}

// LoadCrew charge un crew depuis le disque
func (as *AgentService) LoadCrew(crewID string) (*domain.Crew, error) {
	// Ajouter un timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	// Canal pour résultat
	resultChan := make(chan *domain.Crew, 1)
	errChan := make(chan error, 1)
	
	go func() {
		// Essayer de charger depuis le cache en mémoire d'abord
		as.mutex.RLock()
		if crew, ok := as.crews[crewID]; ok {
			as.mutex.RUnlock()
			resultChan <- crew
			return
		}
		as.mutex.RUnlock()
		
		// Essayer de charger par ID
		crewPath := filepath.Join(as.basePath, crewID+".json")
		if _, err := os.Stat(crewPath); err == nil {
			data, err := ioutil.ReadFile(crewPath)
			if err != nil {
				errChan <- err
				return
			}
			
			crew := &domain.Crew{}
			if err := json.Unmarshal(data, crew); err != nil {
				errChan <- err
				return
			}
			
			// Mettre en cache
			as.mutex.Lock()
			as.crews[crewID] = crew
			as.mutex.Unlock()
			
			resultChan <- crew
			return
		}
		
		// Si c'est un nom et pas un ID, chercher le crew par son nom
		// directement dans les fichiers sans appeler ListCrews() qui pourrait bloquer
		files, err := ioutil.ReadDir(as.basePath)
		if err != nil {
			errChan <- err
			return
		}
		
		for _, file := range files {
			if !file.IsDir() && strings.HasPrefix(file.Name(), "crew_") && strings.HasSuffix(file.Name(), ".json") {
				data, err := ioutil.ReadFile(filepath.Join(as.basePath, file.Name()))
				if err != nil {
					continue
				}
				
				crew := &domain.Crew{}
				if err := json.Unmarshal(data, crew); err != nil {
					continue
				}
				
				if crew.Name == crewID {
					// Mettre en cache
					as.mutex.Lock()
					as.crews[crew.ID] = crew
					as.mutex.Unlock()
					
					resultChan <- crew
					return
				}
			}
		}
		
		errChan <- fmt.Errorf("crew not found: %s", crewID)
	}()
	
	// Attendre le résultat ou le timeout
	select {
	case crew := <-resultChan:
		return crew, nil
	case err := <-errChan:
		return nil, err
	case <-ctx.Done():
		return nil, fmt.Errorf("timeout lors du chargement du crew")
	}
}

// GetCrewByName récupère un crew par son nom
func (as *AgentService) GetCrewByName(name string) (*domain.Crew, error) {
	crews, err := as.ListCrews()
	if err != nil {
		return nil, err
	}
	
	for _, crew := range crews {
		if crew.Name == name {
			return crew, nil
		}
	}
	
	return nil, fmt.Errorf("crew with name '%s' not found", name)
}

// ListCrews liste tous les crews disponibles
func (as *AgentService) ListCrews() ([]*domain.Crew, error) {
	as.mutex.RLock()
	defer as.mutex.RUnlock()
	
	// Rechercher tous les fichiers crew_*.json dans le dossier des agents
	files, err := ioutil.ReadDir(as.basePath)
	if err != nil {
		return nil, fmt.Errorf("error reading agents directory: %w", err)
	}
	
	var crews []*domain.Crew
	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".json") || !strings.HasPrefix(file.Name(), "crew_") {
			continue
		}
		
		crewID := strings.TrimSuffix(file.Name(), ".json")
		crew, err := as.LoadCrew(crewID)
		if err != nil {
			fmt.Printf("Warning: Could not load crew %s: %v\n", crewID, err)
			continue
		}
		
		crews = append(crews, crew)
	}
	
	return crews, nil
}

// DeleteCrew supprime un crew
func (as *AgentService) DeleteCrew(crewID string) error {
	as.mutex.Lock()
	defer as.mutex.Unlock()
	
	// Supprimer du disque
	crewPath := filepath.Join(as.basePath, crewID+".json")
	err := os.Remove(crewPath)
	if err != nil {
		return fmt.Errorf("error deleting crew file: %w", err)
	}
	
	// Supprimer de la mémoire
	delete(as.crews, crewID)
	return nil
}

// AddAgentToCrew ajoute un agent à un crew
func (as *AgentService) AddAgentToCrew(crew *domain.Crew, agentID string) error {
	// Vérifier que l'agent existe
	_, err := as.LoadAgent(agentID)
	if err != nil {
		return fmt.Errorf("agent not found: %w", err)
	}
	
	// Ajouter l'agent au crew
	crew.AddAgent(agentID)
	
	// Sauvegarder le crew
	return as.saveCrew(crew)
}

// AddWorkflowStep ajoute une étape au workflow d'un crew
func (as *AgentService) AddWorkflowStep(crew *domain.Crew, agentID, instruction string, dependsOn []int, outputToNext bool) error {
	// Vérifier que l'agent existe
	_, err := as.LoadAgent(agentID)
	if err != nil {
		return fmt.Errorf("agent not found: %w", err)
	}
	
	// Créer l'étape
	step := domain.WorkflowStep{
		AgentID:      agentID,
		Instruction:  instruction,
		DependsOn:    dependsOn,
		OutputToNext: outputToNext,
	}
	
	// Ajouter l'étape au workflow
	crew.AddWorkflowStep(step)
	
	// Sauvegarder le crew
	return as.saveCrew(crew)
}

// RunCrew exécute un crew avec une instruction donnée
func (as *AgentService) RunCrew(crew *domain.Crew, instruction string) (map[string]string, error) {
	results := make(map[string]string)
	var mutex sync.Mutex
	var wg sync.WaitGroup
	errChan := make(chan error, len(crew.Workflow.Steps))

	// Charger tous les agents nécessaires
	fmt.Printf("Chargement des agents pour le crew '%s' (%s)\n", crew.Name, crew.ID)
	fmt.Printf("Liste des agents à charger: %v\n", crew.Agents)

	for i, step := range crew.Workflow.Steps {
		wg.Add(1)
		go func(i int, step domain.WorkflowStep) {
			defer wg.Done()

			// Charger l'agent
			agent, err := as.LoadAgent(step.AgentID)
			if err != nil {
				errChan <- fmt.Errorf("error loading agent for step %d: %w", i, err)
				return
			}

			// Préparer l'instruction pour cet agent
			stepInstruction := instruction
			if step.Instruction != "" {
				stepInstruction = strings.Replace(step.Instruction, "{INPUT_TEXT}", instruction, -1)
			}

			fmt.Printf("Running agent '%s' with instruction: %s\n", agent.Name, stepInstruction)
			
			// Exécuter l'agent
			result, err := as.RunAgent(agent, stepInstruction)
			if err != nil {
				errChan <- fmt.Errorf("error running agent %s: %w", agent.Name, err)
				return
			}

			// Stocker le résultat
			mutex.Lock()
			results[step.AgentID] = result
			mutex.Unlock()
		}(i, step)
	}

	// Attendre que tous les agents terminent
	wg.Wait()
	close(errChan)

	// Vérifier s'il y a eu des erreurs
	var errs []error
	for err := range errChan {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return results, fmt.Errorf("errors during crew execution: %v", errs)
	}

	return results, nil
}

// Search recherche des informations dans le RAG associé à un agent
func (as *AgentService) Search(agentID, query string) ([]*domain.DocumentChunk, error) {
	agent, err := as.LoadAgent(agentID)
	if err != nil {
		return nil, fmt.Errorf("error loading agent: %w", err)
	}
	
	if agent.RAGName == "" {
		return nil, fmt.Errorf("agent does not have an associated RAG")
	}
	
	// On utilise la variable rag ici au lieu de la juste déclarer
	contextChunks, err := as.ragService.GetRagChunks(agent.RAGName, ChunkFilter{
		Query: query,
		Limit: 5,
	})
	if err != nil {
		return nil, fmt.Errorf("error searching RAG: %w", err)
	}
	
	return contextChunks, nil
} 