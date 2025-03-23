package tests

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dontizi/rlama/internal/service"
	"github.com/stretchr/testify/assert"
)

func TestDocumentLoader(t *testing.T) {
	// Create a temporary test directory
	tempDir, err := os.MkdirTemp("", "document-loader-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test files
	files := map[string]string{
		"test.txt":          "This is a test document.",
		"test.md":           "# Markdown Title\nThis is markdown content.",
		"excluded.csv":      "col1,col2\nvalue1,value2",
		"subdir/nested.txt": "This is a nested file.",
	}

	for path, content := range files {
		fullPath := filepath.Join(tempDir, path)
		os.MkdirAll(filepath.Dir(fullPath), 0755)
		err := os.WriteFile(fullPath, []byte(content), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file %s: %v", path, err)
		}
	}

	t.Run("LoadDocumentsBasic", func(t *testing.T) {
		loader := service.NewDocumentLoader()
		docs, err := loader.LoadDocumentsFromFolderWithOptions(tempDir, service.DocumentLoaderOptions{})
		assert.NoError(t, err)
		assert.GreaterOrEqual(t, len(docs), 3) // At least 3 documents loaded
	})

	t.Run("LoadDocumentsWithOptions", func(t *testing.T) {
		loader := service.NewDocumentLoader()
		options := service.DocumentLoaderOptions{
			ExcludeDirs: []string{"subdir"},
			ExcludeExts: []string{".csv"},
			ProcessExts: []string{".txt"},
		}

		docs, err := loader.LoadDocumentsFromFolderWithOptions(tempDir, options)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(docs)) // Only test.txt should be loaded

		for _, doc := range docs {
			assert.Equal(t, "test.txt", doc.Name)
			assert.Contains(t, doc.Content, "This is a test document.")
		}
	})
}
