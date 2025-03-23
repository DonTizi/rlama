package tests

import (
	"testing"
	"time"

	"github.com/dontizi/rlama/internal/domain"
	"github.com/dontizi/rlama/internal/repository"
	"github.com/stretchr/testify/assert"
)

func TestProfiles(t *testing.T) {
	// Créer un nom unique pour ne pas interférer avec les profils existants
	testProfileName := "test-profile-for-automated-testing"
	
	// Obtenir le repository des profils
	profileRepo := repository.NewProfileRepository()
	
	// Nettoyage préalable au cas où
	_ = profileRepo.Delete(testProfileName)
	
	// Test d'ajout d'un profil
	t.Run("CreateProfile", func(t *testing.T) {
		// Créer un nouveau profil avec la structure appropriée
		profile := &domain.APIProfile{
			Name:      testProfileName,
			Provider:  "openai",
			APIKey:    "test-api-key",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		
		// Sauvegarder le profil
		err := profileRepo.Save(profile)
		assert.NoError(t, err)
		
		// Vérifier que le profil existe
		exists := profileRepo.Exists(testProfileName)
		assert.True(t, exists, "Le profil devrait exister après sa création")
	})
	
	// Test de récupération d'un profil
	t.Run("GetProfile", func(t *testing.T) {
		profile, err := profileRepo.Load(testProfileName)
		assert.NoError(t, err)
		assert.Equal(t, testProfileName, profile.Name)
		assert.Equal(t, "test-api-key", profile.APIKey)
		assert.Equal(t, "openai", profile.Provider)
	})
	
	// Test de listing des profils
	t.Run("ListProfiles", func(t *testing.T) {
		profileNames, err := profileRepo.ListAll()
		assert.NoError(t, err)
		
		assert.Contains(t, profileNames, testProfileName, "Le profil de test devrait apparaître dans la liste")
	})
	
	// Test de suppression d'un profil
	t.Run("DeleteProfile", func(t *testing.T) {
		err := profileRepo.Delete(testProfileName)
		assert.NoError(t, err)
		
		// Vérifier que le profil n'existe plus
		exists := profileRepo.Exists(testProfileName)
		assert.False(t, exists, "Le profil ne devrait plus exister après sa suppression")
	})
}