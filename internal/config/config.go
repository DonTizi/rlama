package config

import (
	"os"
	"path/filepath"
)

var (
	// DataDir is the directory where RLAMA stores all its data
	DataDir string
)

func init() {
	// Check for data directory from environment variable first
	DataDir = os.Getenv("RLAMA_DATA_DIR")
	
	// If not set in environment, use default location in user's home directory
	if DataDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			panic("Could not determine user home directory: " + err.Error())
		}
		DataDir = filepath.Join(homeDir, ".rlama")
	}

	// Ensure the data directory exists
	if err := os.MkdirAll(DataDir, 0755); err != nil {
		panic("Could not create data directory: " + err.Error())
	}
}

// GetDataDir returns the data directory path
// Priority: environment variable > default (~/.rlama)
func GetDataDir() string {
	// Check if environment variable is set
	envDataDir := os.Getenv("RLAMA_DATA_DIR")
	if envDataDir != "" {
		return envDataDir
	}
	
	// Use default location
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ".rlama" // Fallback to current directory
	}
	
	return filepath.Join(homeDir, ".rlama")
} 