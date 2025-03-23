package cmd

import (
	"fmt"

	"github.com/dontizi/rlama/internal/client"
	"github.com/dontizi/rlama/internal/repository"
	"github.com/spf13/cobra"
)

var updateModelCmd = &cobra.Command{
	Use:   "update-model [rag-name] [new-model]",
	Short: "Update the Ollama model used by a RAG system",
	Long: `Change the Ollama model used by an existing RAG system.
Example: rlama update-model my-docs llama3.2
	
Note: This does not regenerate embeddings. For optimal results, you may want to
recreate the RAG with the new model instead.`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		ragName := args[0]
		newModel := args[1]

		// Get Ollama client from root command
		ollamaClient := GetOllamaClient()

		// Check if this is a Hugging Face model
		if client.IsHuggingFaceModel(newModel) {
			// Extract model name and quantization
			hfModelName := client.GetHuggingFaceModelName(newModel)
			quantization := client.GetQuantizationFromModelRef(newModel)

			fmt.Printf("Detected Hugging Face model. Pulling %s", hfModelName)
			if quantization != "" {
				fmt.Printf(" with quantization %s", quantization)
			}
			fmt.Println("...")

			// Pull the model from Hugging Face
			if err := ollamaClient.PullHuggingFaceModel(hfModelName, quantization); err != nil {
				return fmt.Errorf("error pulling Hugging Face model: %w", err)
			}

			fmt.Println("Successfully pulled Hugging Face model.")
		} else {
			// For regular Ollama models
			if err := ollamaClient.CheckOllamaAndModel(newModel); err != nil {
				return err
			}
		}

		// Load the RAG
		repo := repository.NewRagRepository()
		rag, err := repo.Load(ragName)
		if err != nil {
			return err
		}

		// Update the model
		oldModel := rag.ModelName
		rag.ModelName = newModel

		// Save the RAG
		if err := repo.Save(rag); err != nil {
			return fmt.Errorf("error saving the RAG: %w", err)
		}

		fmt.Printf("Successfully updated RAG '%s' model from '%s' to '%s'.\n",
			ragName, oldModel, newModel)
		fmt.Println("Note: Embeddings have not been regenerated. For optimal results, consider recreating the RAG.")

		// Check if the profile exists if specified
		if updateModelProfileName != "" {
			profileRepo := repository.NewProfileRepository()
			if !profileRepo.Exists(updateModelProfileName) {
				return fmt.Errorf("profile '%s' does not exist", updateModelProfileName)
			}

			// Update the profile in the RAG
			rag.APIProfileName = updateModelProfileName
			fmt.Printf("Using profile '%s' for model '%s'\n", updateModelProfileName, newModel)
		}

		return nil
	},
}

var updateModelProfileName string

func init() {
	rootCmd.AddCommand(updateModelCmd)
	updateModelCmd.Flags().StringVar(&updateModelProfileName, "profile", "", "API profile to use for this model")
}
