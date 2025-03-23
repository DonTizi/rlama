# RLAMA E2E Tests

This directory contains end-to-end tests for the RLAMA application. These tests validate the functionality of various components and ensure that they work correctly together.

## Overview

The tests in this directory are organized by component, with each file focusing on a specific aspect of the application. They include both unit tests for individual components and integration tests that verify the interaction between different parts of the system.

## Running the Tests

To run all tests:

```bash
go test ./tests/e2e/...
```

To run a specific test:

```bash
go test ./tests/e2e -run TestName
```

For skipping slow E2E tests:

```bash
go test ./tests/e2e/... -short
```

## Test Files

### `chunker_test.go`

Tests the document chunking functionality, which splits documents into smaller pieces for processing.

- **TestChunker**: Tests the chunker service with both default and custom configurations
  - Verifies chunks are created correctly with the expected properties
  - Checks that custom chunk size and overlap settings are respected

### `cli_test.go`

Tests the CLI commands and their interactions. This is a comprehensive end-to-end test that verifies the entire application works as expected.

- **TestBasicCliCommands**: Contains multiple subtests:
  - **BasicCommands**: Tests basic CLI commands like version and help
  - **ProfileAddAndList**: Tests profile management commands
  - **RAGCommands**: Tests RAG creation and management commands
  - **DocumentCommands**: Tests document management commands
  - **WatchCommands**: Tests directory watching functionality
  - **UpdateCommands**: Tests model updating commands
  - **HuggingFaceCommands**: Tests Hugging Face integration
  - **APIServer**: Tests the API server functionality

Each test compiles the application binary, creates a test environment, executes commands, and verifies the output.

### `document_loader_test.go`

Tests the document loading functionality, which reads documents from the filesystem.

- **TestDocumentLoader**: Contains subtests:
  - **LoadDocumentsBasic**: Tests basic document loading functionality
  - **LoadDocumentsWithOptions**: Tests document loading with custom options (excluding directories/extensions)

### `llm_client_test.go`

Tests the LLM client implementations, which interact with language models.

- **TestIsOpenAIModel**: Tests the function that determines if a model is from OpenAI
- **TestOllamaClient**: Tests the Ollama client implementation
  - **GenerateEmbedding**: Tests embedding generation
  - **GenerateCompletion**: Tests text completion generation

### `profile_repository_test.go`

Tests the profile repository, which manages API profiles.

- **TestProfileRepository**: Contains subtests:
  - **SaveAndLoad**: Tests saving and loading profiles
  - **Delete**: Tests profile deletion
  - **ListAll**: Tests listing all profiles

### `profile_test.go`

Tests profile operations in an integrated manner.

- **TestProfiles**: Contains subtests:
  - **CreateProfile**: Tests profile creation
  - **GetProfile**: Tests profile retrieval
  - **ListProfiles**: Tests profile listing
  - **DeleteProfile**: Tests profile deletion

### `rag_service_test.go`

Tests the RAG (Retrieval Augmented Generation) service, which manages document retrieval and generation.

- **TestRagServiceOperations**: Contains a comprehensive test:
  - **CreateAndQueryRag**: Tests creating a RAG system, updating it, and querying it

This test uses actual Ollama embeddings and completions for realistic testing.

### `rag_test.go`

Tests RAG operations using mocked services.

- **TestRagOperations**: Tests RAG creation using mocked clients
  - Uses mock Ollama client and embedding service to verify the RAG creation flow

### `test_helpers.go`

Contains helper functions and mock implementations for testing:

- **MockOllamaClient**: Mocks the Ollama client interface
- **MockEmbeddingService**: Mocks the embedding service
- **TestRagService**: A test implementation of the RAG service that uses mocks

### `wizard_test.go`

Tests the wizard functionality, which guides users through the RAG creation process.

- **TestWizard**: Tests wizard operations using mocked services
  - **CreateRAG**: Tests RAG creation
  - **LoadRAG**: Tests loading a RAG system
  - **DeleteRAG**: Tests deleting a RAG system

## Test Structure

Most tests follow this general structure:

1. **Setup**: Create necessary temporary files, directories, and mock objects
2. **Test Execution**: Call the methods being tested
3. **Assertion**: Verify the results match expectations
4. **Cleanup**: Remove temporary files and resources

## Mock Objects

The tests use mock objects to simulate external dependencies:

- **MockOllamaClient**: Simulates the Ollama API for language model operations
- **MockEmbeddingService**: Simulates the embedding service for vector embeddings

These mocks allow testing without requiring the actual external services to be available.

## Special Considerations

- Some tests use actual filesystem operations and create temporary directories
- The CLI tests compile the actual binary and execute it
- Some tests interact with real services and may require external dependencies
- Tests with external dependencies can be skipped using the `-short` flag
