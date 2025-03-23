package tests

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/dontizi/rlama/internal/domain"
	"github.com/dontizi/rlama/internal/service"
	"github.com/stretchr/testify/assert"
)

// TestChunkingEvaluation évalue différentes stratégies de chunking sur différents types de documents
func TestChunkingEvaluation(t *testing.T) {
	// Créer un dossier temporaire pour les documents de test
	tempDir, err := ioutil.TempDir("", "chunking-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Créer différents types de documents pour les tests
	createTestDocuments(t, tempDir)

	// Initialiser l'évaluateur
	chunkerService := service.NewChunkerService(service.DefaultChunkingConfig())
	evaluator := service.NewChunkingEvaluator(chunkerService)

	// Tester l'évaluation sur différents types de documents
	t.Run("EvaluateMarkdownDocument", func(t *testing.T) {
		mdDoc := loadTestDocument(t, filepath.Join(tempDir, "test_markdown.md"))

		// Évaluer chaque stratégie individuellement
		t.Log("Évaluation des stratégies sur un document Markdown")

		strategies := []string{"fixed", "semantic", "hybrid", "hierarchical"}
		for _, strategy := range strategies {
			config := service.ChunkingConfig{
				ChunkSize:        1000,
				ChunkOverlap:     100,
				ChunkingStrategy: strategy,
			}

			metrics := evaluator.EvaluateChunkingStrategy(mdDoc, config)
			t.Logf("Stratégie %s - Score: %.4f, Chunks: %d",
				strategy, metrics.SemanticCoherenceScore, metrics.TotalChunks)

			// Vérifier que les résultats sont cohérents
			assert.True(t, metrics.ContentCoverage > 0.8, "La couverture de contenu devrait être au moins de 80%")
			assert.NotZero(t, metrics.TotalChunks, "Le nombre de chunks ne devrait pas être zéro")
		}

		// Tester la comparaison automatique de stratégies
		bestResults := evaluator.CompareChunkingStrategies(mdDoc)
		assert.NotEmpty(t, bestResults, "La comparaison devrait retourner au moins un résultat")

		t.Logf("Meilleure stratégie pour Markdown: %s (taille: %d, chevauchement: %d, score: %.4f)",
			bestResults[0].Strategy, bestResults[0].ChunkSize, bestResults[0].ChunkOverlap,
			bestResults[0].SemanticCoherenceScore)
	})

	t.Run("EvaluateCodeDocument", func(t *testing.T) {
		codeDoc := loadTestDocument(t, filepath.Join(tempDir, "test_code.go"))

		// Évaluer chaque stratégie individuellement
		t.Log("Évaluation des stratégies sur un document de code")

		strategies := []string{"fixed", "semantic", "hybrid", "hierarchical"}
		for _, strategy := range strategies {
			config := service.ChunkingConfig{
				ChunkSize:        1000,
				ChunkOverlap:     100,
				ChunkingStrategy: strategy,
			}

			metrics := evaluator.EvaluateChunkingStrategy(codeDoc, config)
			t.Logf("Stratégie %s - Score: %.4f, Chunks: %d",
				strategy, metrics.SemanticCoherenceScore, metrics.TotalChunks)

			// Vérifier que les résultats sont cohérents
			assert.True(t, metrics.ContentCoverage > 0.8, "La couverture de contenu devrait être au moins de 80%")
		}

		// Tester la comparaison automatique de stratégies
		bestResults := evaluator.CompareChunkingStrategies(codeDoc)
		assert.NotEmpty(t, bestResults, "La comparaison devrait retourner au moins un résultat")

		t.Logf("Meilleure stratégie pour Code: %s (taille: %d, chevauchement: %d, score: %.4f)",
			bestResults[0].Strategy, bestResults[0].ChunkSize, bestResults[0].ChunkOverlap,
			bestResults[0].SemanticCoherenceScore)
	})

	t.Run("EvaluateLongTextDocument", func(t *testing.T) {
		textDoc := loadTestDocument(t, filepath.Join(tempDir, "test_longtext.txt"))

		// Évaluer chaque stratégie individuellement
		t.Log("Évaluation des stratégies sur un document texte long")

		strategies := []string{"fixed", "semantic", "hybrid", "hierarchical"}
		for _, strategy := range strategies {
			config := service.ChunkingConfig{
				ChunkSize:        1000,
				ChunkOverlap:     100,
				ChunkingStrategy: strategy,
			}

			metrics := evaluator.EvaluateChunkingStrategy(textDoc, config)
			t.Logf("Stratégie %s - Score: %.4f, Chunks: %d",
				strategy, metrics.SemanticCoherenceScore, metrics.TotalChunks)
		}

		// Tester la comparaison automatique de stratégies
		bestResults := evaluator.CompareChunkingStrategies(textDoc)
		assert.NotEmpty(t, bestResults, "La comparaison devrait retourner au moins un résultat")

		t.Logf("Meilleure stratégie pour Texte long: %s (taille: %d, chevauchement: %d, score: %.4f)",
			bestResults[0].Strategy, bestResults[0].ChunkSize, bestResults[0].ChunkOverlap,
			bestResults[0].SemanticCoherenceScore)
	})

	t.Run("OptimalConfigTest", func(t *testing.T) {
		// Tester la fonction GetOptimalChunkingConfig pour différents types de documents
		mdDoc := loadTestDocument(t, filepath.Join(tempDir, "test_markdown.md"))
		codeDoc := loadTestDocument(t, filepath.Join(tempDir, "test_code.go"))
		textDoc := loadTestDocument(t, filepath.Join(tempDir, "test_longtext.txt"))

		mdConfig := evaluator.GetOptimalChunkingConfig(mdDoc)
		codeConfig := evaluator.GetOptimalChunkingConfig(codeDoc)
		textConfig := evaluator.GetOptimalChunkingConfig(textDoc)

		t.Logf("Configuration optimale pour Markdown: %s (taille: %d, chevauchement: %d)",
			mdConfig.ChunkingStrategy, mdConfig.ChunkSize, mdConfig.ChunkOverlap)
		t.Logf("Configuration optimale pour Code: %s (taille: %d, chevauchement: %d)",
			codeConfig.ChunkingStrategy, codeConfig.ChunkSize, codeConfig.ChunkOverlap)
		t.Logf("Configuration optimale pour Texte long: %s (taille: %d, chevauchement: %d)",
			textConfig.ChunkingStrategy, textConfig.ChunkSize, textConfig.ChunkOverlap)

		// Vérifier que les configurations sont différentes en fonction du type de document
		// (pas nécessairement vrai, mais souvent le cas)
		if mdConfig.ChunkingStrategy == codeConfig.ChunkingStrategy &&
			mdConfig.ChunkSize == codeConfig.ChunkSize &&
			mdConfig.ChunkOverlap == codeConfig.ChunkOverlap {
			t.Log("Note: Les configurations optimales pour Markdown et Code sont identiques")
		}
	})
}

// loadTestDocument charge un document à partir d'un fichier
func loadTestDocument(t *testing.T, path string) *domain.Document {
	content, err := ioutil.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read test file %s: %v", path, err)
	}

	return &domain.Document{
		ID:      filepath.Base(path),
		Name:    filepath.Base(path),
		Path:    path,
		Content: string(content),
	}
}

// createTestDocuments crée différents types de documents de test
func createTestDocuments(t *testing.T, dir string) {
	// 1. Document Markdown avec des sections et sous-sections
	markdownContent := `# Test Markdown Document
	
## Introduction
This is a test document in Markdown format. It contains multiple sections and subsections.

### Purpose
The purpose of this document is to test how different chunking strategies handle Markdown documents.

## Section 1: Structured Content
This section contains structured content with lists, code blocks, and paragraphs.

### Lists
Here is a list:
- Item 1
- Item 2
- Item 3

### Code Block
Here is a code block:
` + "```" + `
func testFunction() {
    fmt.Println("Hello, world!")
}
` + "```" + `

## Section 2: Long Paragraphs
This section contains longer paragraphs to test how chunking handles continuous text.

Lorem ipsum dolor sit amet, consectetur adipiscing elit. Nullam auctor, nisl eget ultricies tincidunt, 
nisl nisl aliquam nisl, eget ultricies nisl nisl eget nisl. Nullam auctor, nisl eget ultricies tincidunt,
nisl nisl aliquam nisl, eget ultricies nisl nisl eget nisl. Nullam auctor, nisl eget ultricies tincidunt,
nisl nisl aliquam nisl, eget ultricies nisl nisl eget nisl.

Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam,
quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat.
Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur.

## Conclusion
This is the conclusion of the test document. It should be placed in a different chunk from the introduction.
	`

	// 2. Document de code avec des fonctions et classes
	codeContent := `package main

import (
	"fmt"
	"strings"
)

// TestStruct is a test struct
type TestStruct struct {
	Name string
	Age  int
}

// NewTestStruct creates a new TestStruct
func NewTestStruct(name string, age int) *TestStruct {
	return &TestStruct{
		Name: name,
		Age:  age,
	}
}

// GetFullName returns the full name of the person
func (t *TestStruct) GetFullName() string {
	return fmt.Sprintf("%s (Age: %d)", t.Name, t.Age)
}

// ProcessData processes some data
func ProcessData(data []string) []string {
	var result []string
	
	for _, item := range data {
		if len(item) > 0 {
			result = append(result, strings.ToUpper(item))
		}
	}
	
	return result
}

// LongFunction is a long function to test chunking
func LongFunction() {
	fmt.Println("This is a long function")
	fmt.Println("It contains multiple lines")
	fmt.Println("To test how the chunking works")
	fmt.Println("With longer code blocks")
	
	// A nested block
	for i := 0; i < 10; i++ {
		if i%2 == 0 {
			fmt.Printf("Even number: %d\n", i)
		} else {
			fmt.Printf("Odd number: %d\n", i)
		}
	}
	
	// Another nested block
	data := []string{"apple", "banana", "cherry"}
	processed := ProcessData(data)
	for _, item := range processed {
		fmt.Println(item)
	}
}

func main() {
	LongFunction()
	
	person := NewTestStruct("John Doe", 30)
	fmt.Println(person.GetFullName())
}`

	// 3. Document texte long avec des paragraphes
	longTextContent := `Long Text Document for Testing Chunking Strategies

This document contains long paragraphs of text to test how different chunking strategies handle continuous prose without much structure.

First Paragraph: Lorem ipsum dolor sit amet, consectetur adipiscing elit. Nullam auctor, nisl eget ultricies tincidunt, nisl nisl aliquam nisl, eget ultricies nisl nisl eget nisl. Nullam auctor, nisl eget ultricies tincidunt, nisl nisl aliquam nisl, eget ultricies nisl nisl eget nisl. Nullam auctor, nisl eget ultricies tincidunt, nisl nisl aliquam nisl, eget ultricies nisl nisl eget nisl. Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat. Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur.

Second Paragraph: Excepteur sint occaecat cupidatat non proident, sunt in culpa qui officia deserunt mollit anim id est laborum. Nullam auctor, nisl eget ultricies tincidunt, nisl nisl aliquam nisl, eget ultricies nisl nisl eget nisl. Nullam auctor, nisl eget ultricies tincidunt, nisl nisl aliquam nisl, eget ultricies nisl nisl eget nisl. Nullam auctor, nisl eget ultricies tincidunt, nisl nisl aliquam nisl, eget ultricies nisl nisl eget nisl. Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat. Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur.

Third Paragraph: Excepteur sint occaecat cupidatat non proident, sunt in culpa qui officia deserunt mollit anim id est laborum. Nullam auctor, nisl eget ultricies tincidunt, nisl nisl aliquam nisl, eget ultricies nisl nisl eget nisl. Nullam auctor, nisl eget ultricies tincidunt, nisl nisl aliquam nisl, eget ultricies nisl nisl eget nisl. Nullam auctor, nisl eget ultricies tincidunt, nisl nisl aliquam nisl, eget ultricies nisl nisl eget nisl. Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat. Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur.

A sentence that stands alone.

Fourth Paragraph: Excepteur sint occaecat cupidatat non proident, sunt in culpa qui officia deserunt mollit anim id est laborum. Nullam auctor, nisl eget ultricies tincidunt, nisl nisl aliquam nisl, eget ultricies nisl nisl eget nisl. Nullam auctor, nisl eget ultricies tincidunt, nisl nisl aliquam nisl, eget ultricies nisl nisl eget nisl. Nullam auctor, nisl eget ultricies tincidunt, nisl nisl aliquam nisl, eget ultricies nisl nisl eget nisl. Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat. Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur.

Fifth Paragraph with some questions. How does the chunker handle questions? Does it maintain them in the same chunk? What about if the questions are at the end of a paragraph? These are important considerations for maintaining the semantic coherence of the text.

Sixth Paragraph which is quite short.

Seventh Paragraph: Excepteur sint occaecat cupidatat non proident, sunt in culpa qui officia deserunt mollit anim id est laborum. Nullam auctor, nisl eget ultricies tincidunt, nisl nisl aliquam nisl, eget ultricies nisl nisl eget nisl. Nullam auctor, nisl eget ultricies tincidunt, nisl nisl aliquam nisl, eget ultricies nisl nisl eget nisl. Nullam auctor, nisl eget ultricies tincidunt, nisl nisl aliquam nisl, eget ultricies nisl nisl eget nisl. Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat. Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur.

Conclusion: This document was created to test chunking strategies on long continuous text without much structure. The ideal chunking strategy should maintain paragraph boundaries and avoid splitting sentences.`

	// Écrire les fichiers de test
	if err := ioutil.WriteFile(filepath.Join(dir, "test_markdown.md"), []byte(markdownContent), 0644); err != nil {
		t.Fatalf("Failed to write markdown test file: %v", err)
	}

	if err := ioutil.WriteFile(filepath.Join(dir, "test_code.go"), []byte(codeContent), 0644); err != nil {
		t.Fatalf("Failed to write code test file: %v", err)
	}

	if err := ioutil.WriteFile(filepath.Join(dir, "test_longtext.txt"), []byte(longTextContent), 0644); err != nil {
		t.Fatalf("Failed to write longtext test file: %v", err)
	}

	fmt.Printf("Created test documents in %s\n", dir)
}
