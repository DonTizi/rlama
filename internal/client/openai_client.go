package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	"github.com/dontizi/rlama/internal/repository"
)

// OpenAIClient is a client for the OpenAI API
type OpenAIClient struct {
	BaseURL string
	APIKey  string
	Client  *http.Client
}

// OpenAICompletionRequest is the structure for completion requests to OpenAI
type OpenAICompletionRequest struct {
	Model       string          `json:"model"`
	Messages    []OpenAIMessage `json:"messages"`
	Temperature float64         `json:"temperature,omitempty"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
}

// OpenAIMessage represents a message in the format expected by OpenAI
type OpenAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// OpenAICompletionResponse is the structure of the OpenAI API response
type OpenAICompletionResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index        int           `json:"index"`
		Message      OpenAIMessage `json:"message"`
		FinishReason string        `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// NewOpenAIClient creates a new OpenAI client
func NewOpenAIClient() *OpenAIClient {
	// Use API key from environment
	apiKey := os.Getenv("OPENAI_API_KEY")

	return &OpenAIClient{
		BaseURL: "https://api.openai.com/v1",
		APIKey:  apiKey,
		Client:  &http.Client{},
	}
}

// NewOpenAIClientWithProfile creates a new OpenAI client with a specific profile
func NewOpenAIClientWithProfile(profileName string) (*OpenAIClient, error) {
	// Create a profile repository
	profileRepo := repository.NewProfileRepository()

	// If no profile is specified, use the environment variable
	if profileName == "" {
		apiKey := os.Getenv("OPENAI_API_KEY")
		return &OpenAIClient{
			BaseURL: "https://api.openai.com/v1",
			APIKey:  apiKey,
			Client:  &http.Client{},
		}, nil
	}

	// Load the specified profile
	profile, err := profileRepo.Load(profileName)
	if err != nil {
		return nil, fmt.Errorf("error loading profile '%s': %w", profileName, err)
	}

	// Check that it's an OpenAI profile
	if profile.Provider != "openai" {
		return nil, fmt.Errorf("profile '%s' is not an OpenAI profile", profileName)
	}

	// Update last used date
	profile.LastUsedAt = time.Now()
	profileRepo.Save(profile)

	return &OpenAIClient{
		BaseURL: "https://api.openai.com/v1",
		APIKey:  profile.APIKey,
		Client:  &http.Client{},
	}, nil
}

// GenerateCompletion generates a response from a prompt with OpenAI
func (c *OpenAIClient) GenerateCompletion(model, prompt string) (string, error) {
	if c.APIKey == "" {
		return "", fmt.Errorf("OPENAI_API_KEY environment variable not set")
	}

	// Build the request
	reqBody := OpenAICompletionRequest{
		Model: model,
		Messages: []OpenAIMessage{
			{
				Role:    "system",
				Content: "You are a helpful assistant that answers questions based on the provided context.",
			},
			{
				Role:    "user",
				Content: prompt,
			},
		},
		Temperature: 0.7, // Default value, can be configured
	}

	reqJSON, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	// Create the HTTP request
	req, err := http.NewRequest("POST", fmt.Sprintf("%s/chat/completions", c.BaseURL), bytes.NewBuffer(reqJSON))
	if err != nil {
		return "", err
	}

	// Add necessary headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.APIKey))

	// Send the request
	resp, err := c.Client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := ioutil.ReadAll(resp.Body)
		return "", fmt.Errorf("failed to generate completion: %s (status: %d)", string(bodyBytes), resp.StatusCode)
	}

	// Decode the response
	var completionResp OpenAICompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&completionResp); err != nil {
		return "", err
	}

	// Check that there is at least one choice
	if len(completionResp.Choices) == 0 {
		return "", fmt.Errorf("no response generated")
	}

	// Return the content of the response
	return completionResp.Choices[0].Message.Content, nil
}

// CheckOpenAIAndModel checks if OpenAI is accessible and if the model is available
func (c *OpenAIClient) CheckOpenAIAndModel(modelName string) error {
	if c.APIKey == "" {
		return fmt.Errorf("⚠️ OPENAI_API_KEY environment variable not set.\n" +
			"Please set your OpenAI API key before using OpenAI models.")
	}

	// We could check the validity of the model here
	// but for now, we assume the model is valid if the API key is set

	return nil
}
