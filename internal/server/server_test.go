package server

import "testing"

func TestNewServer(t *testing.T) {
	srv := NewServer("11249", nil)
	if srv == nil {
		t.Error("Expected non-nil server")
	}
}
