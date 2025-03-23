package crawler

import (
	"net/url"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

func TestCrawl4AIStyleConverter(t *testing.T) {
	// Sample HTML content
	html := `
	<!DOCTYPE html>
	<html>
	<head>
		<title>Test Page</title>
	</head>
	<body>
		<header>This is a header that should be removed</header>
		<nav>This is navigation that should be removed</nav>

		<main>
			<h1>Main Heading</h1>
			<p>This is a <strong>paragraph</strong> with <em>some</em> formatting.</p>
			
			<h2>Subheading</h2>
			<ul>
				<li>Item 1</li>
				<li>Item 2</li>
				<li>Item 3</li>
			</ul>

			<p>Here's a <a href="https://example.com">link</a> to an example site.</p>
		</main>

		<footer>This is a footer that should be removed</footer>
		<script>alert('This script should be removed');</script>
	</body>
	</html>
	`

	// Set up the test
	reader := strings.NewReader(html)
	doc, err := goquery.NewDocumentFromReader(reader)
	if err != nil {
		t.Fatalf("Failed to create document: %v", err)
	}

	// Create converter and run conversion
	converter := NewCrawl4AIStyleConverter()
	baseURL, _ := url.Parse("https://example.com")
	markdown, err := converter.ConvertHTMLToMarkdown(doc, baseURL)
	if err != nil {
		t.Fatalf("Failed to convert HTML to Markdown: %v", err)
	}

	// Check result
	t.Logf("Generated Markdown:\n%s", markdown)

	// Verify content was processed correctly
	expectedStrings := []string{
		"# Main Heading",
		"This is a **paragraph** with *some* formatting.",
		"## Subheading",
		"- Item 1",
		"- Item 2",
		"- Item 3",
		"[link](https://example.com)",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(markdown, expected) {
			t.Errorf("Expected Markdown to contain '%s', but it doesn't", expected)
		}
	}

	// Verify unwanted elements were removed
	unwantedStrings := []string{
		"This is a header that should be removed",
		"This is navigation that should be removed",
		"This is a footer that should be removed",
		"alert('This script should be removed');",
	}

	for _, unwanted := range unwantedStrings {
		if strings.Contains(markdown, unwanted) {
			t.Errorf("Markdown should not contain '%s', but it does", unwanted)
		}
	}
}
