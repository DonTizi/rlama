package main

import (
	"github.com/dontizi/rlama/cmd"
	"github.com/dontizi/rlama/internal/client"
	"github.com/dontizi/rlama/internal/service"
)

func main() {
	// Initialisation correcte des services sans créer des références circulaires
	ollamaClient := client.NewDefaultOllamaClient()
	ragService := service.NewRagService(ollamaClient)
	agentService := service.NewAgentService(ollamaClient, ragService)
	// ragService.SetAgentService(agentService) // Removed circular dependency

	// Rendre ces services accessibles aux commandes
	cmd.SetServices(ragService, agentService)

	// Execute the root command
	cmd.Execute()
}
