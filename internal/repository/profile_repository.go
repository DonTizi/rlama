package repository

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/dontizi/rlama/internal/domain"
)

// ProfileRepository gère le stockage des profils API
type ProfileRepository struct {
	basePath string
}

// NewProfileRepository crée une nouvelle instance de ProfileRepository
func NewProfileRepository() *ProfileRepository {
	// use ~/.rlama/profiles as default directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "."
	}

	basePath := filepath.Join(homeDir, ".rlama", "profiles")

	// Create the directory if it doesn't exist
	os.MkdirAll(basePath, 0755)

	return &ProfileRepository{
		basePath: basePath,
	}
}

// getProfilePath returns the full path for a given profile
func (r *ProfileRepository) getProfilePath(name string) string {
	return filepath.Join(r.basePath, name+".json")
}

// Exists checks if a profile exists
func (r *ProfileRepository) Exists(name string) bool {
	_, err := os.Stat(r.getProfilePath(name))
	return err == nil
}

// Save saves a profile
func (r *ProfileRepository) Save(profile *domain.APIProfile) error {
	profile.UpdatedAt = time.Now()

	data, err := json.MarshalIndent(profile, "", "  ")
	if err != nil {
		return fmt.Errorf("error marshaling profile: %w", err)
	}

	err = os.WriteFile(r.getProfilePath(profile.Name), data, 0644)
	if err != nil {
		return fmt.Errorf("error writing profile file: %w", err)
	}

	return nil
}

// Load loads a profile
func (r *ProfileRepository) Load(name string) (*domain.APIProfile, error) {
	data, err := os.ReadFile(r.getProfilePath(name))
	if err != nil {
		return nil, fmt.Errorf("error reading profile '%s': %w", name, err)
	}

	var profile domain.APIProfile
	if err := json.Unmarshal(data, &profile); err != nil {
		return nil, fmt.Errorf("error unmarshaling profile '%s': %w", name, err)
	}

	return &profile, nil
}

// Delete deletes a profile
func (r *ProfileRepository) Delete(name string) error {
	if !r.Exists(name) {
		return fmt.Errorf("profile '%s' does not exist", name)
	}

	err := os.Remove(r.getProfilePath(name))
	if err != nil {
		return fmt.Errorf("error deleting profile '%s': %w", name, err)
	}

	return nil
}

// ListAll returns a list of all profiles
func (r *ProfileRepository) ListAll() ([]string, error) {
	files, err := os.ReadDir(r.basePath)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("error reading profiles directory: %w", err)
	}

	var profileNames []string
	for _, file := range files {
		if !file.IsDir() && filepath.Ext(file.Name()) == ".json" {
			// Remove the .json extension
			name := file.Name()[:len(file.Name())-5]
			profileNames = append(profileNames, name)
		}
	}

	return profileNames, nil
}
