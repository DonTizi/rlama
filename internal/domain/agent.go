package domain

import (
	"fmt"
	"time"
)

// Agent représente un agent IA dans le système RLAMA
type Agent struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Role        string            `json:"role"`
	ModelName   string            `json:"model_name"`
	RAGName     string            `json:"rag_name,omitempty"` // RAG optionnel associé à cet agent
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	Tools       []Tool            `json:"tools"`
	Memory      []Message         `json:"memory,omitempty"` // Historique des conversations
	Parameters  map[string]string `json:"parameters,omitempty"`
	AllowSystemCommands bool      `json:"allow_system_commands,omitempty"`
	AllowFileOperations bool      `json:"allow_file_operations,omitempty"`
}

// Tool représente un outil que l'agent peut utiliser
type Tool struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Type        ToolType `json:"type"`
	Function    string   `json:"function,omitempty"` // Nom de la fonction à exécuter
	Parameters  []string `json:"parameters,omitempty"`
}

// ToolType représente les différents types d'outils disponibles
type ToolType string

const (
	ToolTypeSearch       ToolType = "search"       // Recherche dans le RAG
	ToolTypeWebSearch    ToolType = "web_search"   // Recherche web
	ToolTypeCalculator   ToolType = "calculator"   // Calculatrice
	ToolTypeCodeExec     ToolType = "code_exec"    // Exécution de code
	ToolTypeAPICall      ToolType = "api_call"     // Appel API
	ToolTypeFileManager  ToolType = "file_manager" // Gestion de fichiers
	ToolTypeCustom       ToolType = "custom"       // Outil personnalisé
)

// Message représente un message dans la conversation d'un agent
type Message struct {
	Role      string    `json:"role"` // "user", "assistant", "system", "tool"
	Content   string    `json:"content"`
	ToolName  string    `json:"tool_name,omitempty"`
	ToolInput string    `json:"tool_input,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// Crew représente un groupe d'agents qui collaborent
type Crew struct {
	ID                string      `json:"ID"`
	Name              string      `json:"Name"`
	Description       string      `json:"Description,omitempty"`
	Agents            []string    `json:"Agents,omitempty"`
	Workflow          Workflow    `json:"Workflow,omitempty"`
	CreatedAt         time.Time   `json:"CreatedAt,omitempty"`
	UpdatedAt         time.Time   `json:"UpdatedAt,omitempty"`
	RAGName           string      `json:"RAGName,omitempty"`
	InstructionTemplate string    `json:"InstructionTemplate,omitempty"`
}

// WorkflowType définit comment les agents interagissent
type WorkflowType string

const (
	WorkflowSequential WorkflowType = "sequential" // Les agents travaillent l'un après l'autre
	WorkflowParallel   WorkflowType = "parallel"   // Les agents travaillent en parallèle
	WorkflowCustom     WorkflowType = "custom"     // Flux de travail personnalisé
)

// Workflow définit comment les agents interagissent
type Workflow struct {
	Type       WorkflowType     `json:"type"`
	Steps      []WorkflowStep   `json:"steps,omitempty"`
	CustomFlow map[string][]int `json:"custom_flow,omitempty"` // Pour les flux personnalisés
}

// WorkflowStep représente une étape dans un workflow
type WorkflowStep struct {
	AgentID      string `json:"agent_id"`
	Instruction  string `json:"instruction"`
	DependsOn    []int  `json:"depends_on,omitempty"` // Indices des étapes dont celle-ci dépend
	OutputToNext bool   `json:"output_to_next"`       // Si la sortie doit être transmise à l'étape suivante
}

// NewAgent crée un nouvel agent avec les paramètres de base
func NewAgent(name, description, role, modelName string) *Agent {
	now := time.Now()
	return &Agent{
		ID:          fmt.Sprintf("agent_%d", now.UnixNano()),
		Name:        name,
		Description: description,
		Role:        role,
		ModelName:   modelName,
		CreatedAt:   now,
		UpdatedAt:   now,
		Tools:       []Tool{},
		Memory:      []Message{},
		Parameters:  make(map[string]string),
	}
}

// AddTool ajoute un outil à l'agent
func (a *Agent) AddTool(tool Tool) {
	a.Tools = append(a.Tools, tool)
	a.UpdatedAt = time.Now()
}

// RemoveTool supprime un outil de l'agent
func (a *Agent) RemoveTool(toolName string) bool {
	for i, tool := range a.Tools {
		if tool.Name == toolName {
			a.Tools = append(a.Tools[:i], a.Tools[i+1:]...)
			a.UpdatedAt = time.Now()
			return true
		}
	}
	return false
}

// AddMessage ajoute un message à la mémoire de l'agent
func (a *Agent) AddMessage(role, content string) {
	message := Message{
		Role:      role,
		Content:   content,
		Timestamp: time.Now(),
	}
	a.Memory = append(a.Memory, message)
	a.UpdatedAt = time.Now()
}

// AddToolMessage ajoute un message d'outil à la mémoire de l'agent
func (a *Agent) AddToolMessage(toolName, input, output string) {
	inputMsg := Message{
		Role:      "tool",
		ToolName:  toolName,
		ToolInput: input,
		Content:   fmt.Sprintf("Using tool %s with input: %s", toolName, input),
		Timestamp: time.Now(),
	}
	
	outputMsg := Message{
		Role:      "tool",
		ToolName:  toolName,
		Content:   output,
		Timestamp: time.Now(),
	}
	
	a.Memory = append(a.Memory, inputMsg, outputMsg)
	a.UpdatedAt = time.Now()
}

// SetParameter définit un paramètre pour l'agent
func (a *Agent) SetParameter(key, value string) {
	a.Parameters[key] = value
	a.UpdatedAt = time.Now()
}

// GetParameter récupère un paramètre de l'agent
func (a *Agent) GetParameter(key string) (string, bool) {
	val, exists := a.Parameters[key]
	return val, exists
}

// NewCrew crée un nouveau crew avec les paramètres de base
func NewCrew(name, description string, workflowType WorkflowType) *Crew {
	now := time.Now()
	return &Crew{
		ID:          fmt.Sprintf("crew_%d", now.UnixNano()),
		Name:        name,
		Description: description,
		Agents:      []string{},
		Workflow: Workflow{
			Type:  workflowType,
			Steps: []WorkflowStep{},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// AddAgent ajoute un agent au crew
func (c *Crew) AddAgent(agentID string) {
	c.Agents = append(c.Agents, agentID)
	c.UpdatedAt = time.Now()
}

// AddWorkflowStep ajoute une étape au workflow
func (c *Crew) AddWorkflowStep(step WorkflowStep) {
	c.Workflow.Steps = append(c.Workflow.Steps, step)
	c.UpdatedAt = time.Now()
} 