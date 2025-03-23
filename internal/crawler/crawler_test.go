package crawler

import "testing"

func TestNewWebCrawler(t *testing.T) {
	crawler, err := NewWebCrawler("https://example.com", 2, 1, nil)
	if err != nil {
		t.Errorf("Failed to create crawler: %v", err)
	}
	if crawler == nil {
		t.Error("Expected non-nil crawler")
	}
}
