package tests

import (
	"os"
	"testing"
	"time"

	"github.com/dontizi/rlama/internal/domain"
	"github.com/dontizi/rlama/internal/repository"
	"github.com/stretchr/testify/assert"
)

func TestProfileRepository(t *testing.T) {
	// Create a temporary test directory
	testDir, err := os.MkdirTemp("", "rlama-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(testDir)

	// Initialize the repository
	repo := repository.NewProfileRepository()

	// Test profile creation and saving
	t.Run("SaveAndLoad", func(t *testing.T) {
		profile := &domain.APIProfile{
			Name:      "test-profile",
			Provider:  "openai",
			APIKey:    "sk-test-key",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		// Save the profile
		err := repo.Save(profile)
		assert.NoError(t, err)

		// Verify file exists
		assert.True(t, repo.Exists("test-profile"))

		// Load the profile
		loaded, err := repo.Load("test-profile")
		assert.NoError(t, err)
		assert.Equal(t, profile.Name, loaded.Name)
		assert.Equal(t, profile.Provider, loaded.Provider)
		assert.Equal(t, profile.APIKey, loaded.APIKey)
	})

	t.Run("Delete", func(t *testing.T) {
		profile := &domain.APIProfile{
			Name:      "delete-test",
			Provider:  "openai",
			APIKey:    "sk-delete-key",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		// Save then delete
		err := repo.Save(profile)
		assert.NoError(t, err)
		assert.True(t, repo.Exists("delete-test"))

		err = repo.Delete("delete-test")
		assert.NoError(t, err)
		assert.False(t, repo.Exists("delete-test"))
	})

	t.Run("ListAll", func(t *testing.T) {
		// Create some test profiles
		profiles := []*domain.APIProfile{
			{Name: "profile1", Provider: "openai", APIKey: "key1"},
			{Name: "profile2", Provider: "openai", APIKey: "key2"},
		}

		for _, p := range profiles {
			err := repo.Save(p)
			assert.NoError(t, err)
		}

		// List all profiles
		list, err := repo.ListAll()
		assert.NoError(t, err)
		assert.GreaterOrEqual(t, len(list), 2)
		assert.Contains(t, list, "profile1")
		assert.Contains(t, list, "profile2")
	})
}