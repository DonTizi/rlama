package client

import (
	"testing"
)

func TestNewOllamaClient(t *testing.T) {
	client := NewDefaultOllamaClient()
	if client == nil {
		t.Error("Expected non-nil client")
	}
}
