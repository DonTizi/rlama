package vector

import "testing"

func TestNewEnhancedHybridStore(t *testing.T) {
	store, err := NewEnhancedHybridStore(":memory:", 1536)
	if err != nil {
		t.Errorf("Failed to create store: %v", err)
	}
	if store == nil {
		t.Error("Expected non-nil store")
	}
}
