package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/dontizi/rlama/internal/domain"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var (
	autoRun   bool
	inputFile string
	watchDir  string
)

var crewWizardCmd = &cobra.Command{
	Use:   "crew-wizard",
	Short: "Crew creation wizard",
	Long:  `Interactive wizard to create and configure crews and their workflows.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		reader := bufio.NewReader(os.Stdin)

		// Terminal colors
		titleColor := color.New(color.FgCyan, color.Bold)
		promptColor := color.New(color.FgGreen)
		successColor := color.New(color.FgGreen, color.Bold)
		infoColor := color.New(color.FgYellow)

		titleColor.Println("\n=== RLAMA Crew Creation Wizard ===")
		fmt.Println()
		infoColor.Println("This wizard will guide you through creating a complete crew.")
		infoColor.Println("A crew is a group of agents organized in a workflow to accomplish complex tasks.")
		fmt.Println()

		// Create base crew
		crew, err := createCrewWizard(reader, promptColor)
		if err != nil {
			return err
		}

		// Add agents
		err = addAgentsWizard(reader, crew, promptColor, successColor)
		if err != nil {
			return err
		}

		// Configure workflow
		err = configureWorkflowWizard(reader, crew, promptColor, infoColor)
		if err != nil {
			return err
		}

		// Save crew configuration
		err = saveCrew(crew)
		if err != nil {
			return fmt.Errorf("error saving crew configuration: %w", err)
		}

		// Configure automation if requested
		if autoRun {
			err = configureAutomation(crew, watchDir, inputFile)
			if err != nil {
				return fmt.Errorf("error configuring automation: %w", err)
			}
		}

		successColor.Printf("\nCrew '%s' has been successfully created and configured!\n", crew.Name)
		return nil
	},
}

func createCrewWizard(reader *bufio.Reader, promptColor *color.Color) (*domain.Crew, error) {
	promptColor.Print("Enter crew name: ")
	name, _ := reader.ReadString('\n')
	name = strings.TrimSpace(name)

	promptColor.Print("Enter crew description: ")
	description, _ := reader.ReadString('\n')
	description = strings.TrimSpace(description)

	// Create a new crew with sequential workflow by default
	crew := domain.NewCrew(name, description, domain.WorkflowSequential)

	return crew, nil
}

func addAgentsWizard(reader *bufio.Reader, crew *domain.Crew, promptColor, successColor *color.Color) error {
	agentService := GetAgentService()

	for {
		promptColor.Print("\nAdd a new agent? (y/n): ")
		answer, _ := reader.ReadString('\n')
		if strings.ToLower(strings.TrimSpace(answer)) != "y" {
			break
		}

		promptColor.Print("Enter agent name: ")
		name, _ := reader.ReadString('\n')
		name = strings.TrimSpace(name)

		promptColor.Print("Enter agent description: ")
		description, _ := reader.ReadString('\n')
		description = strings.TrimSpace(description)

		promptColor.Print("Enter agent model (default: llama2): ")
		modelName, _ := reader.ReadString('\n')
		modelName = strings.TrimSpace(modelName)
		if modelName == "" {
			modelName = "llama2"
		}

		// Create a new agent
		agent, err := agentService.CreateAgent(name, description, "assistant", modelName, "")
		if err != nil {
			return fmt.Errorf("failed to create agent: %w", err)
		}

		// Add the agent to the crew
		crew.AddAgent(agent.ID)
		successColor.Printf("Agent '%s' added successfully!\n", name)
	}

	return nil
}

func configureWorkflowWizard(reader *bufio.Reader, crew *domain.Crew, promptColor, infoColor *color.Color) error {
	if len(crew.Agents) < 2 {
		infoColor.Println("\nSkipping workflow configuration as there are less than 2 agents.")
		return nil
	}

	promptColor.Println("\nWorkflow Configuration")
	promptColor.Println("Configure how agents work together. For each agent, specify which other agents receive its output.")

	// Load all agents first
	agentService := GetAgentService()
	agents := make([]*domain.Agent, 0, len(crew.Agents))
	for _, agentID := range crew.Agents {
		agent, err := agentService.LoadAgent(agentID)
		if err != nil {
			return fmt.Errorf("failed to load agent %s: %w", agentID, err)
		}
		agents = append(agents, agent)
	}

	for i, agent := range agents {
		promptColor.Printf("\nConfigure outputs for agent '%s'\n", agent.Name)

		// Display available agents
		promptColor.Println("Available agents:")
		for j, other := range agents {
			if other.ID != agent.ID {
				promptColor.Printf("%d. %s\n", j+1, other.Name)
			}
		}

		promptColor.Print("Enter agent numbers to receive output (comma-separated, or 0 for none): ")
		indices, _ := reader.ReadString('\n')
		indices = strings.TrimSpace(indices)

		if indices != "0" {
			// Parse the indices
			for _, idx := range strings.Split(indices, ",") {
				idx = strings.TrimSpace(idx)
				if idx == "" {
					continue
				}

				j, err := strconv.Atoi(idx)
				if err != nil || j < 1 || j > len(agents) {
					continue
				}

				// Add the workflow step
				step := domain.WorkflowStep{
					AgentID:      agent.ID,
					Instruction:  fmt.Sprintf("Process output from agent '%s'", agent.Name),
					DependsOn:    []int{i},
					OutputToNext: true,
				}
				crew.AddWorkflowStep(step)
			}
		}
	}

	return nil
}

func saveCrew(crew *domain.Crew) error {
	// Create crews directory if it doesn't exist
	crewsDir := "crews"
	if err := os.MkdirAll(crewsDir, 0755); err != nil {
		return err
	}

	// Convert crew to JSON
	crewJSON, err := json.MarshalIndent(crew, "", "  ")
	if err != nil {
		return err
	}

	// Save to file
	filename := filepath.Join(crewsDir, crew.Name+".json")
	return ioutil.WriteFile(filename, crewJSON, 0644)
}

func configureAutomation(crew *domain.Crew, watchDir, inputFile string) error {
	if watchDir != "" {
		// Configure directory watching
		// Implementation depends on your monitoring system
	}

	if inputFile != "" {
		return processInputFile(crew, inputFile)
	}

	return nil
}

func processInputFile(crew *domain.Crew, inputFile string) error {
	// Read and process input file
	data, err := ioutil.ReadFile(inputFile)
	if err != nil {
		return err
	}

	// Parse input data according to your format
	// This is a placeholder for your actual implementation
	_ = data

	return nil
}

func init() {
	rootCmd.AddCommand(crewWizardCmd)

	crewWizardCmd.Flags().BoolVar(&autoRun, "auto-run", false, "Automatically start the crew after creation")
	crewWizardCmd.Flags().StringVar(&inputFile, "input-file", "", "Input file for batch processing")
	crewWizardCmd.Flags().StringVar(&watchDir, "watch-dir", "", "Directory to watch for new files")
}
