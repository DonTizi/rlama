package tests

import (
	"reflect"

	"github.com/dontizi/rlama/internal/client"
	"github.com/dontizi/rlama/internal/domain"
	"github.com/dontizi/rlama/internal/repository"
	"github.com/dontizi/rlama/internal/service"
	"github.com/stretchr/testify/mock"
)

// MockOllamaClient simule un client Ollama
type MockOllamaClient struct {
	mock.Mock
}

func (m *MockOllamaClient) GenerateCompletion(model, prompt string) (string, error) {
	args := m.Called(model, prompt)
	return args.String(0), args.Error(1)
}

func (m *MockOllamaClient) GenerateEmbedding(model, text string) ([]float32, error) {
	args := m.Called(model, text)
	return args.Get(0).([]float32), args.Error(1)
}

func (m *MockOllamaClient) CheckOllamaAndModel(modelName string) error {
	args := m.Called(modelName)
	return args.Error(0)
}

func (m *MockOllamaClient) CheckLLMAndModel(modelName string) error {
	return m.CheckOllamaAndModel(modelName)
}

// MockEmbeddingService simule le service d'embedding
type MockEmbeddingService struct {
	mock.Mock
}

func (m *MockEmbeddingService) GenerateChunkEmbeddings(chunks []*domain.DocumentChunk, modelName string) error {
	args := m.Called(chunks, modelName)
	return args.Error(0)
}

func (m *MockEmbeddingService) GenerateQueryEmbedding(query string, modelName string) ([]float32, error) {
	args := m.Called(query, modelName)
	return args.Get(0).([]float32), args.Error(1)
}

func (m *MockEmbeddingService) GetOllamaClient() *client.OllamaClient {
	return nil
}

// MockBGERerankerClient simule le client BGE Reranker
type MockBGERerankerClient struct {
	mock.Mock
}

func (m *MockBGERerankerClient) GetModelName() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockBGERerankerClient) ComputeScores(pairs [][]string, normalize bool) ([]float64, error) {
	args := m.Called(pairs, normalize)
	return args.Get(0).([]float64), args.Error(1)
}

func (m *MockBGERerankerClient) CheckDependencies() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockBGERerankerClient) CheckModelExists() error {
	args := m.Called()
	return args.Error(0)
}

// NewMockRerankerService crée un RerankerService utilisant un mock BGERerankerClient
func NewMockRerankerService(ollamaClient *client.OllamaClient, mockBGEClient *MockBGERerankerClient) *service.RerankerService {
	// Créer une structure RerankerService avec des champs privés
	rerankerService := &service.RerankerService{}

	// Utiliser reflect pour accéder aux champs privés et les modifier
	serviceValue := reflect.ValueOf(rerankerService).Elem()

	// Remplacer le client Ollama
	ollamaField := serviceValue.FieldByName("ollamaClient")
	if ollamaField.IsValid() && ollamaField.CanSet() {
		ollamaField.Set(reflect.ValueOf(ollamaClient))
	}

	// Remplacer le client BGE Reranker par notre mock
	bgeField := serviceValue.FieldByName("bgeRerankerClient")
	if bgeField.IsValid() && bgeField.CanSet() {
		// Cast le MockBGERerankerClient à client.BGERerankerClient via interface{}
		bgeField.Set(reflect.ValueOf(mockBGEClient))
	}

	return rerankerService
}

// NewRagServiceWithMockReranker crée un RagService avec un RerankerService utilisant un mock BGERerankerClient
func NewRagServiceWithMockReranker(ollamaClient *client.OllamaClient, embeddingService *service.EmbeddingService, mockBGEClient *MockBGERerankerClient) service.RagService {
	if ollamaClient == nil {
		ollamaClient = client.NewDefaultOllamaClient()
	}

	// Créer un RagService standard
	ragService := &service.RagServiceImpl{}

	// Utiliser reflect pour accéder aux champs privés
	serviceValue := reflect.ValueOf(ragService).Elem()

	// Configurer les services
	if embeddingService != nil {
		embedField := serviceValue.FieldByName("embeddingService")
		if embedField.IsValid() && embedField.CanSet() {
			embedField.Set(reflect.ValueOf(embeddingService))
		}
	} else {
		embedField := serviceValue.FieldByName("embeddingService")
		if embedField.IsValid() && embedField.CanSet() {
			embedField.Set(reflect.ValueOf(service.NewEmbeddingService(ollamaClient)))
		}
	}

	// Configurer le client Ollama
	ollamaField := serviceValue.FieldByName("ollamaClient")
	if ollamaField.IsValid() && ollamaField.CanSet() {
		ollamaField.Set(reflect.ValueOf(ollamaClient))
	}

	// Configurer le DocumentLoader
	loaderField := serviceValue.FieldByName("documentLoader")
	if loaderField.IsValid() && loaderField.CanSet() {
		loaderField.Set(reflect.ValueOf(service.NewDocumentLoader()))
	}

	// Configurer le RagRepository
	repoField := serviceValue.FieldByName("ragRepository")
	if repoField.IsValid() && repoField.CanSet() {
		repoField.Set(reflect.ValueOf(repository.NewRagRepository()))
	}

	// Configurer le RerankerService avec notre mock
	rerankerService := NewMockRerankerService(ollamaClient, mockBGEClient)
	rerankerField := serviceValue.FieldByName("rerankerService")
	if rerankerField.IsValid() && rerankerField.CanSet() {
		rerankerField.Set(reflect.ValueOf(rerankerService))
	}

	return ragService
}

// TestRagService est une implémentation de test du service RAG
type TestRagService struct {
	mockOllama    *MockOllamaClient
	mockEmbedding *MockEmbeddingService
	ragRepository *repository.RagRepository
}

func (s *TestRagService) CreateRagWithOptions(modelName, ragName, folderPath string, options service.DocumentLoaderOptions) error {
	if err := s.mockOllama.CheckOllamaAndModel(modelName); err != nil {
		return err
	}

	rag := domain.NewRagSystem(ragName, modelName)

	// Simuler l'ajout d'un document et la génération d'embeddings
	chunk := &domain.DocumentChunk{
		ID:      "test-chunk",
		Content: "Test content",
	}

	// Appeler GenerateEmbedding une seule fois
	embedding, err := s.mockOllama.GenerateEmbedding(modelName, chunk.Content)
	if err != nil {
		return err
	}
	chunk.Embedding = embedding

	// Appeler GenerateChunkEmbeddings une seule fois
	err = s.mockEmbedding.GenerateChunkEmbeddings([]*domain.DocumentChunk{chunk}, modelName)
	if err != nil {
		return err
	}

	// Appeler GenerateQueryEmbedding une seule fois
	_, err = s.mockEmbedding.GenerateQueryEmbedding("test query", modelName)
	if err != nil {
		return err
	}

	rag.AddChunk(chunk)
	return s.ragRepository.Save(rag)
}

func (s *TestRagService) LoadRag(ragName string) (*domain.RagSystem, error) {
	return s.ragRepository.Load(ragName)
}

func (s *TestRagService) DeleteRag(ragName string) error {
	return s.ragRepository.Delete(ragName)
}

func (s *TestRagService) ListRags() ([]string, error) {
	return []string{"test-rag"}, nil
}

// Ajouter les méthodes manquantes pour l'interface RagService

func (s *TestRagService) GetRagChunks(ragName string, filter service.ChunkFilter) ([]*domain.DocumentChunk, error) {
	// Mock pour retourner quelques chunks de test
	chunks := []*domain.DocumentChunk{
		{
			ID:          "test-chunk-1",
			DocumentID:  "test-doc-1",
			Content:     "This is test content 1",
			ChunkNumber: 0,
			TotalChunks: 2,
		},
		{
			ID:          "test-chunk-2",
			DocumentID:  "test-doc-1",
			Content:     "This is test content 2",
			ChunkNumber: 1,
			TotalChunks: 2,
		},
	}
	return chunks, nil
}

func (s *TestRagService) Query(rag *domain.RagSystem, query string, contextSize int) (string, error) {
	// Vérifier le modèle avec CheckLLMAndModel (important pour satisfaire l'expectation du mock)
	if err := s.mockOllama.CheckLLMAndModel(rag.ModelName); err != nil {
		return "", err
	}

	// Simuler une réponse en utilisant notre mock ollama client
	return s.mockOllama.GenerateCompletion("any-model", "any-prompt")
}

func (s *TestRagService) AddDocsWithOptions(ragName string, folderPath string, options service.DocumentLoaderOptions) error {
	// Mock implémentation simple
	return nil
}

func (s *TestRagService) UpdateModel(ragName string, newModel string) error {
	// Mock implémentation simple
	return nil
}

func (s *TestRagService) UpdateRag(rag *domain.RagSystem) error {
	// Mock implémentation simple pour sauvegarder le RAG modifié
	return s.ragRepository.Save(rag)
}

func (s *TestRagService) UpdateRerankerModel(ragName string, model string) error {
	// Mock implémentation simple
	return nil
}

func (s *TestRagService) ListAllRags() ([]string, error) {
	return []string{"test-rag"}, nil
}

func (s *TestRagService) GetOllamaClient() *client.OllamaClient {
	// Ce mock ne possède pas un vrai client Ollama
	return nil
}

// Méthodes liées au directory watching
func (s *TestRagService) SetupDirectoryWatching(ragName string, dirPath string, watchInterval int, options service.DocumentLoaderOptions) error {
	return nil
}

func (s *TestRagService) DisableDirectoryWatching(ragName string) error {
	return nil
}

func (s *TestRagService) CheckWatchedDirectory(ragName string) (int, error) {
	return 0, nil
}

// Méthodes liées au web watching
func (s *TestRagService) SetupWebWatching(ragName string, websiteURL string, watchInterval int, options domain.WebWatchOptions) error {
	return nil
}

func (s *TestRagService) DisableWebWatching(ragName string) error {
	return nil
}

func (s *TestRagService) CheckWatchedWebsite(ragName string) (int, error) {
	return 0, nil
}
