package config

import "testing"

func TestGetDataDir(t *testing.T) {
	dir := GetDataDir()
	if dir == "" {
		t.Error("Expected non-empty data directory")
	}
}
