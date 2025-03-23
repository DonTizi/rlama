package crawler

import (
	"net/url"
	"strings"

	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"
	"github.com/PuerkitoBio/goquery"
)

// Crawl4AIStyleConverter provides enhanced HTML to Markdown conversion
// inspired by Crawl4AI's approach to create LLM-friendly markdown content
type Crawl4AIStyleConverter struct{}

// NewCrawl4AIStyleConverter creates a new Markdown converter with enhancements
func NewCrawl4AIStyleConverter() *Crawl4AIStyleConverter {
	return &Crawl4AIStyleConverter{}
}

// ConvertHTMLToMarkdown converts HTML content to Markdown with optimizations
func (c *Crawl4AIStyleConverter) ConvertHTMLToMarkdown(doc *goquery.Document, baseURL *url.URL) (string, error) {
	// Pre-process the document
	c.cleanDocument(doc)

	// Extract main content
	contentNode := c.extractMainContent(doc)

	// Get HTML content from the main content
	html, err := contentNode.Html()
	if err != nil {
		return "", err
	}

	// Convert to markdown
	markdown, err := htmltomarkdown.ConvertString(html)
	if err != nil {
		return "", err
	}

	// Post-process markdown to clean it up
	markdown = c.postProcessMarkdown(markdown)

	return markdown, nil
}

// cleanDocument removes unwanted elements from the HTML document
func (c *Crawl4AIStyleConverter) cleanDocument(doc *goquery.Document) {
	// Remove unwanted elements that typically don't contain useful content
	doc.Find("script, style, noscript, iframe, svg, form, " +
		"header, nav:not(.main-navigation), footer, aside, " +
		".cookie-banner, .cookie-dialog, .cookie-consent, .cookie-notice, " +
		".ad, .ads, .advertisement, .banner, " +
		".popup, .modal, .newsletter, .subscription, " +
		"[role='banner'], [role='complementary'], [role='contentinfo'], " +
		"[aria-hidden='true'], .hidden, .visually-hidden").Remove()
}

// extractMainContent finds the main content node of the document
func (c *Crawl4AIStyleConverter) extractMainContent(doc *goquery.Document) *goquery.Selection {
	// Try to find the main content area using common selectors
	contentSelectors := []string{
		"main", "article", "[role='main']",
		"#main", "#content", "#main-content", "#primaryContent",
		".main", ".content", ".main-content", ".article-content", ".post-content",
		"[itemprop='articleBody']", "[itemprop='mainContentOfPage']",
	}

	for _, selector := range contentSelectors {
		selection := doc.Find(selector)
		if selection.Length() > 0 {
			// Verify the selection has substantive content
			text := selection.Text()
			if len(strings.TrimSpace(text)) > 200 { // If it has more than 200 chars of text
				return selection
			}
		}
	}

	// Fallback: If no main content area could be determined, use body
	return doc.Find("body")
}

// postProcessMarkdown cleans up the generated markdown
func (c *Crawl4AIStyleConverter) postProcessMarkdown(markdown string) string {
	// Replace multiple blank lines with a single blank line
	for strings.Contains(markdown, "\n\n\n") {
		markdown = strings.ReplaceAll(markdown, "\n\n\n", "\n\n")
	}

	// Remove trailing whitespace from each line
	lines := strings.Split(markdown, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " \t")
	}
	markdown = strings.Join(lines, "\n")

	// Trim leading and trailing whitespace from the entire string
	markdown = strings.TrimSpace(markdown) + "\n"

	return markdown
}
