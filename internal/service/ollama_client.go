package service

import (
	"regexp"
	"strconv"
	"strings"
)

type OllamaClient struct {
	ModelSizes map[string]int // Cache for model sizes in billions of parameters
}

func (oc *OllamaClient) GetModelSize(modelName string) (int, error) {
	info, err := oc.GetModelInfo(modelName)
	if err != nil {
		// Fallback to parsing model name
		return parseModelSize(modelName), nil
	}
	
	// Extract size from model info
	if strings.Contains(strings.ToLower(info.Details.Size), "billion") {
		return parseModelSize(info.Details.Size), nil
	}
	return 7, nil // Default to 7B if unknown
}

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

func estimateModelSize(name string) int {
	// Default estimates for common models
	if strings.Contains(strings.ToLower(name), "tiny") || 
	   strings.Contains(strings.ToLower(name), "mini") {
		return 3
	} else if strings.Contains(strings.ToLower(name), "small") {
		return 7
	} else if strings.Contains(strings.ToLower(name), "medium") {
		return 13
	} else {
		return 13 // Default to medium size if unknown
	}
}

func parseModelSize(modelName string) int {
	// Extract size from model name like "llama2:7b"
	sizePart := ""
	parts := strings.Split(modelName, ":")
	if len(parts) > 1 {
		sizePart = parts[len(parts)-1]
	} else {
		sizePart = modelName
	}

	// Remove non-numeric characters
	sizeStr := ""
	for _, c := range sizePart {
		if c >= '0' && c <= '9' {
			sizeStr += string(c)
		}
	}

	if sizeStr == "" {
		return 7 // Default to 7B
	}
	
	size, _ := strconv.Atoi(sizeStr)
	return size
}

// Add this temporary implementation
func (oc *OllamaClient) GetModelInfo(modelName string) (*ModelInfo, error) {
	// Placeholder implementation
	return &ModelInfo{
		Details: struct {
			Size string
		}{
			Size: "7B",
		},
	}, nil
}

// Add ModelInfo struct
type ModelInfo struct {
	Details struct {
		Size string
	}
} 