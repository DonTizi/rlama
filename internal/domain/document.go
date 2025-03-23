package domain

import (
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Document represents a document indexed in the RAG system
type Document struct {
	ID          string    `json:"id"`
	Path        string    `json:"path"`
	Name        string    `json:"name"`
	Content     string    `json:"content"`
	Metadata    string    `json:"metadata"`
	Embedding   []float32 `json:"-"` // Do not serialize to JSON
	CreatedAt   time.Time `json:"created_at"`
	ContentType string    `json:"content_type"`
	Size        int64     `json:"size"`
	URL         string    `json:"url,omitempty"` // Source URL for web documents
}

// NewDocument creates a new instance of Document
func NewDocument(path string, content string) *Document {
	// Clean the extracted content
	cleanedContent := cleanExtractedText(content)

	return &Document{
		ID:          filepath.Base(path),
		Path:        path,
		Name:        filepath.Base(path),
		Content:     cleanedContent,
		Metadata:    "",
		Embedding:   nil,
		CreatedAt:   time.Now(),
		ContentType: guessContentType(path),
		Size:        int64(len(cleanedContent)),
	}
}

// cleanExtractedText cleans the extracted text to improve its quality
func cleanExtractedText(text string) string {
	// Replace non-printable characters with spaces
	re := regexp.MustCompile(`[\x00-\x09\x0B\x0C\x0E-\x1F\x7F]+`)
	text = re.ReplaceAllString(text, " ")

	// Replace sequences of more than 2 newlines with 2 newlines
	re = regexp.MustCompile(`\n{3,}`)
	text = re.ReplaceAllString(text, "\n\n")

	// Replace sequences of more than 2 spaces with 1 space
	re = regexp.MustCompile(`[ \t]{2,}`)
	text = re.ReplaceAllString(text, " ")

	// Remove lines that contain only special characters or numbers
	lines := strings.Split(text, "\n")
	var cleanedLines []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if len(trimmed) > 0 {
			// Check if the line contains at least some letters
			re = regexp.MustCompile(`[a-zA-Z]{2,}`)
			if re.MatchString(trimmed) || len(trimmed) > 20 {
				cleanedLines = append(cleanedLines, line)
			}
		}
	}

	return strings.Join(cleanedLines, "\n")
}

// guessContentType tries to determine the content type based on the file extension
func guessContentType(path string) string {
	ext := filepath.Ext(path)
	switch strings.ToLower(ext) {
	case ".txt":
		return "text/plain"
	case ".md", ".markdown":
		return "text/markdown"
	case ".html", ".htm":
		return "text/html"
	case ".pdf":
		return "application/pdf"
	case ".docx":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case ".doc":
		return "application/msword"
	case ".csv":
		return "text/csv"
	case ".json":
		return "application/json"
	case ".pptx":
		return "application/vnd.openxmlformats-officedocument.presentationml.presentation"
	case ".ppt":
		return "application/vnd.ms-powerpoint"
	case ".xlsx":
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	case ".xls":
		return "application/vnd.ms-excel"
	case ".rtf":
		return "application/rtf"
	case ".odt":
		return "application/vnd.oasis.opendocument.text"
	default:
		return "application/octet-stream"
	}
}
