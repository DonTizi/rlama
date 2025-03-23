package repository

import (
	"testing"
)

func TestNewRagRepository(t *testing.T) {
	repo := NewRagRepository()
	if repo == nil {
		t.Error("Expected non-nil repository")
	}
}
