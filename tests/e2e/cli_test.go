package tests

import (
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test E2E qui exécute l'application complète
func TestBasicCliCommands(t *testing.T) {
	// Skip si un flag spécifique n'est pas passé, car ces tests sont lents
	if testing.Short() {
		t.Skip("Skipping E2E tests in short mode")
	}

	// Trouver le chemin de la racine du projet
	projectRoot, err := findProjectRoot()
	require.NoError(t, err, "Failed to find project root")

	// Trouver ou compiler le binaire
	binaryPath := os.Getenv("RLAMA_BINARY")
	if binaryPath == "" {
		// S'assurer que le dossier bin existe
		binDir := filepath.Join(projectRoot, "bin")
		err = os.MkdirAll(binDir, 0755)
		require.NoError(t, err, "Failed to create bin directory")

		binaryPath = filepath.Join(binDir, "rlama")
		if runtime.GOOS == "windows" {
			binaryPath += ".exe" // Ajouter l'extension .exe pour Windows
		}

		// Compiler le binaire
		cmd := exec.Command("go", "build", "-o", binaryPath, filepath.Join(projectRoot, "main.go"))
		cmd.Dir = projectRoot // Exécuter la commande depuis la racine du projet
		output, err := cmd.CombinedOutput()
		require.NoError(t, err, "Failed to build binary: %s", string(output))
	}

	// Vérifier que le binaire existe
	_, err = os.Stat(binaryPath)
	require.NoError(t, err, "Binary not found at %s", binaryPath)

	// Créer un répertoire de test
	testDir, err := os.MkdirTemp("", "rlama-e2e-test")
	require.NoError(t, err, "Failed to create temp dir")
	defer os.RemoveAll(testDir)

	// Test des commandes de base
	t.Run("BasicCommands", func(t *testing.T) {
		// Version
		cmd := exec.Command(binaryPath, "--version")
		output, err := cmd.CombinedOutput()
		assert.NoError(t, err)
		assert.Contains(t, string(output), "RLAMA version")

		// Help
		cmd = exec.Command(binaryPath, "--help")
		output, err = cmd.CombinedOutput()
		assert.NoError(t, err)
		assert.Contains(t, string(output), "Available Commands")
	})

	// Test de création et listing de profil
	t.Run("ProfileAddAndList", func(t *testing.T) {
		// Définir HOME pour isoler les tests
		oldHome := os.Getenv("HOME")
		os.Setenv("HOME", testDir)
		defer os.Setenv("HOME", oldHome)

		// Créer un profil
		cmd := exec.Command(binaryPath, "profile", "add", "e2e-test", "openai", "sk-test-key")
		output, err := cmd.CombinedOutput()
		assert.NoError(t, err, "Failed to add profile: %s", string(output))
		assert.Contains(t, string(output), "Profile 'e2e-test' for 'openai' added successfully")

		// Lister les profils
		cmd = exec.Command(binaryPath, "profile", "list")
		output, err = cmd.CombinedOutput()
		assert.NoError(t, err, "Failed to list profiles: %s", string(output))
		assert.Contains(t, string(output), "e2e-test")
		assert.Contains(t, string(output), "openai")

		// Test de suppression du profil
		deleteCmd := exec.Command(binaryPath, "profile", "delete", "e2e-test")
		deleteCmd.Stdin = strings.NewReader("y\n")
		output, err = deleteCmd.CombinedOutput()
		assert.NoError(t, err, "Failed to remove profile: %s", string(output))
		assert.Contains(t, string(output), "Profile 'e2e-test' deleted successfully")

		// Vérifier que le profil a bien été supprimé
		cmd = exec.Command(binaryPath, "profile", "list")
		output, err = cmd.CombinedOutput()
		assert.NoError(t, err)
		assert.NotContains(t, string(output), "e2e-test")
	})

	// Test des commandes RAG
	t.Run("RAGCommands", func(t *testing.T) {
		// Define HOME for isolation
		oldHome := os.Getenv("HOME")
		os.Setenv("HOME", testDir)
		defer os.Setenv("HOME", oldHome)

		// Create docs directory
		docsDir := filepath.Join(testDir, "docs")
		err := os.MkdirAll(docsDir, 0755)
		require.NoError(t, err, "Failed to create docs directory")

		// Create test file
		testFile := filepath.Join(docsDir, "test.txt")
		err = os.WriteFile(testFile, []byte("This is a test document for RAG testing."), 0644)
		assert.NoError(t, err, "Failed to create test file")

		// IMPORTANT: Ensure any existing test RAGs are deleted first
		deleteCmd := exec.Command(binaryPath, "delete", "test-rag")
		deleteCmd.Stdin = strings.NewReader("y\n") // Automatic yes to prompt
		deleteCmd.Run()                            // Ignore errors as it might not exist

		// Create profile for tests
		cmd := exec.Command(binaryPath, "profile", "add", "rag-test", "openai", "sk-test-key")
		output, err := cmd.CombinedOutput()
		assert.NoError(t, err, "Failed to create profile for RAG test")

		// Create RAG
		cmd = exec.Command(binaryPath, "rag", "llama2", "test-rag", docsDir, "--profile", "rag-test")
		output, err = cmd.CombinedOutput()
		assert.NoError(t, err, "Failed to create RAG: %s", string(output))
		assert.Contains(t, string(output), "RAG 'test-rag' created successfully")

		// Lister les RAGs
		cmd = exec.Command(binaryPath, "list")
		output, err = cmd.CombinedOutput()
		assert.NoError(t, err, "Failed to list RAGs: %s", string(output))
		assert.Contains(t, string(output), "test-rag")

		// Nettoyer
		deleteCmd = exec.Command(binaryPath, "delete", "test-rag")
		deleteCmd.Stdin = strings.NewReader("y\n")
		output, err = deleteCmd.CombinedOutput()
		assert.NoError(t, err, "Failed to remove RAG: %s", string(output))
		assert.Contains(t, string(output), "The RAG system 'test-rag' has been successfully deleted")

		// Nettoyer le profil
		deleteCmd = exec.Command(binaryPath, "profile", "delete", "rag-test")
		deleteCmd.Stdin = strings.NewReader("y\n")
		output, err = deleteCmd.CombinedOutput()
		assert.NoError(t, err, "Failed to remove profile: %s", string(output))
		assert.Contains(t, string(output), "Profile 'rag-test' deleted successfully")
	})

	// Test des commandes de gestion des documents
	t.Run("DocumentCommands", func(t *testing.T) {
		// Define HOME for isolation
		oldHome := os.Getenv("HOME")
		os.Setenv("HOME", testDir)
		defer os.Setenv("HOME", oldHome)

		// Create docs directory
		docsDir := filepath.Join(testDir, "docs")
		err := os.MkdirAll(docsDir, 0755)
		require.NoError(t, err, "Failed to create docs directory")

		// Create test file
		testFile := filepath.Join(docsDir, "test.txt")
		err = os.WriteFile(testFile, []byte("This is a test document for RAG testing."), 0644)
		assert.NoError(t, err, "Failed to create test file")

		// Clean up existing test RAGs
		deleteCmd := exec.Command(binaryPath, "delete", "doc-test-rag")
		deleteCmd.Stdin = strings.NewReader("y\n")
		deleteCmd.Run() // Ignore errors

		deleteCmd = exec.Command(binaryPath, "delete", "new-doc-rag")
		deleteCmd.Stdin = strings.NewReader("y\n")
		deleteCmd.Run() // Ignore errors

		// Create profile
		cmd := exec.Command(binaryPath, "profile", "add", "doc-test-profile", "openai", "sk-test-key")
		output, err := cmd.CombinedOutput()
		assert.NoError(t, err)

		// Create a RAG for testing
		cmd = exec.Command(binaryPath, "rag", "llama2", "doc-test-rag", docsDir, "--profile", "doc-test-profile")
		output, err = cmd.CombinedOutput()
		assert.NoError(t, err)
		assert.Contains(t, string(output), "RAG 'doc-test-rag' created successfully")

		// List-docs initial
		cmd = exec.Command(binaryPath, "list-docs", "doc-test-rag")
		output, err = cmd.CombinedOutput()
		t.Logf("Initial list-docs output: %s", string(output))
		assert.NoError(t, err)
		assert.Contains(t, string(output), "test.txt")

		// Add-docs pour le nouveau document
		newDocsDir := filepath.Join(testDir, "new-docs")
		err = os.MkdirAll(newDocsDir, 0755)
		assert.NoError(t, err)

		newFile := filepath.Join(newDocsDir, "new.txt")
		err = os.WriteFile(newFile, []byte("New test content"), 0644)
		assert.NoError(t, err)

		// Créer un nouveau RAG avec le nouveau document
		cmd = exec.Command(binaryPath, "rag", "llama2", "new-doc-rag", newDocsDir, "--profile", "doc-test-profile")
		output, err = cmd.CombinedOutput()
		t.Logf("new rag output: %s", string(output))
		assert.NoError(t, err)
		assert.Contains(t, string(output), "successfully")

		// List-docs pour vérifier
		cmd = exec.Command(binaryPath, "list-docs", "new-doc-rag")
		output, err = cmd.CombinedOutput()
		t.Logf("list-docs output: %s", string(output))
		assert.NoError(t, err)
		assert.Contains(t, string(output), "new.txt")

		// Nettoyer les deux RAGs
		deleteCmd = exec.Command(binaryPath, "delete", "doc-test-rag")
		deleteCmd.Stdin = strings.NewReader("y\n")
		output, err = deleteCmd.CombinedOutput()
		assert.NoError(t, err)

		deleteCmd = exec.Command(binaryPath, "delete", "new-doc-rag")
		deleteCmd.Stdin = strings.NewReader("y\n")
		output, err = deleteCmd.CombinedOutput()
		assert.NoError(t, err)

		// Nettoyer le profil
		deleteCmd = exec.Command(binaryPath, "profile", "delete", "doc-test-profile")
		deleteCmd.Stdin = strings.NewReader("y\n")
		output, err = deleteCmd.CombinedOutput()
		assert.NoError(t, err)
	})

	// Test des commandes de surveillance
	t.Run("WatchCommands", func(t *testing.T) {
		// Définir HOME pour isoler les tests
		oldHome := os.Getenv("HOME")
		os.Setenv("HOME", testDir)
		defer os.Setenv("HOME", oldHome)

		// Créer un dossier pour les documents
		docsDir := filepath.Join(testDir, "watch-docs")
		err := os.MkdirAll(docsDir, 0755)
		require.NoError(t, err, "Failed to create docs directory")

		// Créer un fichier de test
		testFile := filepath.Join(docsDir, "test.txt")
		err = os.WriteFile(testFile, []byte("This is a test document for watch testing."), 0644)
		assert.NoError(t, err, "Failed to create test file")

		// Créer un profil pour les tests
		cmd := exec.Command(binaryPath, "profile", "add", "watch-test-profile", "openai", "sk-test-key")
		output, err := cmd.CombinedOutput()
		assert.NoError(t, err)

		// Créer un RAG avant de configurer la surveillance
		cmd = exec.Command(binaryPath, "rag", "llama2", "watch-test-rag", docsDir, "--profile", "watch-test-profile")
		output, err = cmd.CombinedOutput()
		assert.NoError(t, err)
		assert.Contains(t, string(output), "RAG 'watch-test-rag' created successfully")

		// Watch
		cmd = exec.Command(binaryPath, "watch", "watch-test-rag", docsDir, "60")
		output, err = cmd.CombinedOutput()
		assert.NoError(t, err)
		assert.Contains(t, string(output), "Directory watching set up")

		// Check-watched
		cmd = exec.Command(binaryPath, "check-watched", "watch-test-rag")
		output, err = cmd.CombinedOutput()
		assert.NoError(t, err)

		// Watch-off
		cmd = exec.Command(binaryPath, "watch-off", "watch-test-rag")
		output, err = cmd.CombinedOutput()
		assert.NoError(t, err)
		assert.Contains(t, string(output), "Directory watching disabled")

		// Nettoyer
		deleteCmd := exec.Command(binaryPath, "delete", "watch-test-rag")
		deleteCmd.Stdin = strings.NewReader("y\n")
		output, err = deleteCmd.CombinedOutput()
		assert.NoError(t, err)

		// Nettoyer le profil
		deleteCmd = exec.Command(binaryPath, "profile", "delete", "watch-test-profile")
		deleteCmd.Stdin = strings.NewReader("y\n")
		output, err = deleteCmd.CombinedOutput()
		assert.NoError(t, err)
	})

	// Test des commandes de mise à jour
	t.Run("UpdateCommands", func(t *testing.T) {
		// Définir HOME pour isoler les tests
		oldHome := os.Getenv("HOME")
		os.Setenv("HOME", testDir)
		defer os.Setenv("HOME", oldHome)

		// Créer un dossier pour les documents
		docsDir := filepath.Join(testDir, "update-docs")
		err := os.MkdirAll(docsDir, 0755)
		require.NoError(t, err, "Failed to create docs directory")

		// Créer un fichier de test
		testFile := filepath.Join(docsDir, "test.txt")
		err = os.WriteFile(testFile, []byte("This is a test document for update testing."), 0644)
		assert.NoError(t, err, "Failed to create test file")

		// Créer un profil pour les tests
		cmd := exec.Command(binaryPath, "profile", "add", "update-test-profile", "openai", "sk-test-key")
		output, err := cmd.CombinedOutput()
		assert.NoError(t, err)

		// Créer un RAG à mettre à jour
		cmd = exec.Command(binaryPath, "rag", "llama2", "test-rag", docsDir, "--profile", "update-test-profile")
		output, err = cmd.CombinedOutput()
		assert.NoError(t, err)

		// Update-model
		cmd = exec.Command(binaryPath, "update-model", "test-rag", "llama3")
		output, err = cmd.CombinedOutput()
		assert.NoError(t, err)
		assert.Contains(t, string(output), "Successfully updated")

		// Nettoyer
		deleteCmd := exec.Command(binaryPath, "delete", "test-rag")
		deleteCmd.Stdin = strings.NewReader("y\n")
		output, err = deleteCmd.CombinedOutput()
		assert.NoError(t, err)

		// Nettoyer le profil
		deleteCmd = exec.Command(binaryPath, "profile", "delete", "update-test-profile")
		deleteCmd.Stdin = strings.NewReader("y\n")
		output, err = deleteCmd.CombinedOutput()
		assert.NoError(t, err)
	})

	// Test des commandes Hugging Face
	t.Run("HuggingFaceCommands", func(t *testing.T) {
		// Hf-browse
		cmd := exec.Command(binaryPath, "hf-browse", "llama3")
		output, err := cmd.CombinedOutput()
		assert.NoError(t, err)
		assert.Contains(t, string(output), "To use a Hugging Face model")
	})

	// Test du serveur API
	t.Run("APIServer", func(t *testing.T) {
		// Démarrer le serveur en arrière-plan
		cmd := exec.Command(binaryPath, "api", "--port", "11249")
		err := cmd.Start()
		assert.NoError(t, err)

		// Ensure cleanup
		defer func() {
			if cmd.Process != nil {
				cmd.Process.Kill()
			}
		}()

		// Wait and retry health check
		maxRetries := 5
		var lastErr error
		for i := 0; i < maxRetries; i++ {
			time.Sleep(2 * time.Second)
			resp, err := http.Get("http://localhost:11249/health")
			if err == nil && resp.StatusCode == http.StatusOK {
				resp.Body.Close()
				return
			}
			lastErr = err
		}
		t.Fatalf("Server failed to start after %d retries: %v", maxRetries, lastErr)
	})
}

// findProjectRoot remonte les dossiers jusqu'à trouver go.mod
func findProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", os.ErrNotExist
		}
		dir = parent
	}
}
