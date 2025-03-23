package service

import "testing"

func TestNewRagService(t *testing.T) {
	svc := NewRagService(nil)
	if svc == nil {
		t.Error("Expected non-nil service")
	}
}
