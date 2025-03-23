package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// BGERerankerClient handles interactions with the BGE Reranker model via Python
type BGERerankerClient struct {
	modelName string
	useFP16   bool
}

// NewBGERerankerClient creates a new instance of BGERerankerClient
func NewBGERerankerClient(modelName string) *BGERerankerClient {
	client := &BGERerankerClient{
		modelName: modelName,
		useFP16:   true,
	}

	// Check dependencies and model
	if err := client.CheckDependencies(); err != nil {
		fmt.Printf("⚠️ Warning: %v\n", err)
		fmt.Println("To install dependencies, run: rlama install-dependencies")
	} else {
		// Only check model if dependencies are available
		if err := client.CheckModelExists(); err != nil {
			fmt.Printf("⚠️ Warning: %v\n", err)
			fmt.Println("The BGE Reranker model might not be accessible. Check your internet connection and model name.")
		}
	}

	return client
}

// GetModelName returns the model name used by this client
func (c *BGERerankerClient) GetModelName() string {
	return c.modelName
}

// ComputeScores calculates relevance scores between queries and passages
func (c *BGERerankerClient) ComputeScores(pairs [][]string, normalize bool) ([]float64, error) {
	// Convert Go boolean to Python boolean
	normalizeStr := "False"
	if normalize {
		normalizeStr = "True"
	}

	useFP16Str := "False"
	if c.useFP16 {
		useFP16Str = "True"
	}

	pythonScript := fmt.Sprintf(`
import sys
import os
import json
import warnings

# Suppress warnings (to avoid tokenizer warnings in output)
warnings.filterwarnings("ignore")
os.environ["TOKENIZERS_PARALLELISM"] = "false"

try:
    from FlagEmbedding import FlagReranker
    
    # Load the input data
    input_data = json.loads(sys.stdin.read())
    pairs = input_data["pairs"]
    normalize = input_data["normalize"]
    
    # Initialize the reranker
    reranker = FlagReranker('%s', use_fp16=%s)
    
    # Compute scores
    scores = reranker.compute_score(pairs, normalize=%s)
    
    # Convert scores to list if it's not already
    if not isinstance(scores, list):
        scores = [float(scores)]
    else:
        scores = [float(score) for score in scores]
    
    # Output the results as JSON
    print(json.dumps({"scores": scores}))
except Exception as e:
    print(json.dumps({"error": str(e)}))
    sys.exit(1)
`, c.modelName, useFP16Str, normalizeStr)

	// Prepare input data
	inputData := map[string]interface{}{
		"pairs":     pairs,
		"normalize": normalize,
	}
	inputJSON, err := json.Marshal(inputData)
	if err != nil {
		return nil, fmt.Errorf("error marshaling input data: %w", err)
	}

	// Execute the Python script
	cmd := exec.Command("python", "-c", pythonScript)
	cmd.Stdin = strings.NewReader(string(inputJSON))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("error executing Python script: %w, output: %s", err, string(output))
	}

	// Extract just the JSON part from the output
	jsonStart := bytes.LastIndex(output, []byte("{"))
	if jsonStart < 0 {
		return nil, fmt.Errorf("no JSON found in output: %s", string(output))
	}
	jsonData := output[jsonStart:]

	// Parse the output
	var result map[string]interface{}
	if err := json.Unmarshal(jsonData, &result); err != nil {
		return nil, fmt.Errorf("error parsing Python output: %w, output: %s", err, string(output))
	}

	// Check for error
	if errorMsg, ok := result["error"].(string); ok {
		return nil, fmt.Errorf("Python script error: %s", errorMsg)
	}

	// Extract scores
	scoresInterface, ok := result["scores"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid scores format in Python output")
	}

	scores := make([]float64, len(scoresInterface))
	for i, s := range scoresInterface {
		scores[i], ok = s.(float64)
		if !ok {
			return nil, fmt.Errorf("invalid score type at index %d", i)
		}
	}

	return scores, nil
}

// CheckDependencies checks if FlagEmbedding is installed
func (c *BGERerankerClient) CheckDependencies() error {
	checkScript := `
import importlib.util
import sys

# Check if FlagEmbedding is installed
flag_spec = importlib.util.find_spec("FlagEmbedding")
if flag_spec is None:
    print("not_installed")
    sys.exit(0)
else:
    print("installed")
    sys.exit(0)
`
	cmd := exec.Command("python", "-c", checkScript)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("error checking Python dependencies: %w", err)
	}

	result := strings.TrimSpace(string(output))
	if result == "not_installed" {
		return fmt.Errorf("FlagEmbedding library is not installed. Run 'rlama install-dependencies' to install it")
	}

	return nil
}

// CheckModelExists verifies that the model exists and is accessible
func (c *BGERerankerClient) CheckModelExists() error {
	pythonScript := `
import sys
import json
try:
    from FlagEmbedding import FlagReranker
    
    # Just initialize the model to check if it exists
    model_name = "BAAI/bge-reranker-v2-m3"
    reranker = FlagReranker(model_name, use_fp16=True)
    print(json.dumps({"success": True}))
except Exception as e:
    print(json.dumps({"error": str(e)}))
    sys.exit(1)
`
	cmd := exec.Command("python", "-c", pythonScript)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("error executing Python script: %w, output: %s", err, string(output))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(output, &result); err != nil {
		return fmt.Errorf("error parsing Python output: %w, output: %s", err, string(output))
	}

	if _, ok := result["error"]; ok {
		return fmt.Errorf("model check failed: %s", result["error"])
	}

	return nil
}
