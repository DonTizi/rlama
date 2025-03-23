package domain

import (
	"testing"
)

func TestNewRagSystem(t *testing.T) {
	rag := NewRagSystem("test", "model")
	if rag.Name != "test" || rag.ModelName != "model" {
		t.Error("RAG system not initialized correctly")
	}
}
