package crawler

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/dontizi/rlama/internal/domain"
	"golang.org/x/net/html/charset"
)

// WebCrawler manage web crawling operations
type WebCrawler struct {
	client       *http.Client
	baseURL      *url.URL
	maxDepth     int
	concurrency  int
	excludePaths []string
	visited      map[string]bool
	visitedMutex sync.Mutex
	useSitemap   bool     // Option to use sitemap
	singleURL    bool     // Option to crawl only the specified URL
	urlsList     []string // Custom list of URLs to crawl
}

// NewWebCrawler creates a new web crawler
func NewWebCrawler(urlStr string, maxDepth, concurrency int, excludePaths []string) (*WebCrawler, error) {
	baseURL, err := url.Parse(urlStr)
	if err != nil {
		return nil, fmt.Errorf("invalid base URL: %w", err)
	}

	return &WebCrawler{
		client:       &http.Client{Timeout: 30 * time.Second},
		baseURL:      baseURL,
		maxDepth:     maxDepth,
		concurrency:  concurrency,
		excludePaths: excludePaths,
		visited:      make(map[string]bool),
		useSitemap:   true,  // By default, use sitemap if available
		singleURL:    false, // By default, do normal crawling
		urlsList:     nil,   // By default, no custom list
	}, nil
}

// isWebContent checks if a URL points to text/HTML content rather than binary files
func isWebContent(urlStr string) bool {
	// Extensions to ignore (binary files, etc.)
	excludeExtensions := []string{
		".zip", ".rar", ".tar", ".gz", ".pdf", ".doc", ".docx",
		".xls", ".xlsx", ".ppt", ".pptx", ".exe", ".bin",
		".dmg", ".iso", ".img", ".apk", ".ipa", ".mp3",
		".mp4", ".avi", ".mov", ".flv", ".mkv",
	}

	lowerURL := strings.ToLower(urlStr)
	for _, ext := range excludeExtensions {
		if strings.HasSuffix(lowerURL, ext) {
			return false
		}
	}

	return true
}

// CrawlWebsite crawls the website and returns the documents
func (wc *WebCrawler) CrawlWebsite() ([]domain.Document, error) {
	// If single URL mode, only crawl the base URL
	if wc.singleURL {
		return wc.crawlSingleURL()
	}

	// If custom list of URLs, use this list
	if len(wc.urlsList) > 0 {
		return wc.crawlURLsList()
	}

	// Otherwise, normal behavior with sitemap or standard crawling
	// Try to find a sitemap first
	if wc.useSitemap {
		sitemapURLs := []string{
			fmt.Sprintf("%s://%s/sitemap.xml", wc.baseURL.Scheme, wc.baseURL.Host),
			fmt.Sprintf("%s://%s/sitemap_index.xml", wc.baseURL.Scheme, wc.baseURL.Host),
		}

		for _, sitemapURL := range sitemapURLs {
			urls, err := wc.parseSitemap(sitemapURL)
			if err == nil && len(urls) > 0 {
				fmt.Printf("Found sitemap at %s with %d URLs\n", sitemapURL, len(urls))
				return wc.crawlURLsFromSitemap(urls)
			}
		}
		fmt.Println("No sitemap found or error parsing sitemap, falling back to standard crawling")
	}

	// If no sitemap or option disabled, continue with standard crawling
	return wc.crawlStandard()
}

// crawlSingleURL crawls only the base URL without following any links
func (wc *WebCrawler) crawlSingleURL() ([]domain.Document, error) {
	fmt.Println("Single URL mode: crawling only the specified URL without following links")

	var documents []domain.Document

	// Fetch and parse the single URL
	doc, err := wc.fetchAndParseURL(wc.baseURL.String())
	if err != nil {
		return nil, fmt.Errorf("error fetching single URL %s: %w", wc.baseURL.String(), err)
	}

	if doc != nil {
		documents = append(documents, *doc)
	}

	return documents, nil
}

// crawlURLsList crawls the specific list of URLs provided by the user
func (wc *WebCrawler) crawlURLsList() ([]domain.Document, error) {
	fmt.Printf("URLs list mode: crawling %d specific URLs\n", len(wc.urlsList))

	var documents []domain.Document
	var wg sync.WaitGroup
	var mu sync.Mutex
	semaphore := make(chan struct{}, wc.concurrency)
	errorChan := make(chan error, len(wc.urlsList))

	for _, urlStr := range wc.urlsList {
		// Check if the URL should be excluded
		shouldExclude := false
		for _, exclude := range wc.excludePaths {
			if strings.Contains(urlStr, exclude) {
				shouldExclude = true
				break
			}
		}

		if shouldExclude {
			continue
		}

		wg.Add(1)
		semaphore <- struct{}{}

		go func(url string) {
			defer wg.Done()
			defer func() { <-semaphore }()

			// Use the existing URL crawling function
			doc, err := wc.fetchAndParseURL(url)
			if err != nil {
				errorChan <- err
				return
			}

			if doc != nil {
				mu.Lock()
				documents = append(documents, *doc)
				mu.Unlock()
			}
		}(urlStr)
	}

	wg.Wait()
	close(errorChan)

	// Log any errors but continue with the documents we have
	for err := range errorChan {
		fmt.Printf("Warning during crawling: %v\n", err)
	}

	return documents, nil
}

// crawlStandard performs the standard crawling
func (wc *WebCrawler) crawlStandard() ([]domain.Document, error) {
	var documents []domain.Document
	visited := make(map[string]bool)
	queue := []string{wc.baseURL.String()}

	for len(queue) > 0 && len(visited) <= wc.maxDepth*100 {
		url := queue[0]
		queue = queue[1:]

		if visited[url] {
			continue
		}
		visited[url] = true

		doc, err := wc.fetchAndParseURL(url)
		if err != nil {
			fmt.Printf("Warning: Error fetching %s: %v\n", url, err)
			continue
		}

		if doc != nil {
			documents = append(documents, *doc)
		}

		// Don't crawl deeper if we've reached the maximum depth
		urlDepth := strings.Count(url, "/")
		baseDepth := strings.Count(wc.baseURL.String(), "/")
		if urlDepth-baseDepth >= wc.maxDepth {
			continue
		}

		// Find the links on the page
		links, err := wc.extractLinks(url)
		if err != nil {
			fmt.Printf("Warning: Error extracting links from %s: %v\n", url, err)
			continue
		}

		queue = append(queue, links...)
	}

	return documents, nil
}

// extractLinks gets all valid links from a page
func (wc *WebCrawler) extractLinks(urlStr string) ([]string, error) {
	resp, err := wc.client.Get(urlStr)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	var links []string
	doc.Find("a[href]").Each(func(i int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if !exists {
			return
		}

		// Convert to absolute URL
		absURL, err := wc.resolveURL(href)
		if err != nil {
			return
		}

		// Check if the URL is on the same domain
		if !wc.isSameDomain(absURL) {
			return
		}

		// Check the exclusions
		for _, exclude := range wc.excludePaths {
			if strings.Contains(absURL, exclude) {
				return
			}
		}

		links = append(links, absURL)
	})

	return links, nil
}

// resolveURL converts a relative URL to absolute
func (wc *WebCrawler) resolveURL(href string) (string, error) {
	relURL, err := url.Parse(href)
	if err != nil {
		return "", err
	}
	absURL := wc.baseURL.ResolveReference(relURL)
	return absURL.String(), nil
}

// isSameDomain checks if a URL is on the same domain as the base URL
func (wc *WebCrawler) isSameDomain(urlStr string) bool {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return false
	}
	return parsedURL.Host == wc.baseURL.Host
}

// convertToMarkdown converts HTML content to Markdown
func (wc *WebCrawler) convertToMarkdown(doc *goquery.Document) string {
	// Remove unwanted elements
	doc.Find("script, style, noscript, iframe, svg").Remove()

	// Get the main content
	var content string
	mainContent := doc.Find("main, article, .content, #content, .main, #main").First()
	if mainContent.Length() > 0 {
		content = mainContent.Text()
	} else {
		content = doc.Find("body").Text()
	}

	// Basic cleanup
	content = strings.TrimSpace(content)
	content = strings.ReplaceAll(content, "\n\n\n", "\n\n")

	return content
}

// fetchAndParseURL fetches and parses a single URL
func (wc *WebCrawler) fetchAndParseURL(urlStr string) (*domain.Document, error) {
	fmt.Printf("Crawling: %s\n", urlStr)
	resp, err := wc.client.Get(urlStr)
	if err != nil {
		return nil, fmt.Errorf("error fetching %s: %w", urlStr, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("received status code %d for %s", resp.StatusCode, urlStr)
	}

	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "text/html") &&
		!strings.Contains(contentType, "text/plain") &&
		!strings.Contains(contentType, "application/xhtml+xml") {
		return nil, nil
	}

	reader, err := charset.NewReader(resp.Body, contentType)
	if err != nil {
		return nil, fmt.Errorf("error creating reader: %w", err)
	}

	doc, err := goquery.NewDocumentFromReader(reader)
	if err != nil {
		return nil, fmt.Errorf("error parsing HTML: %w", err)
	}

	title := doc.Find("title").Text()
	title = strings.TrimSpace(title)
	if title == "" {
		title = doc.Find("h1").First().Text()
		title = strings.TrimSpace(title)
	}
	if title == "" {
		title = urlStr
	}

	// Use convertToMarkdown instead of extractMarkdownFromHTML
	content := wc.convertToMarkdown(doc)

	document := &domain.Document{
		URL:     urlStr,
		Path:    wc.getRelativePath(urlStr),
		Content: fmt.Sprintf("# %s\n\n%s", title, content),
	}

	return document, nil
}

// getRelativePath returns the relative path of a URL to the base URL
func (wc *WebCrawler) getRelativePath(urlStr string) string {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return ""
	}

	if parsedURL.Host == wc.baseURL.Host {
		return parsedURL.Path
	}
	return ""
}

// extractContentAsMarkdown extracts main content from an HTML document and converts it to Markdown
func extractContentAsMarkdown(doc *goquery.Document) (string, error) {
	// Create a Crawl4AI style converter
	converter := NewCrawl4AIStyleConverter()

	// Use the enhanced converter for HTML to Markdown conversion
	baseURL, _ := url.Parse("")
	markdown, err := converter.ConvertHTMLToMarkdown(doc, baseURL)
	if err != nil {
		return "", err
	}

	return markdown, nil
}

// SetUseSitemap sets whether to use sitemap for crawling
func (wc *WebCrawler) SetUseSitemap(useSitemap bool) {
	wc.useSitemap = useSitemap
}

// SetSingleURLMode sets whether to crawl only the specified URL without following links
func (wc *WebCrawler) SetSingleURLMode(singleURL bool) {
	wc.singleURL = singleURL
}

// SetURLsList sets a custom list of URLs to crawl
func (wc *WebCrawler) SetURLsList(urlsList []string) {
	wc.urlsList = urlsList
}

// parseSitemap parses a sitemap XML and returns the list of URLs
func (wc *WebCrawler) parseSitemap(sitemapURL string) ([]string, error) {
	resp, err := wc.client.Get(sitemapURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("sitemap request returned status code %d", resp.StatusCode)
	}

	// Use goquery to parse the XML
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	var urls []string

	// Find all <loc> tags in the sitemap
	doc.Find("url loc").Each(func(i int, s *goquery.Selection) {
		url := strings.TrimSpace(s.Text())
		if url != "" {
			urls = append(urls, url)
		}
	})

	return urls, nil
}

// crawlURLsFromSitemap crawls all URLs found in the sitemap
func (wc *WebCrawler) crawlURLsFromSitemap(urls []string) ([]domain.Document, error) {
	var documents []domain.Document
	var wg sync.WaitGroup
	var mu sync.Mutex
	semaphore := make(chan struct{}, wc.concurrency)
	errorChan := make(chan error, len(urls))

	for _, urlStr := range urls {
		// Check if the URL should be excluded
		shouldExclude := false
		for _, exclude := range wc.excludePaths {
			if strings.Contains(urlStr, exclude) {
				shouldExclude = true
				break
			}
		}

		if shouldExclude {
			continue
		}

		// Mark as visited
		wc.visitedMutex.Lock()
		wc.visited[urlStr] = true
		wc.visitedMutex.Unlock()

		wg.Add(1)
		semaphore <- struct{}{}

		go func(url string) {
			defer wg.Done()
			defer func() { <-semaphore }()

			// Use the existing URL crawling function
			doc, err := wc.fetchAndParseURL(url)
			if err != nil {
				errorChan <- err
				return
			}

			if doc != nil {
				mu.Lock()
				documents = append(documents, *doc)
				mu.Unlock()
			}
		}(urlStr)
	}

	wg.Wait()
	close(errorChan)

	// Log any errors but continue with the documents we have
	for err := range errorChan {
		fmt.Printf("Warning during crawling: %v\n", err)
	}

	return documents, nil
}
