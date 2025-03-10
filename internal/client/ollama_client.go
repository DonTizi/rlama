package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// Default connection settings for Ollama
const (
	DefaultOllamaHost = "localhost"
	DefaultOllamaPort = "11434"
)

// OllamaClient est un client pour l'API Ollama
type OllamaClient struct {
	BaseURL    string
	Client     *http.Client
	ModelSizes map[string]int // Cache for model sizes in billions of parameters
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

// GenerationRequest est la structure de la requête pour l'API /api/generate
type GenerationRequest struct {
	Model    string   `json:"model"`
	Prompt   string   `json:"prompt"`
	Context  []int    `json:"context,omitempty"`
	Options  Options  `json:"options,omitempty"`
	Format   string   `json:"format,omitempty"`
	Template string   `json:"template,omitempty"`
	Stream   bool     `json:"stream"`
}

// Options pour l'API generate
type Options struct {
	Temperature float64 `json:"temperature,omitempty"`
	TopP        float64 `json:"top_p,omitempty"`
	TopK        int     `json:"top_k,omitempty"`
	NumPredict  int     `json:"num_predict,omitempty"`
}

// GenerationResponse est la structure de la réponse de l'API /api/generate
type GenerationResponse struct {
	Model     string `json:"model"`
	Response  string `json:"response"`
	Context   []int  `json:"context"`
	CreatedAt string `json:"created_at"`
	Done      bool   `json:"done"`
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
	
	baseURL := fmt.Sprintf("http://%s:%s", host, port)
	
	return &OllamaClient{
		BaseURL:    baseURL,
		Client:     &http.Client{},
		ModelSizes: make(map[string]int), // Initialize the map
	}
}

// NewDefaultOllamaClient crée un nouveau client Ollama avec les valeurs par défaut
// Gardé pour compatibilité avec le code existant
func NewDefaultOllamaClient() *OllamaClient {
	return NewOllamaClient(DefaultOllamaHost, DefaultOllamaPort)
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

// GenerateCompletion génère une réponse pour le prompt donné
func (c *OllamaClient) GenerateCompletion(model, prompt string) (string, error) {
	reqBody := GenerationRequest{
		Model:  model,
		Prompt: prompt,
		Stream: false,
		Options: Options{
			Temperature: 0.7,
			TopP:        0.9,
			NumPredict:  1024,
		},
	}

	reqJSON, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	resp, err := c.Client.Post(
		fmt.Sprintf("%s/api/generate", c.BaseURL),
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

	var genResp GenerationResponse
	if err := json.NewDecoder(resp.Body).Decode(&genResp); err != nil {
		return "", err
	}

	return genResp.Response, nil
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

// GetModelSize returns the size of the specified model in billions of parameters
func (oc *OllamaClient) GetModelSize(modelName string) (int, error) {
	// Check cache first
	if size, exists := oc.ModelSizes[modelName]; exists {
		return size, nil
	}
	
	// Try to detect from model name using regex patterns
	if size := detectSizeFromName(modelName); size > 0 {
		oc.ModelSizes[modelName] = size
		return size, nil
	}
	
	// If we couldn't detect from name, make an educated guess
	estimatedSize := estimateModelSize(modelName)
	oc.ModelSizes[modelName] = estimatedSize
	return estimatedSize, nil
}

// Helper function to detect size from model name
func detectSizeFromName(name string) int {
	// Common patterns:
	// - llama-3b, llama-3-b, llama3b, llama3.2b, llama:3b
	// - 3b, 3B in the name
	patterns := []string{
		`(\d+)b`,
		`(\d+)[.-]b`,
		`(\d+)B`,
		`[:-](\d+)b`,
		`[:-](\d+)B`,
	}
	
	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(name)
		if len(matches) > 1 {
			size, err := strconv.Atoi(matches[1])
			if err == nil {
				return size
			}
		}
	}
	
	return 0 // Couldn't detect
}

// Estimate size based on model family
func estimateModelSize(name string) int {
	lowerName := strings.ToLower(name)
	
	// Check for common size indicators in the name
	if strings.Contains(lowerName, "tiny") || 
	   strings.Contains(lowerName, "mini") ||
	   strings.Contains(lowerName, "3b") ||
	   strings.Contains(lowerName, "3.2") {
		return 3
	} else if strings.Contains(lowerName, "7b") ||
	          strings.Contains(lowerName, "small") {
		return 7
	} else if strings.Contains(lowerName, "13b") ||
	          strings.Contains(lowerName, "medium") {
		return 13
	} else if strings.Contains(lowerName, "70b") ||
	          strings.Contains(lowerName, "large") {
		return 70
	} else {
		return 13 // Default to medium size if unknown
	}
} 