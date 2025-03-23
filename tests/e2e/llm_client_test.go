package tests

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dontizi/rlama/internal/client"
	"github.com/stretchr/testify/assert"
)

func TestIsOpenAIModel(t *testing.T) {
	testCases := []struct {
		model    string
		expected bool
	}{
		{"gpt-4", true},
		{"gpt-3.5-turbo", true},
		{"gpt-4-turbo-preview", true},
		{"llama3", false},
		{"mistral", false},
		{"gpt-fake", false},
	}

	for _, tc := range testCases {
		t.Run(tc.model, func(t *testing.T) {
			result := client.IsOpenAIModel(tc.model)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestOllamaClient(t *testing.T) {
	// Créer un serveur HTTP mockant Ollama
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/embeddings":
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"embedding": [0.1, 0.2, 0.3]}`))
		case "/api/generate":
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"model": "llama3", "response": "Test response", "context": [], "done": true}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// Extract host and port from the server URL
	host := server.URL

	// Créer un client Ollama pointant vers notre serveur mock
	ollamaClient := &client.OllamaClient{
		BaseURL: host,
		Client:  http.DefaultClient,
	}

	t.Run("GenerateEmbedding", func(t *testing.T) {
		embedding, err := ollamaClient.GenerateEmbedding("llama3", "Test text")
		assert.NoError(t, err)
		assert.NotEmpty(t, embedding)
	})

	t.Run("GenerateCompletion", func(t *testing.T) {
		response, err := ollamaClient.GenerateCompletion("llama3", "Test prompt")
		assert.NoError(t, err)
		assert.Equal(t, "Test response", response)
	})
}
