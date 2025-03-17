package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"bufio"
	"time"
)

// Default connection settings for Ollama
const (
	DefaultOllamaHost = "localhost"
	DefaultOllamaPort = "11434"
)

// OllamaClient est un client pour l'API Ollama
type OllamaClient struct {
	BaseURL string
	Client  *http.Client
}

// EmbeddingRequest est la structure de la requête pour l'API /api/embeddings
type EmbeddingRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

// EmbeddingResponse est la structure de la réponse de l'API /api/embeddings
type EmbeddingResponse struct {
	Embedding []float32 `json:"embedding"`
}

// GenerateOptions représente les options pour une requête de génération à Ollama
type GenerateOptions struct {
	Model       string      `json:"model"`
	Prompt      string      `json:"prompt"`
	System      string      `json:"system,omitempty"`
	Template    string      `json:"template,omitempty"`
	Context     []int       `json:"context,omitempty"`
	Format      string      `json:"format,omitempty"`
	Stream      bool        `json:"stream,omitempty"`
	Raw         bool        `json:"raw,omitempty"`
	Options     interface{} `json:"options,omitempty"`
	Temperature float64     `json:"temperature,omitempty"`
	TopP        float64     `json:"top_p,omitempty"`
	TopK        int         `json:"top_k,omitempty"`
	MaxTokens   int         `json:"num_predict,omitempty"`
	Tools       []Tool      `json:"tools,omitempty"`
}

// Tool représente un outil pouvant être utilisé par le LLM
type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Function    func(string) (string, error) `json:"-"`
}

// GenerationResponse représente la réponse d'une requête de génération
type GenerationResponse struct {
	Model     string `json:"model"`
	Response  string `json:"response"`
	CreatedAt string `json:"created_at"`
	Done      bool   `json:"done"`
	Context   []int  `json:"context,omitempty"`
	// Ajoutez d'autres champs si nécessaire
}

// NewOllamaClient crée un nouveau client Ollama
// Si host ou port sont vides, les valeurs par défaut sont utilisées
// Si OLLAMA_HOST est défini, il est utilisé comme valeur par défaut
func NewOllamaClient(host, port string) *OllamaClient {
	// Check for OLLAMA_HOST environment variable
	ollamaHostEnv := os.Getenv("OLLAMA_HOST")
	
	// Default values
	defaultHost := DefaultOllamaHost
	defaultPort := DefaultOllamaPort
	
	// If OLLAMA_HOST is set, parse it
	if ollamaHostEnv != "" {
		// OLLAMA_HOST could be in the form "host:port" or just "host"
		parts := strings.Split(ollamaHostEnv, ":")
		if len(parts) >= 1 {
			defaultHost = parts[0]
		}
		if len(parts) >= 2 {
			defaultPort = parts[1]
		}
	}
	
	// Command flags override environment variables
	if host == "" {
		host = defaultHost
	}
	if port == "" {
		port = defaultPort
	}
	
	// Store base URL without http:// prefix
	baseURL := fmt.Sprintf("%s:%s", host, port)
	
	return &OllamaClient{
		BaseURL: baseURL,
		Client:  &http.Client{},
	}
}

// NewDefaultOllamaClient creates a new Ollama client with default settings
func NewDefaultOllamaClient() *OllamaClient {
	host := DefaultOllamaHost
	port := DefaultOllamaPort
	
	// Check for OLLAMA_HOST environment variable
	if envHost := os.Getenv("OLLAMA_HOST"); envHost != "" {
		// If it contains a port, extract it
		if strings.Contains(envHost, ":") {
			parts := strings.Split(envHost, ":")
			host = parts[0]
			if len(parts) > 1 {
				port = parts[1]
			}
		} else {
			host = envHost
		}
	}
	
	// Create the base URL without http:// prefix - we'll add it in the request methods
	baseURL := fmt.Sprintf("%s:%s", host, port)
	
	return &OllamaClient{
		BaseURL: baseURL,
		Client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// GenerateEmbedding génère un embedding pour le texte donné
func (c *OllamaClient) GenerateEmbedding(model, text string) ([]float32, error) {
	reqBody := EmbeddingRequest{
		Model:  model,
		Prompt: text,
	}

	reqJSON, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	resp, err := c.Client.Post(
		fmt.Sprintf("%s/api/embeddings", c.BaseURL),
		"application/json",
		bytes.NewBuffer(reqJSON),
	)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := ioutil.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to generate embedding: %s (status: %d)", string(bodyBytes), resp.StatusCode)
	}

	var embeddingResp EmbeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&embeddingResp); err != nil {
		return nil, err
	}

	return embeddingResp.Embedding, nil
}

// GenerateCompletion génère une réponse basée sur un historique de messages
func (c *OllamaClient) GenerateCompletion(model string, messages []map[string]string) (string, error) {
	reqBody := map[string]interface{}{
		"model":    model,
		"messages": messages,
		"stream":   false,
	}

	reqJSON, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	resp, err := c.Client.Post(
		fmt.Sprintf("%s/api/chat", c.BaseURL),
		"application/json",
		bytes.NewBuffer(reqJSON),
	)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := ioutil.ReadAll(resp.Body)
		return "", fmt.Errorf("failed to generate completion: %s (status: %d)", string(bodyBytes), resp.StatusCode)
	}

	var chatResp map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return "", err
	}

	// Extraire la réponse
	if message, ok := chatResp["message"].(map[string]interface{}); ok {
		if content, ok := message["content"].(string); ok {
			return content, nil
		}
	}

	return "", fmt.Errorf("invalid response format")
}

// IsOllamaRunning checks if Ollama is installed and running
func (c *OllamaClient) IsOllamaRunning() (bool, error) {
	resp, err := c.Client.Get(fmt.Sprintf("%s/api/version", c.BaseURL))
	if err != nil {
		return false, fmt.Errorf("Ollama is not accessible: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("Ollama responded with error code: %d", resp.StatusCode)
	}
	
	return true, nil
}

// CheckOllamaAndModel verifies if Ollama is running and if the specified model is available
func (c *OllamaClient) CheckOllamaAndModel(modelName string) error {
	// Check if Ollama is running
	running, err := c.IsOllamaRunning()
	if err != nil {
		return fmt.Errorf("⚠️ Ollama is not installed or not running.\n"+
			"RLAMA requires Ollama to function.\n"+
			"Please install Ollama with: curl -fsSL https://ollama.com/install.sh | sh\n"+
			"Then start it before using RLAMA.")
	}
	
	if !running {
		return fmt.Errorf("⚠️ Ollama is not running.\n"+
			"Please start Ollama before using RLAMA.")
	}
	
	// Check if model is available (optional)
	// This check could be added here
	
	return nil
}

// RunHuggingFaceModel exécute un modèle Hugging Face en mode interactif
func (c *OllamaClient) RunHuggingFaceModel(modelPath string, quantization string) error {
	// Extraction du nom du modèle et quantization
	hfModelName := GetHuggingFaceModelName(modelPath)
	
	// Si quantization n'est pas spécifiée dans les arguments mais dans le chemin du modèle
	if quantization == "" {
		quantization = GetQuantizationFromModelRef(modelPath)
	}
	
	// Pull le modèle depuis Hugging Face si nécessaire
	err := c.PullHuggingFaceModel(hfModelName, quantization)
	if err != nil {
		return fmt.Errorf("error pulling Hugging Face model: %w", err)
	}
	
	// Construire le nom du modèle local
	localModelName := strings.Replace(hfModelName, "/", "-", -1)
	if quantization != "" {
		localModelName += "-" + quantization
	}
	
	// Mode interactif
	fmt.Printf("\nRunning model %s. Type your messages (or 'exit' to quit):\n\n", localModelName)
	
	scanner := bufio.NewReader(os.Stdin)
	messages := []map[string]string{
		{"role": "system", "content": "You are a helpful assistant."},
	}
	
	for {
		fmt.Print("> ")
		userInput, _ := scanner.ReadString('\n')
		userInput = strings.TrimSpace(userInput)
		
		if userInput == "exit" {
			break
		}
		
		if userInput == "" {
			continue
		}
		
		// Ajouter le message utilisateur
		messages = append(messages, map[string]string{
			"role":    "user",
			"content": userInput,
		})
		
		// Générer la réponse
		response, err := c.GenerateCompletion(localModelName, messages)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			continue
		}
		
		// Afficher la réponse
		fmt.Println("\n" + response + "\n")
		
		// Ajouter la réponse à l'historique
		messages = append(messages, map[string]string{
			"role":    "assistant",
			"content": response,
		})
		
		// Limiter la taille de l'historique pour éviter les débordements de contexte
		if len(messages) > 10 {
			// Garder le message système et les 9 derniers messages
			messages = append(messages[:1], messages[len(messages)-9:]...)
		}
	}
	
	return nil
}

// PullHuggingFaceModel pulls a Hugging Face model into Ollama without running it
func (c *OllamaClient) PullHuggingFaceModel(hfModelPath string, quantization string) error {
	modelRef := "hf.co/" + hfModelPath
	if quantization != "" {
		modelRef += ":" + quantization
	}
	
	cmd := exec.Command("ollama", "pull", modelRef)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	
	return cmd.Run()
}

// IsHuggingFaceModel checks if a model name is a Hugging Face model reference
func IsHuggingFaceModel(modelName string) bool {
	return strings.HasPrefix(modelName, "hf.co/") || 
		   strings.HasPrefix(modelName, "huggingface.co/")
}

// GetHuggingFaceModelName extracts the repository name from a Hugging Face model reference
func GetHuggingFaceModelName(modelRef string) string {
	// Strip any prefix
	modelName := modelRef
	if strings.HasPrefix(modelRef, "hf.co/") {
		modelName = strings.TrimPrefix(modelRef, "hf.co/")
	} else if strings.HasPrefix(modelRef, "huggingface.co/") {
		modelName = strings.TrimPrefix(modelRef, "huggingface.co/")
	}
	
	// Strip any quantization suffix
	if colonIdx := strings.Index(modelName, ":"); colonIdx != -1 {
		modelName = modelName[:colonIdx]
	}
	
	return modelName
}

// GetQuantizationFromModelRef extracts the quantization suffix from a model reference
func GetQuantizationFromModelRef(modelRef string) string {
	if colonIdx := strings.Index(modelRef, ":"); colonIdx != -1 {
		return modelRef[colonIdx+1:]
	}
	return ""
}

// ListModels récupère la liste des modèles disponibles sur le serveur Ollama
func (c *OllamaClient) ListModels() ([]string, error) {
	resp, err := c.Client.Get(fmt.Sprintf("%s/api/tags", c.BaseURL))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := ioutil.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to list models: %s (status: %d)", string(bodyBytes), resp.StatusCode)
	}

	var listResp map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		return nil, err
	}

	// Extraire les noms des modèles
	var models []string
	if modelsArray, ok := listResp["models"].([]interface{}); ok {
		for _, modelInfo := range modelsArray {
			if model, ok := modelInfo.(map[string]interface{}); ok {
				if name, ok := model["name"].(string); ok {
					models = append(models, name)
				}
			}
		}
	}

	return models, nil
}

// SendCompletion envoie une requête de complétion à Ollama avec un timeout
func (oc *OllamaClient) SendCompletion(prompt string, model string) (string, error) {
	// Créer un contexte avec timeout
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Préparer le payload de la requête
	payload := map[string]interface{}{
		"prompt": prompt,
		"model":  model,
		"stream": false,
	}

	reqJSON, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("erreur de sérialisation JSON: %w", err)
	}

	// Créer la requête avec le contexte (pour le timeout)
	reqURL := fmt.Sprintf("%s/api/generate", oc.BaseURL)
	req, err := http.NewRequestWithContext(ctx, "POST", reqURL, bytes.NewBuffer(reqJSON))
	if err != nil {
		return "", fmt.Errorf("erreur de création de la requête: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Exécuter la requête
	resp, err := oc.Client.Do(req)
	if err != nil {
		// Vérifier si l'erreur est due au timeout
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("timeout: la requête a pris trop de temps")
		}
		return "", fmt.Errorf("erreur d'envoi de la requête: %w", err)
	}
	defer resp.Body.Close()

	// Vérifier le code de statut
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := ioutil.ReadAll(resp.Body)
		return "", fmt.Errorf("erreur API (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	// Lire et traiter la réponse
	var genResp GenerationResponse
	if err := json.NewDecoder(resp.Body).Decode(&genResp); err != nil {
		return "", fmt.Errorf("erreur de décodage de la réponse: %w", err)
	}

	return genResp.Response, nil
}

// Ping vérifie la connexion à Ollama
func (oc *OllamaClient) Ping() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	// Ajouter le préfixe http:// à l'URL
	url := fmt.Sprintf("http://%s/api/version", oc.BaseURL)
	
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	
	resp, err := oc.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Ollama a répondu avec le code: %d", resp.StatusCode)
	}
	
	return nil
}

// GenerateWithTimeout génère du texte avec un timeout configurable
func (c *OllamaClient) GenerateWithTimeout(ctx context.Context, options GenerateOptions) (string, error) {
	// Préparer l'URL
	url := fmt.Sprintf("http://%s/api/generate", c.BaseURL)
	
	// Préparer la requête JSON
	requestBody, err := json.Marshal(options)
	if err != nil {
		return "", fmt.Errorf("error marshaling request: %w", err)
	}
	
	// Créer la requête HTTP
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(requestBody))
	if err != nil {
		return "", fmt.Errorf("error creating request: %w", err)
	}
	
	// Configurer les headers
	req.Header.Set("Content-Type", "application/json")
	
	// Envoyer la requête
	client := &http.Client{
		Timeout: 5 * time.Minute,
	}
	
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("error sending request: %w", err)
	}
	defer resp.Body.Close()
	
	// Vérifier le code de statut
	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return "", fmt.Errorf("error response from Ollama: %s", string(body))
	}
	
	// Lire la réponse ligne par ligne pour gérer le streaming
	var fullResponse strings.Builder
	scanner := bufio.NewScanner(resp.Body)
	
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		
		// Décoder chaque ligne comme un objet JSON séparé
		var response GenerationResponse
		if err := json.Unmarshal([]byte(line), &response); err != nil {
			return "", fmt.Errorf("error unmarshaling response: %w", err)
		}
		
		// Ajouter le texte de la réponse
		fullResponse.WriteString(response.Response)
	}
	
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("error reading response stream: %w", err)
	}
	
	return fullResponse.String(), nil
} 