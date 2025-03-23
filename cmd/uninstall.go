package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

var forceUninstall bool

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Uninstall RLAMA and all its files",
	Long:  `Completely uninstall RLAMA by removing the executable and all associated data files.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// 1. Check if the user confirmed the deletion
		if !forceUninstall {
			fmt.Print("This action will remove RLAMA and all your data. Are you sure? (y/n): ")
			var response string
			fmt.Scanln(&response)

			response = strings.ToLower(strings.TrimSpace(response))
			if response != "y" && response != "yes" {
				fmt.Println("Uninstallation cancelled.")
				return nil
			}
		}

		// 2. Delete the data directory
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("unable to determine user directory: %w", err)
		}

		dataDir := filepath.Join(homeDir, ".rlama")
		fmt.Printf("Removing data directory: %s\n", dataDir)

		if _, err := os.Stat(dataDir); err == nil {
			err = os.RemoveAll(dataDir)
			if err != nil {
				return fmt.Errorf("unable to remove data directory: %w", err)
			}
			fmt.Println("✓ Data directory removed")
		} else {
			fmt.Println("Data directory doesn't exist or has already been removed")
		}

		// 3. Remove the executable
		var executablePath string
		if runtime.GOOS == "windows" {
			// Try different locations where it might be installed
			localAppData := os.Getenv("LOCALAPPDATA")
			possiblePaths := []string{
				filepath.Join(localAppData, "RLAMA", "rlama.exe"),
				filepath.Join(os.Getenv("ProgramFiles"), "RLAMA", "rlama.exe"),
				filepath.Join(os.Getenv("ProgramFiles(x86)"), "RLAMA", "rlama.exe"),
				filepath.Join(homeDir, "AppData", "Local", "RLAMA", "rlama.exe"),
			}

			for _, path := range possiblePaths {
				if _, err := os.Stat(path); err == nil {
					executablePath = path
					break
				}
			}
		} else {
			executablePath = "/usr/local/bin/rlama"
		}

		fmt.Printf("Removing executable: %s\n", executablePath)

		if executablePath == "" && runtime.GOOS == "windows" {
			fmt.Println("Could not find RLAMA executable. If RLAMA is installed elsewhere, you may need to:")
			fmt.Println("1. Run Command Prompt as Administrator")
			fmt.Println("2. Navigate to the installation directory")
			fmt.Println("3. Manually delete the rlama.exe file")
			fmt.Println("\nRLAMA data directory has been removed successfully.")
			return nil
		}

		if _, err := os.Stat(executablePath); err == nil {
			// On macOS and Linux, we probably need sudo
			var err error
			if runtime.GOOS == "windows" {
				// On Windows, try to remove directly
				err = os.Remove(executablePath)
				if err != nil {
					// If direct removal fails, try with elevated privileges using full PowerShell path
					fmt.Println("Need elevated privileges to remove the executable")
					powershellPath := filepath.Join(os.Getenv("SystemRoot"), "System32", "WindowsPowerShell", "v1.0", "powershell.exe")
					err = execCommand(powershellPath, "-Command", fmt.Sprintf("Start-Process -Verb RunAs -FilePath 'cmd.exe' -ArgumentList '/c del \"%s\"'", executablePath))
				}
			} else if isRoot() {
				// If we're already root on Unix systems
				err = os.Remove(executablePath)
			} else {
				fmt.Println("You may need to enter your password to remove the executable")
				err = execCommand("sudo", "rm", executablePath)
			}

			if err != nil {
				if runtime.GOOS == "windows" {
					return fmt.Errorf("unable to remove executable: %w\nTry running the command prompt as administrator and run 'rlama uninstall' again", err)
				}
				return fmt.Errorf("unable to remove executable: %w", err)
			}
			fmt.Println("✓ Executable removed")
		} else {
			fmt.Println("Executable doesn't exist or has already been removed")
		}

		fmt.Println("\nRLAMA has been successfully uninstalled.")
		return nil
	},
}

// execCommand executes a system command
func execCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func init() {
	rootCmd.AddCommand(uninstallCmd)
	uninstallCmd.Flags().BoolVarP(&forceUninstall, "force", "f", false, "Uninstall without asking for confirmation")
}

// isRoot returns true if the current process is running with root/admin privileges
// This is a safe wrapper around os.Geteuid() which doesn't exist on Windows
func isRoot() bool {
	if runtime.GOOS == "windows" {
		// On Windows, check if we have admin privileges using a different method
		// However, this is not easily determined, so we'll return false
		// and let the code try direct removal first
		return false
	}

	// On Unix systems, check if euid is 0 (root)
	return os.Geteuid() == 0
}
