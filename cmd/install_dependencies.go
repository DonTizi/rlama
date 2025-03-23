package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/spf13/cobra"
)

var installDependenciesCmd = &cobra.Command{
	Use:   "install-dependencies",
	Short: "Install necessary dependencies for RLAMA",
	Long:  `Install system and Python dependencies for optimal RLAMA performance, including the BGE reranker.`,
	Run: func(cmd *cobra.Command, args []string) {
		installDependencies()
	},
}

func init() {
	rootCmd.AddCommand(installDependenciesCmd)
}

func installDependencies() {
	fmt.Println("Installing dependencies...")

	// Find the installation script path
	execPath, err := os.Executable()
	if err != nil {
		fmt.Printf("⚠️ Error determining executable path: %v\n", err)

		// Use an alternative solution
		installDependenciesFallback()
		return
	}

	// The scripts directory is presumed to be in the same directory as the executable
	// or in the parent directory for development environments
	scriptDir := filepath.Join(filepath.Dir(execPath), "scripts")
	scriptPath := filepath.Join(scriptDir, "install_deps.sh")

	// Check if the script exists
	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		// Try in the parent directory (for development)
		scriptDir = filepath.Join(filepath.Dir(execPath), "..", "scripts")
		scriptPath = filepath.Join(scriptDir, "install_deps.sh")

		if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
			fmt.Println("⚠️ Installation script not found. Using alternative method.")
			installDependenciesFallback()
			return
		}
	}

	// Execute the script
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		// On Windows, use bash.exe (WSL or Git Bash)
		cmd = exec.Command("bash.exe", scriptPath)
	} else {
		// On Unix-like, execute the script directly
		cmd = exec.Command("bash", scriptPath)
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Printf("⚠️ Error executing installation script: %v\n", err)
		fmt.Println("Attempting direct installation of Python dependencies...")
		installPythonDependencies()
	}
}

// This function is used if the install_deps.sh script is not found
func installDependenciesFallback() {
	fmt.Println("Installing necessary Python dependencies...")
	installPythonDependencies()
}

func installPythonDependencies() {
	// Determine the Python command to use
	pythonCmd := "python3"
	if _, err := exec.LookPath(pythonCmd); err != nil {
		pythonCmd = "python"
		if _, err := exec.LookPath(pythonCmd); err != nil {
			fmt.Println("⚠️ Python is not installed or is not in the PATH.")
			fmt.Println("Please install Python 3.x then run: pip install -U FlagEmbedding torch transformers")
			return
		}
	}

	// Install Python dependencies
	fmt.Println("Installing FlagEmbedding and associated dependencies...")
	installCmd := exec.Command(pythonCmd, "-m", "pip", "install", "--user", "-U", "FlagEmbedding", "torch", "transformers")
	installCmd.Stdout = os.Stdout
	installCmd.Stderr = os.Stderr

	if err := installCmd.Run(); err != nil {
		fmt.Printf("⚠️ Error installing Python dependencies: %v\n", err)
		fmt.Println("Please install manually: pip install -U FlagEmbedding torch transformers")
	} else {
		fmt.Println("✅ Python dependencies installed successfully!")
	}
}
