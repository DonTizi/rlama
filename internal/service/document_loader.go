package service

import (
	// Suppression des imports non utilisés
	// "bytes"
	// "encoding/json"
	"fmt"
	// "io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/dontizi/rlama/internal/domain"
	"github.com/ledongthuc/pdf"
)

// DocumentLoader is responsible for loading documents from the file system
type DocumentLoader struct {
	supportedExtensions map[string]bool
	extractorPath       string // Path to the external extractor
}

// NewDocumentLoader creates a new instance of DocumentLoader
func NewDocumentLoader() *DocumentLoader {
	return &DocumentLoader{
		supportedExtensions: map[string]bool{
			// Plain text
			".txt":   true,
			".md":    true,
			".html":  true,
			".htm":   true,
			".json":  true,
			".csv":   true,
			".log":   true,
			".xml":   true,
			".yaml":  true,
			".yml":   true,
			// Source code
			".go":    true,
			".py":    true,
			".js":    true,
			".java":  true,
			".c":     true,
			".cpp":   true,
			".h":     true,
			".rb":    true,
			".php":   true,
			".rs":    true,
			".swift": true,
			".kt":    true,
			// Documents
			".pdf":   true,
			".docx":  true,
			".doc":   true,
			".rtf":   true,
			".odt":   true,
			".pptx":  true,
			".ppt":   true,
			".xlsx":  true,
			".xls":   true,
			".epub":  true,
			".org":   true,
		},
		// We'll use pdftotext if available
		extractorPath: findExternalExtractor(),
	}
}

// findExternalExtractor looks for external extraction tools
func findExternalExtractor() string {
	// Priority of text extractors
	extractors := []string{
		"pdftotext", // For PDFs (Poppler-utils)
		"textutil",  // macOS
		"catdoc",    // For .doc
		"unrtf",     // For .rtf
	}

	for _, extractor := range extractors {
		path, err := exec.LookPath(extractor)
		if err == nil {
			fmt.Printf("External extractor found: %s\n", path)
			return path
		}
	}

	fmt.Println("No external extractor found. Text extraction will be limited.")
	return ""
}

// LoadDocumentsFromFolder loads all supported documents from the specified folder
func (dl *DocumentLoader) LoadDocumentsFromFolder(folderPath string) ([]*domain.Document, error) {
	var documents []*domain.Document
	var supportedFiles []string
	var unsupportedFiles []string

	// Check if the folder exists
	info, err := os.Stat(folderPath)
	if os.IsNotExist(err) {
		// Try to create the folder
		if err := os.MkdirAll(folderPath, 0755); err != nil {
			return nil, fmt.Errorf("folder '%s' does not exist and cannot be created: %w", folderPath, err)
		}
		fmt.Printf("Folder '%s' has been created.\n", folderPath)
		// Get information about the newly created folder
		info, err = os.Stat(folderPath)
		if err != nil {
			return nil, fmt.Errorf("unable to access folder '%s': %w", folderPath, err)
		}
	} else if err != nil {
		return nil, fmt.Errorf("unable to access folder '%s': %w", folderPath, err)
	}
	
	if !info.IsDir() {
		return nil, fmt.Errorf("the specified path is not a folder: %s", folderPath)
	}

	// Preliminary file check
	err = filepath.Walk(folderPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Ignore folders and hidden files (starting with .)
		if info.IsDir() || strings.HasPrefix(filepath.Base(path), ".") {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		if dl.supportedExtensions[ext] {
			supportedFiles = append(supportedFiles, path)
		} else {
			unsupportedFiles = append(unsupportedFiles, path)
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("error while analyzing folder: %w", err)
	}

	// Display info about found files
	if len(supportedFiles) == 0 {
		if len(unsupportedFiles) == 0 {
			return nil, fmt.Errorf("folder '%s' is empty. Please add documents before creating a RAG", folderPath)
		} else {
			extensionsMsg := "Supported extensions: "
			for ext := range dl.supportedExtensions {
				extensionsMsg += ext + " "
			}
			return nil, fmt.Errorf("no supported files found in '%s'. %d unsupported files detected.\n%s", 
				folderPath, len(unsupportedFiles), extensionsMsg)
		}
	}

	fmt.Printf("Found %d supported files and %d unsupported files.\n", len(supportedFiles), len(unsupportedFiles))
	
	// Try to install dependencies if possible
	dl.tryInstallDependencies()
	
	// Process supported files
	for _, path := range supportedFiles {
		ext := strings.ToLower(filepath.Ext(path))
		
		// Text extraction using multiple methods
		textContent, err := dl.extractText(path, ext)
		if err != nil {
			fmt.Printf("Warning: unable to extract text from %s: %v\n", path, err)
			fmt.Println("Attempting extraction as raw text...")
			
			// Try reading as a text file
			rawContent, err := ioutil.ReadFile(path)
			if err != nil {
				fmt.Printf("Failed to read raw %s: %v\n", path, err)
				continue
			}
			
			textContent = string(rawContent)
		}

		// Check that the content is not empty
		if strings.TrimSpace(textContent) == "" {
			fmt.Printf("Warning: no text extracted from %s\n", path)
			
			// For PDFs, try one last method
			if ext == ".pdf" {
				fmt.Println("Attempting extraction with OCR (if installed)...")
				ocrText, err := dl.extractWithOCR(path)
				if err != nil || strings.TrimSpace(ocrText) == "" {
					fmt.Println("OCR failed or not available.")
					continue
				}
				textContent = ocrText
			} else {
				continue
			}
		}

		// Create a document
		doc := domain.NewDocument(path, textContent)
		documents = append(documents, doc)
		fmt.Printf("Document added: %s (%d characters)\n", filepath.Base(path), len(textContent))
	}

	if len(documents) == 0 {
		return nil, fmt.Errorf("no documents with valid content found in folder '%s'", folderPath)
	}

	return documents, nil
}

// extractText extracts text from a file using the appropriate method based on type
func (dl *DocumentLoader) extractText(path string, ext string) (string, error) {
	switch ext {
	case ".pdf":
		return dl.extractFromPDF(path)
	case ".docx", ".doc", ".rtf", ".odt":
		return dl.extractFromDocument(path, ext)
	case ".pptx", ".ppt":
		return dl.extractFromPresentation(path, ext)
	case ".xlsx", ".xls":
		return dl.extractFromSpreadsheet(path, ext)
	default:
		// Treat as a text file
		data, err := ioutil.ReadFile(path)
		if err != nil {
			return "", err
		}
		return string(data), nil
	}
}

// extractFromPDF extracts text from a PDF using different methods
func (dl *DocumentLoader) extractFromPDF(path string) (string, error) {
	// Method 1: Use pdftotext if available
	if strings.Contains(dl.extractorPath, "pdftotext") {
		fmt.Printf("Extracting PDF with pdftotext: %s\n", filepath.Base(path))
		out, err := exec.Command(dl.extractorPath, "-layout", path, "-").Output()
		if err == nil && len(out) > 0 {
			return string(out), nil
		}
		fmt.Printf("pdftotext failed: %v\n", err)
	}
	
	// Method 2: Try with other tools (pdfinfo, pdftk)
	for _, tool := range []string{"pdfinfo", "pdftk"} {
		toolPath, err := exec.LookPath(tool)
		if err == nil {
			fmt.Printf("Attempting extraction with %s\n", tool)
			var out []byte
			if tool == "pdfinfo" {
				out, err = exec.Command(toolPath, path).Output()
			} else {
				out, err = exec.Command(toolPath, path, "dump_data").Output()
			}
			if err == nil && len(out) > 0 {
				return string(out), nil
			}
		}
	}
	
	// Method 3: Last attempt, open as binary file and extract strings
	fmt.Println("Extracting strings from PDF...")
	return dl.extractStringsFromBinary(path)
}

// extractFromDocument extracts text from a Word document or similar
func (dl *DocumentLoader) extractFromDocument(path string, ext string) (string, error) {
	// Method 1: Use textutil on macOS
	if strings.Contains(dl.extractorPath, "textutil") && (ext == ".docx" || ext == ".doc" || ext == ".rtf") {
		fmt.Printf("Extracting document with textutil: %s\n", filepath.Base(path))
		out, err := exec.Command(dl.extractorPath, "-convert", "txt", "-stdout", path).Output()
		if err == nil && len(out) > 0 {
			return string(out), nil
		}
	}
	
	// Method 2: Use catdoc for .doc
	if ext == ".doc" {
		catdocPath, err := exec.LookPath("catdoc")
		if err == nil {
			out, err := exec.Command(catdocPath, path).Output()
			if err == nil && len(out) > 0 {
				return string(out), nil
			}
		}
	}
	
	// Method 3: Use unrtf for .rtf
	if ext == ".rtf" {
		unrtfPath, err := exec.LookPath("unrtf")
		if err == nil {
			out, err := exec.Command(unrtfPath, "--text", path).Output()
			if err == nil && len(out) > 0 {
				return string(out), nil
			}
		}
	}
	
	// Method 4: Extract strings
	return dl.extractStringsFromBinary(path)
}

// extractFromPresentation extracts text from a PowerPoint presentation
func (dl *DocumentLoader) extractFromPresentation(path string, ext string) (string, error) {
	// External tools for PowerPoint are limited
	return dl.extractStringsFromBinary(path)
}

// extractFromSpreadsheet extracts text from an Excel spreadsheet
func (dl *DocumentLoader) extractFromSpreadsheet(path string, ext string) (string, error) {
	// Try to use xlsx2csv for .xlsx
	if ext == ".xlsx" {
		xlsx2csvPath, err := exec.LookPath("xlsx2csv")
		if err == nil {
			out, err := exec.Command(xlsx2csvPath, path).Output()
			if err == nil && len(out) > 0 {
				return string(out), nil
			}
		}
	}
	
	// Try to use xls2csv for .xls
	if ext == ".xls" {
		xls2csvPath, err := exec.LookPath("xls2csv")
		if err == nil {
			out, err := exec.Command(xls2csvPath, path).Output()
			if err == nil && len(out) > 0 {
				return string(out), nil
			}
		}
	}
	
	// Extract strings
	return dl.extractStringsFromBinary(path)
}

// extractStringsFromBinary extracts strings from a binary file
func (dl *DocumentLoader) extractStringsFromBinary(path string) (string, error) {
	// Use the 'strings' tool if available (Unix/Linux/macOS)
	stringsPath, err := exec.LookPath("strings")
	if err == nil {
		out, err := exec.Command(stringsPath, path).Output()
		if err == nil && len(out) > 0 {
			return string(out), nil
		}
	}
	
	// Basic implementation of 'strings' in Go
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return "", err
	}
	
	var result strings.Builder
	var currentWord strings.Builder
	
	for _, b := range data {
		if (b >= 32 && b <= 126) || b == '\n' || b == '\t' || b == '\r' {
			currentWord.WriteByte(b)
		} else {
			if currentWord.Len() >= 4 { // Word of at least 4 characters
				result.WriteString(currentWord.String())
				result.WriteString(" ")
			}
			currentWord.Reset()
		}
	}
	
	if currentWord.Len() >= 4 {
		result.WriteString(currentWord.String())
	}
	
	return result.String(), nil
}

// extractWithOCR attempts to extract text using OCR
func (dl *DocumentLoader) extractWithOCR(path string) (string, error) {
	// Check if tesseract is available
	tesseractPath, err := exec.LookPath("tesseract")
	if err != nil {
		return "", fmt.Errorf("OCR not available: tesseract not found")
	}
	
	// Create a temporary output file
	tempDir, err := ioutil.TempDir("", "rlama-ocr")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tempDir)
	
	outBasePath := filepath.Join(tempDir, "out")
	
	// For PDFs, first convert to images if possible
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".pdf" {
		// Check if pdftoppm is available
		pdftoppmPath, err := exec.LookPath("pdftoppm")
		if err == nil {
			// Convert PDF to images
			fmt.Println("Converting PDF to images for OCR...")
			cmd := exec.Command(pdftoppmPath, "-png", path, filepath.Join(tempDir, "page"))
			if err := cmd.Run(); err != nil {
				return "", fmt.Errorf("failed to convert PDF to images: %w", err)
			}
			
			// OCR on each image
			var allText strings.Builder
			imgFiles, _ := filepath.Glob(filepath.Join(tempDir, "page-*.png"))
			for _, imgFile := range imgFiles {
				fmt.Printf("OCR on %s...\n", filepath.Base(imgFile))
				cmd := exec.Command(tesseractPath, imgFile, outBasePath, "-l", "eng")
				if err := cmd.Run(); err != nil {
					fmt.Printf("Warning: OCR failed for %s: %v\n", imgFile, err)
					continue
				}
				
				// Read the extracted text
				textBytes, err := ioutil.ReadFile(outBasePath + ".txt")
				if err != nil {
					continue
				}
				
				allText.WriteString(string(textBytes))
				allText.WriteString("\n\n")
			}
			
			return allText.String(), nil
		}
	}
	
	// Direct OCR on the file (for images)
	cmd := exec.Command(tesseractPath, path, outBasePath, "-l", "eng")
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("OCR failed: %w", err)
	}
	
	// Read the extracted text
	textBytes, err := ioutil.ReadFile(outBasePath + ".txt")
	if err != nil {
		return "", err
	}
	
	return string(textBytes), nil
}

// tryInstallDependencies attempts to install dependencies if necessary
func (dl *DocumentLoader) tryInstallDependencies() {
	// Check if pip is available (for Python tools)
	pipPath, err := exec.LookPath("pip3")
	if err != nil {
		pipPath, err = exec.LookPath("pip")
	}
	
	if err == nil {
		fmt.Println("Checking Python text extraction tools...")
		// Try to install useful packages
		for _, pkg := range []string{"pdfminer.six", "docx2txt", "xlsx2csv"} {
			cmd := exec.Command(pipPath, "show", pkg)
			if err := cmd.Run(); err != nil {
				fmt.Printf("Installing %s...\n", pkg)
				installCmd := exec.Command(pipPath, "install", "--user", pkg)
				installCmd.Run() // Ignore errors
			}
		}
	}
}

// Add this function to extract text from PDFs
func extractTextFromPDF(pdfPath string) (string, error) {
	// Add a dependency for PDF text extraction
	// go get github.com/ledongthuc/pdf
	
	f, err := os.Open(pdfPath)
	if err != nil {
		return "", err
	}
	defer f.Close()
	
	// Use the PDF library to extract text
	pdfReader, err := pdf.NewReader(f, f.Size())
	if err != nil {
		return "", err
	}
	
	var textBuilder strings.Builder
	for i := 1; i <= pdfReader.NumPage(); i++ {
		page := pdfReader.Page(i)
		if page.V.IsNull() {
			continue
		}
		
		text, err := page.GetPlainText(nil)
		if err != nil {
			continue // Skip pages with errors
		}
		
		textBuilder.WriteString(text)
	}
	
	return textBuilder.String(), nil
} 