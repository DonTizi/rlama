package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var forceUpdate bool

type GitHubRelease struct {
	TagName string `json:"tag_name"`
	Name    string `json:"name"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Check and install RLAMA updates",
	Long: `Check if a new version of RLAMA is available and install it if so.
Example: rlama update

By default, the command asks for confirmation before installing the update.
Use the --force flag to update without confirmation.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Checking for RLAMA updates...")

		// Use the doUpdate function which properly handles Windows updates
		return doUpdate("", forceUpdate)
	},
}

// checkForUpdates checks if updates are available by querying the GitHub API
func checkForUpdates() (*GitHubRelease, bool, error) {
	// Query the GitHub API to get the latest release
	resp, err := http.Get("https://api.github.com/repos/dontizi/rlama/releases/latest")
	if err != nil {
		return nil, false, err
	}
	defer resp.Body.Close()

	// Parse the JSON response
	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, false, err
	}

	// Check if the version is newer
	latestVersion := strings.TrimPrefix(release.TagName, "v")
	hasUpdates := latestVersion != Version

	return &release, hasUpdates, nil
}

func init() {
	rootCmd.AddCommand(updateCmd)
	updateCmd.Flags().BoolVarP(&forceUpdate, "force", "f", false, "Update without asking for confirmation")
}

// Fonction modifiée pour gérer le cas spécifique de Windows
func doUpdate(version string, force bool) error {
	// Si aucune version n'est fournie, obtenir la dernière version
	var latestVersion string
	var err error
	if version == "" {
		latestVersion, err = getLatestVersion()
		if err != nil {
			return fmt.Errorf("error checking for updates: %w", err)
		}
		version = latestVersion
	}

	// Vérifier si une mise à jour est nécessaire
	currentVersion := Version
	if currentVersion == version && !force {
		fmt.Printf("You are already using the latest version of RLAMA (%s)\n", currentVersion)
		return nil
	}

	// Demander confirmation, sauf si --force est utilisé
	if !force {
		fmt.Printf("A new version of RLAMA is available (%s). Do you want to install it? (y/n): ", version)
		var response string
		_, err := fmt.Scanln(&response)
		if err != nil || (strings.ToLower(response) != "y" && strings.ToLower(response) != "yes") {
			fmt.Println("Update cancelled.")
			return nil
		}
	}

	fmt.Printf("Installing RLAMA %s...\n", version)

	// Obtenir le chemin de l'exécutable actuel
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("error getting executable path: %w", err)
	}

	// Créer un répertoire pour les fichiers de mise à jour si nécessaire
	updateDir := filepath.Dir(execPath)
	if err := os.MkdirAll(updateDir, 0755); err != nil {
		return fmt.Errorf("error creating update directory: %w", err)
	}

	// Télécharger la nouvelle version
	binaryURL := fmt.Sprintf("https://github.com/dontizi/rlama/releases/download/v%s/rlama_%s_%s", version, runtime.GOOS, runtime.GOARCH)
	if runtime.GOOS == "windows" {
		binaryURL += ".exe"
	}

	// Chemin pour le nouveau binaire
	newBinaryPath := execPath + ".new"

	// Télécharger le nouveau binaire
	if err := downloadFile(binaryURL, newBinaryPath); err != nil {
		// Nettoyer en cas d'erreur
		os.Remove(newBinaryPath)
		return fmt.Errorf("error downloading update: %w", err)
	}

	// Rendre le nouveau binaire exécutable
	if err := os.Chmod(newBinaryPath, 0755); err != nil {
		os.Remove(newBinaryPath)
		return fmt.Errorf("error setting permissions: %w", err)
	}

	// Sous Windows, nous devons utiliser une approche différente
	if runtime.GOOS == "windows" {
		return doWindowsUpdate(execPath, newBinaryPath)
	}

	// Sur les autres plateformes, nous pouvons remplacer directement
	if err := os.Rename(newBinaryPath, execPath); err != nil {
		os.Remove(newBinaryPath)
		return fmt.Errorf("error replacing binary: %w", err)
	}

	fmt.Printf("Successfully updated to RLAMA %s!\n", version)
	return nil
}

// Nouvelle fonction pour gérer la mise à jour sous Windows
func doWindowsUpdate(originalPath, newPath string) error {
	// Create temporary batch script in a location we know exists
	userProfile := os.Getenv("USERPROFILE")
	if userProfile == "" {
		userProfile = os.TempDir()
	}

	tempDir := filepath.Join(userProfile, "AppData", "Local", "Temp")
	os.MkdirAll(tempDir, 0755) // Ensure the directory exists

	tempBatchFile := filepath.Join(tempDir, "rlama_update.bat")

	// Simple batch script that waits for the process to end and then replaces the file
	batchContent := `@echo off
echo Waiting for RLAMA to close...
echo Please close any running instances of RLAMA.

:checkprocess
tasklist /fi "imagename eq rlama.exe" | find "rlama.exe" >nul
if %errorlevel% equ 0 (
    timeout /t 2 >nul
    goto checkprocess
)

echo RLAMA process exited, proceeding with update...
echo.

set retryCount=0
:retry
set /a retryCount+=1
echo Attempt %retryCount% to replace the binary...

move /y "%s" "%s" >nul 2>&1
if errorlevel 1 (
    echo Failed to replace binary, retrying in 3 seconds...
    if %retryCount% geq 10 (
        echo Maximum retry attempts exceeded.
        echo.
        echo Please manually run this command to complete the update:
        echo move /y "%s" "%s"
        echo.
        echo Or try running Command Prompt as Administrator.
        echo.
        pause
        exit /b 1
    )
    timeout /t 3 >nul
    goto retry
)

echo.
echo Update successful!
echo RLAMA has been updated to the new version.
echo.
pause
`
	batchScript := fmt.Sprintf(batchContent, newPath, originalPath, newPath, originalPath)

	// Write the batch script
	if err := os.WriteFile(tempBatchFile, []byte(batchScript), 0644); err != nil {
		return fmt.Errorf("error creating update script: %w", err)
	}

	// Run the batch script in a new window
	cmd := exec.Command("cmd", "/c", "start", "RLAMA Update", tempBatchFile)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("error starting update process: %w", err)
	}

	fmt.Println("Update will complete after you exit the RLAMA application.")
	fmt.Println("Please close all RLAMA windows and processes to complete the update.")
	fmt.Println("A separate window is now monitoring the update process.")

	return nil
}

// getLatestVersion récupère la dernière version disponible depuis GitHub
func getLatestVersion() (string, error) {
	// Query the GitHub API to get the latest release
	resp, err := http.Get("https://api.github.com/repos/dontizi/rlama/releases/latest")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// Parse the JSON response
	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", err
	}

	// Return the version without the 'v' prefix
	return strings.TrimPrefix(release.TagName, "v"), nil
}

// downloadFile downloads a file from a URL to a local path
// with better error handling and retry attempts
func downloadFile(url string, filepath string) error {
	// Create an HTTP client with timeout
	client := &http.Client{
		Timeout: 120 * time.Second, // 2 minute timeout
	}

	// Create the file
	out, err := os.Create(filepath)
	if err != nil {
		return fmt.Errorf("error creating file: %w", err)
	}
	defer out.Close()

	// Maximum retry attempts
	maxRetries := 3
	retryDelay := 2 * time.Second

	var lastErr error
	for i := 0; i < maxRetries; i++ {
		if i > 0 {
			fmt.Printf("Retry attempt %d/%d...\n", i+1, maxRetries)
			time.Sleep(retryDelay)
			// Increase delay for next retry
			retryDelay *= 2
		}

		// Get the data
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			lastErr = err
			continue
		}

		// Add a user agent to avoid some download restrictions
		req.Header.Set("User-Agent", "rlama-updater/1.0")

		// Send the request
		resp, err := client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("error downloading file: %w", err)
			continue
		}
		defer resp.Body.Close()

		// Check server response
		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("bad status: %s", resp.Status)
			continue
		}

		// Reset file position
		out.Seek(0, 0)

		// Create a progress bar if the file is large enough
		progressReader := io.Reader(resp.Body)
		if resp.ContentLength > 1024*1024 { // If larger than 1MB
			fmt.Printf("Downloading update (%d MB)...\n", resp.ContentLength/(1024*1024))
		}

		// Write the body to file
		_, err = io.Copy(out, progressReader)
		if err != nil {
			lastErr = fmt.Errorf("error saving file: %w", err)
			continue
		}

		// Success
		return nil
	}

	// If we get here, all retries failed
	return fmt.Errorf("download failed after %d attempts: %w", maxRetries, lastErr)
}
