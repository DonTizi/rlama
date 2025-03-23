package service

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/dontizi/rlama/internal/domain"
)

// ChunkingConfig holds configuration for the document chunking process
type ChunkingConfig struct {
	ChunkSize        int    // Target size of each chunk in characters
	ChunkOverlap     int    // Number of characters to overlap between chunks
	IncludeMetadata  bool   // Whether to include metadata in chunk content
	ChunkingStrategy string // Strategy to use: "fixed", "semantic", "hybrid", "hierarchical"
}

// DefaultChunkingConfig returns a default configuration for chunking
func DefaultChunkingConfig() ChunkingConfig {
	return ChunkingConfig{
		ChunkSize:        1500, // Smaller chunks (~375 tokens) for better retrieval
		ChunkOverlap:     150,  // 10% overlap
		IncludeMetadata:  true,
		ChunkingStrategy: "hybrid", // Default to hybrid strategy
	}
}

// ChunkerService handles splitting documents into manageable chunks
type ChunkerService struct {
	config ChunkingConfig
}

// NewChunkerService creates a new chunker service with the specified config
func NewChunkerService(config ChunkingConfig) *ChunkerService {
	return &ChunkerService{
		config: config,
	}
}

// ChunkDocument splits a document into smaller chunks with metadata
// based on the selected chunking strategy
func (cs *ChunkerService) ChunkDocument(doc *domain.Document) []*domain.DocumentChunk {
	content := doc.Content
	chunkSize := cs.config.ChunkSize
	overlap := cs.config.ChunkOverlap

	// For very small documents, just return a single chunk regardless of strategy
	if len(content) <= chunkSize {
		chunk := domain.NewDocumentChunk(doc, content, 0, len(content), 0)
		return []*domain.DocumentChunk{chunk}
	}

	// Apply different chunking strategies based on configuration
	var chunks []*domain.DocumentChunk

	switch cs.config.ChunkingStrategy {
	case "auto":
		// For auto strategy, use the evaluator to determine optimal configuration
		evaluator := NewChunkingEvaluator(cs)
		optimalConfig := evaluator.GetOptimalChunkingConfig(doc)

		// Create a temporary chunker with the optimal configuration
		tempChunker := NewChunkerService(optimalConfig)

		// Use the optimal chunker to create chunks
		chunks = tempChunker.ChunkDocument(doc)

		// Store chunking strategy info in chunk metadata
		for _, chunk := range chunks {
			chunk.Metadata["chunking_strategy"] = optimalConfig.ChunkingStrategy
			chunk.Metadata["chunk_size"] = fmt.Sprintf("%d", optimalConfig.ChunkSize)
			chunk.Metadata["chunk_overlap"] = fmt.Sprintf("%d", optimalConfig.ChunkOverlap)
		}

		// Evaluate to get the metrics
		metrics := evaluator.EvaluateChunkingStrategy(doc, optimalConfig)

		// Print analysis information
		fmt.Printf("\nAuto chunking for '%s':\n", doc.Name)
		fmt.Printf("  Selected strategy: %s\n", optimalConfig.ChunkingStrategy)
		fmt.Printf("  Chunk size: %d, Overlap: %d\n", optimalConfig.ChunkSize, optimalConfig.ChunkOverlap)
		fmt.Printf("  Coherence score: %.4f\n", metrics.SemanticCoherenceScore)
		fmt.Printf("  Chunks created: %d\n", len(chunks))
	case "fixed":
		chunks = cs.createFixedSizeChunks(doc, content, chunkSize, overlap)
	case "semantic":
		chunks = cs.createSemanticChunks(doc, content, chunkSize, overlap)
	case "hierarchical":
		chunks = cs.createHierarchicalChunks(doc, content, chunkSize, overlap)
	case "hybrid":
		// Default hybrid approach - choose strategy based on content type
		chunks = cs.createHybridChunks(doc, content, chunkSize, overlap)
	default:
		// Fallback to hybrid if invalid strategy specified
		chunks = cs.createHybridChunks(doc, content, chunkSize, overlap)
	}

	// Safeguard: If we somehow got no chunks, fall back to fixed-size chunking
	if len(chunks) == 0 {
		chunks = cs.createFixedSizeChunks(doc, content, chunkSize, overlap)
	}

	fmt.Printf("Split document '%s' into %d chunks using '%s' strategy\n",
		doc.Name, len(chunks), cs.config.ChunkingStrategy)

	return chunks
}

// createHybridChunks selects the best chunking strategy based on document type
func (cs *ChunkerService) createHybridChunks(doc *domain.Document, content string, chunkSize int, overlap int) []*domain.DocumentChunk {
	// Check file extension and content characteristics to determine best strategy
	ext := strings.ToLower(filepath.Ext(doc.Path))

	// Determine if content is markdown (either by extension or content analysis)
	isMarkdown := ext == ".md" || ext == ".markdown" || strings.Contains(content, "# ") && strings.Contains(content, "\n## ")

	// Determine if content is HTML
	isHTML := ext == ".html" || ext == ".htm" ||
		(strings.Contains(content, "<html") && strings.Contains(content, "</html>"))

	// Determine if content is code
	isCode := ext == ".go" || ext == ".js" || ext == ".py" || ext == ".java" || ext == ".c" ||
		ext == ".cpp" || ext == ".rs" || ext == ".ts" || ext == ".rb" || ext == ".php"

	// Apply appropriate strategy based on content type
	if isMarkdown {
		return cs.createMarkdownBasedChunks(doc, content, chunkSize, overlap)
	} else if isHTML {
		return cs.createHTMLBasedChunks(doc, content, chunkSize, overlap)
	} else if isCode {
		return cs.createCodeBasedChunks(doc, content, chunkSize, overlap)
	} else if len(content) > chunkSize*5 { // Very long document
		return cs.createHierarchicalChunks(doc, content, chunkSize, overlap)
	} else {
		// Default to paragraph-based chunking for general text
		return cs.createParagraphBasedChunks(doc, content, chunkSize, overlap)
	}
}

// createSemanticChunks creates chunks based on semantic boundaries in the text
func (cs *ChunkerService) createSemanticChunks(doc *domain.Document, content string, chunkSize int, overlap int) []*domain.DocumentChunk {
	// For semantic chunking, we prioritize natural text boundaries
	// This is similar to paragraph-based but with more attention to headers and sections

	// Check if the document has headers (markdown-style or HTML-style)
	hasHeaders := regexp.MustCompile(`(?m)^#+\s|<h[1-6]>`).MatchString(content)

	if hasHeaders {
		// If the document has headers, chunk by sections
		return cs.createSectionBasedChunks(doc, content, chunkSize, overlap)
	} else {
		// Otherwise use paragraph chunking
		return cs.createParagraphBasedChunks(doc, content, chunkSize, overlap)
	}
}

// createSectionBasedChunks splits content based on headers and sections
func (cs *ChunkerService) createSectionBasedChunks(doc *domain.Document, content string, chunkSize int, overlap int) []*domain.DocumentChunk {
	var chunks []*domain.DocumentChunk

	// Find markdown headers or HTML headers
	headerPattern := regexp.MustCompile(`(?m)^(#+\s+.+$|<h[1-6]>.+</h[1-6]>)`)

	// Split content by headers
	sections := headerPattern.Split(content, -1)
	headers := headerPattern.FindAllString(content, -1)

	// Add a dummy header for the first section if it doesn't have one
	if len(headers) < len(sections) && len(sections[0]) > 0 {
		headers = append([]string{"# Introduction"}, headers...)
	} else if len(headers) > len(sections) {
		sections = append([]string{""}, sections...)
	}

	// Process each section
	startPos := 0
	chunkIndex := 0

	for i := 0; i < len(sections); i++ {
		// Skip empty sections
		if strings.TrimSpace(sections[i]) == "" {
			continue
		}

		// Combine header with its content
		var sectionContent string
		if i < len(headers) {
			sectionContent = headers[i] + sections[i]
		} else {
			sectionContent = sections[i]
		}

		// If section is too large, split it further
		if len(sectionContent) > chunkSize*2 {
			// Create sub-chunks for this section
			sectionChunks := cs.createParagraphBasedChunks(doc, sectionContent, chunkSize, overlap)

			// Update positions and indices
			for j, chunk := range sectionChunks {
				chunk.StartPos = startPos + chunk.StartPos
				chunk.EndPos = startPos + chunk.EndPos
				chunk.ChunkIndex = chunkIndex + j
				chunks = append(chunks, chunk)
			}

			chunkIndex += len(sectionChunks)
			startPos += len(sectionContent)
		} else {
			// Create a single chunk for this section
			endPos := startPos + len(sectionContent)
			chunk := domain.NewDocumentChunk(doc, sectionContent, startPos, endPos, chunkIndex)
			chunks = append(chunks, chunk)

			chunkIndex++
			startPos = endPos
		}
	}

	return chunks
}

// createHierarchicalChunks creates a two-level chunking structure
func (cs *ChunkerService) createHierarchicalChunks(doc *domain.Document, content string, chunkSize int, overlap int) []*domain.DocumentChunk {
	var chunks []*domain.DocumentChunk

	// For hierarchical chunking, we first split into major sections,
	// then we further split each section if needed

	// Split into major sections (try headers first, fall back to large chunks)
	headerPattern := regexp.MustCompile(`(?m)^(#+\s+.+$|<h[1-6]>.+</h[1-6]>)`)
	hasMajorSections := headerPattern.MatchString(content)

	if hasMajorSections {
		// Split by headers for major sections
		sections := headerPattern.Split(content, -1)
		headers := headerPattern.FindAllString(content, -1)

		// Add a dummy header for the first section if it doesn't have one
		if len(headers) < len(sections) && len(sections[0]) > 0 {
			headers = append([]string{"# Introduction"}, headers...)
		} else if len(headers) > len(sections) {
			sections = append([]string{""}, sections...)
		}

		// Process each section
		startPos := 0
		chunkIndex := 0

		for i := 0; i < len(sections); i++ {
			// Skip empty sections
			if strings.TrimSpace(sections[i]) == "" {
				continue
			}

			// Combine header with its content
			var sectionContent string
			if i < len(headers) {
				sectionContent = headers[i] + sections[i]
			} else {
				sectionContent = sections[i]
			}

			// For each major section, create sub-chunks
			majorSection := domain.NewDocumentChunk(doc, sectionContent, startPos, startPos+len(sectionContent), chunkIndex)
			majorSection.Metadata["chunk_type"] = "parent_section"
			chunks = append(chunks, majorSection)
			chunkIndex++

			// If section is large enough to need sub-chunks
			if len(sectionContent) > chunkSize {
				// Create sub-chunks with paragraph-based approach
				subChunks := cs.createParagraphBasedChunks(doc, sectionContent, chunkSize, overlap)

				// Update positions and indices for sub-chunks
				for j, chunk := range subChunks {
					chunk.StartPos = startPos + chunk.StartPos
					chunk.EndPos = startPos + chunk.EndPos
					chunk.ChunkIndex = chunkIndex + j
					chunk.Metadata["parent_chunk_id"] = majorSection.ID
					chunk.Metadata["chunk_type"] = "child_section"
					chunks = append(chunks, chunk)
				}

				chunkIndex += len(subChunks)
			}

			startPos += len(sectionContent)
		}
	} else {
		// No clear sections, create artificial major chunks
		majorChunkSize := chunkSize * 3

		// First create large parent chunks with minimal overlap
		for i := 0; i < len(content); i += majorChunkSize {
			end := i + majorChunkSize
			if end > len(content) {
				end = len(content)
			}

			// Try to break at paragraph boundaries
			if end < len(content) {
				for end > i && content[end] != '\n' {
					end--
				}
			}

			majorContent := content[i:end]
			majorChunk := domain.NewDocumentChunk(doc, majorContent, i, end, i/majorChunkSize)
			majorChunk.Metadata["chunk_type"] = "parent_section"
			chunks = append(chunks, majorChunk)

			// Then create smaller sub-chunks for each major chunk
			subChunks := cs.createParagraphBasedChunks(doc, majorContent, chunkSize, overlap)

			// Update positions and indices for sub-chunks
			for j, chunk := range subChunks {
				chunk.StartPos = i + chunk.StartPos
				chunk.EndPos = i + chunk.EndPos
				chunk.ChunkIndex = len(chunks) + j
				chunk.Metadata["parent_chunk_id"] = majorChunk.ID
				chunk.Metadata["chunk_type"] = "child_section"
				chunks = append(chunks, chunk)
			}
		}
	}

	return chunks
}

// createMarkdownBasedChunks optimizes chunking for markdown documents
func (cs *ChunkerService) createMarkdownBasedChunks(doc *domain.Document, content string, chunkSize int, overlap int) []*domain.DocumentChunk {
	// For markdown content, respect header structure
	return cs.createSectionBasedChunks(doc, content, chunkSize, overlap)
}

// createHTMLBasedChunks optimizes chunking for HTML documents
func (cs *ChunkerService) createHTMLBasedChunks(doc *domain.Document, content string, chunkSize int, overlap int) []*domain.DocumentChunk {
	// For HTML content, try to respect tag structure
	// This is a simplified implementation - a full HTML parser would be more accurate

	// Look for major HTML structural elements
	sectionPattern := regexp.MustCompile(`<(div|section|article|main|header|footer|nav|aside)[^>]*>.*?</(div|section|article|main|header|footer|nav|aside)>`)

	if sectionPattern.MatchString(content) {
		// If we find structural elements, try to use them for chunking
		sections := sectionPattern.FindAllStringIndex(content, -1)

		if len(sections) > 0 {
			var chunks []*domain.DocumentChunk
			chunkIndex := 0
			lastEnd := 0

			// Create chunks for content before first section if needed
			if sections[0][0] > 0 {
				preContent := content[:sections[0][0]]
				if len(strings.TrimSpace(preContent)) > 0 {
					chunk := domain.NewDocumentChunk(doc, preContent, 0, sections[0][0], chunkIndex)
					chunks = append(chunks, chunk)
					chunkIndex++
				}
			}

			// Process each section
			for i, section := range sections {
				sectionContent := content[section[0]:section[1]]

				// Handle gaps between sections
				if section[0] > lastEnd {
					gapContent := content[lastEnd:section[0]]
					if len(strings.TrimSpace(gapContent)) > 0 {
						chunk := domain.NewDocumentChunk(doc, gapContent, lastEnd, section[0], chunkIndex)
						chunks = append(chunks, chunk)
						chunkIndex++
					}
				}

				// If section is too large, split it further
				if len(sectionContent) > chunkSize*2 {
					// Strip HTML tags for better text chunking
					sectionChunks := cs.createParagraphBasedChunks(doc, sectionContent, chunkSize, overlap)

					// Update positions and indices
					for j, chunk := range sectionChunks {
						chunk.StartPos = section[0] + chunk.StartPos
						chunk.EndPos = section[0] + chunk.EndPos
						chunk.ChunkIndex = chunkIndex + j
						chunks = append(chunks, chunk)
					}

					chunkIndex += len(sectionChunks)
				} else {
					// Use section as a chunk
					chunk := domain.NewDocumentChunk(doc, sectionContent, section[0], section[1], chunkIndex)
					chunks = append(chunks, chunk)
					chunkIndex++
				}

				lastEnd = section[1]

				// Handle content after the last section
				if i == len(sections)-1 && section[1] < len(content) {
					postContent := content[section[1]:]
					if len(strings.TrimSpace(postContent)) > 0 {
						chunk := domain.NewDocumentChunk(doc, postContent, section[1], len(content), chunkIndex)
						chunks = append(chunks, chunk)
					}
				}
			}

			return chunks
		}
	}

	// Fall back to paragraph-based chunking if no clear structure is found
	return cs.createParagraphBasedChunks(doc, content, chunkSize, overlap)
}

// createCodeBasedChunks optimizes chunking for code documents
func (cs *ChunkerService) createCodeBasedChunks(doc *domain.Document, content string, chunkSize int, overlap int) []*domain.DocumentChunk {
	// For code content, try to respect function/class boundaries

	// Look for function/class definitions in various languages
	// This is a simplified approach - a language-specific parser would be more accurate
	functionPattern := regexp.MustCompile(`(?m)^(func|function|def|class|interface|struct|public|private|protected)\s+\w+[^{]*{`)

	if functionPattern.MatchString(content) {
		// Split by function/class definitions
		matches := functionPattern.FindAllStringIndex(content, -1)

		if len(matches) > 0 {
			var chunks []*domain.DocumentChunk
			chunkIndex := 0
			lastEnd := 0

			// Handle content before first function if needed
			if matches[0][0] > 0 {
				preContent := content[:matches[0][0]]
				if len(strings.TrimSpace(preContent)) > 0 {
					chunk := domain.NewDocumentChunk(doc, preContent, 0, matches[0][0], chunkIndex)
					chunks = append(chunks, chunk)
					chunkIndex++
				}
			}

			// Process each function match
			for i := 0; i < len(matches); i++ {
				start := matches[i][0]
				end := len(content)

				// Set end to the beginning of the next function
				if i < len(matches)-1 {
					end = matches[i+1][0]
				}

				// Handle content between last function end and current function start
				if start > lastEnd && lastEnd > 0 {
					gapContent := content[lastEnd:start]
					if len(strings.TrimSpace(gapContent)) > 0 {
						chunk := domain.NewDocumentChunk(doc, gapContent, lastEnd, start, chunkIndex)
						chunks = append(chunks, chunk)
						chunkIndex++
					}
				}

				// Extract function content
				functionContent := content[start:end]

				// If function is too large, split it further
				if len(functionContent) > chunkSize*2 {
					// Split by logical blocks (like try/catch, if/else)
					subChunks := cs.createFixedSizeChunks(doc, functionContent, chunkSize, overlap)

					// Update positions and indices
					for j, chunk := range subChunks {
						chunk.StartPos = start + chunk.StartPos
						chunk.EndPos = start + chunk.EndPos
						chunk.ChunkIndex = chunkIndex + j
						chunks = append(chunks, chunk)
					}

					chunkIndex += len(subChunks)
				} else {
					// Use function as a chunk
					chunk := domain.NewDocumentChunk(doc, functionContent, start, start+len(functionContent), chunkIndex)
					chunks = append(chunks, chunk)
					chunkIndex++
				}

				lastEnd = end
			}

			return chunks
		}
	}

	// Fall back to line-based chunking for code with no clear structure
	return cs.createFixedSizeChunks(doc, content, chunkSize, overlap)
}

// createFixedSizeChunks creates chunks of fixed size with overlap
func (cs *ChunkerService) createFixedSizeChunks(doc *domain.Document, content string, chunkSize int, overlap int) []*domain.DocumentChunk {
	var chunks []*domain.DocumentChunk
	contentLength := len(content)

	// Calculate total chunks needed
	totalChunks := (contentLength + chunkSize - overlap - 1) / (chunkSize - overlap)
	if totalChunks < 1 {
		totalChunks = 1
	}

	fmt.Printf("Document '%s' size: %d characters, creating %d chunks\n",
		doc.Name, contentLength, totalChunks)

	// Create chunks with overlap
	for i := 0; i < totalChunks; i++ {
		startPos := i * (chunkSize - overlap)
		if startPos > 0 && startPos < contentLength && i > 0 {
			// Adjust start position to avoid breaking words
			for startPos < contentLength && startPos > 0 && content[startPos] != ' ' && content[startPos] != '\n' {
				startPos++
			}
		}

		endPos := startPos + chunkSize
		if endPos > contentLength {
			endPos = contentLength
		} else {
			// Try to end at a natural break
			for endPos < contentLength && endPos > startPos && content[endPos] != ' ' && content[endPos] != '\n' {
				endPos--
			}
		}

		// Skip empty chunks
		if startPos >= endPos {
			continue
		}

		chunkContent := content[startPos:endPos]
		chunk := domain.NewDocumentChunk(doc, chunkContent, startPos, endPos, i)
		chunks = append(chunks, chunk)
	}

	return chunks
}

// createParagraphBasedChunks creates chunks based on paragraph boundaries
func (cs *ChunkerService) createParagraphBasedChunks(doc *domain.Document, content string, chunkSize int, overlap int) []*domain.DocumentChunk {
	var chunks []*domain.DocumentChunk

	// Split by paragraphs to maintain semantic units
	paragraphs := strings.Split(content, "\n\n")
	var currentChunk strings.Builder
	currentSize := 0
	chunkIndex := 0
	startPos := 0

	for i, para := range paragraphs {
		para = strings.TrimSpace(para)
		if para == "" {
			continue
		}

		paraSize := len(para) + 2 // +2 for newlines

		// If this paragraph alone exceeds chunk size, we need to split it
		if paraSize > chunkSize*2 {
			// If we have content in the current chunk, finalize it first
			if currentSize > 0 {
				chunkContent := currentChunk.String()
				endPos := startPos + len(chunkContent)
				chunk := domain.NewDocumentChunk(doc, chunkContent, startPos, endPos, chunkIndex)
				chunks = append(chunks, chunk)
				chunkIndex++

				currentChunk.Reset()
				currentSize = 0
				startPos = endPos
			}

			// Now split the large paragraph into fixed-size chunks
			paraChunks := cs.splitParagraphIntoChunks(doc, para, chunkSize, overlap, startPos, chunkIndex)
			chunks = append(chunks, paraChunks...)
			chunkIndex += len(paraChunks)

			// Update startPos for next chunk
			if len(paraChunks) > 0 {
				lastChunk := paraChunks[len(paraChunks)-1]
				startPos = lastChunk.EndPos
			}

			continue
		}

		// If adding this paragraph would exceed chunk size and we have content
		if currentSize+paraSize > chunkSize && currentSize > 0 {
			// Create a chunk from accumulated content
			chunkContent := currentChunk.String()
			endPos := startPos + len(chunkContent)
			chunk := domain.NewDocumentChunk(doc, chunkContent, startPos, endPos, chunkIndex)
			chunks = append(chunks, chunk)
			chunkIndex++

			// Handle overlap for the next chunk
			if overlap > 0 && len(chunkContent) > overlap {
				// Calculate where to start the next chunk with overlap
				overlapStart := len(chunkContent) - overlap
				if overlapStart < 0 {
					overlapStart = 0
				}

				// Start the new chunk with the end of the previous one
				currentChunk.Reset()
				overlapText := chunkContent[overlapStart:]
				currentChunk.WriteString(overlapText)
				currentSize = len(overlapText)
				startPos = endPos - len(overlapText)
			} else {
				currentChunk.Reset()
				currentSize = 0
				startPos = endPos
			}
		}

		// Add the paragraph to the current chunk
		if currentSize > 0 {
			currentChunk.WriteString("\n\n")
		}
		currentChunk.WriteString(para)
		currentSize += paraSize

		// Handle the last paragraph
		if i == len(paragraphs)-1 && currentSize > 0 {
			chunkContent := currentChunk.String()
			endPos := startPos + len(chunkContent)
			chunk := domain.NewDocumentChunk(doc, chunkContent, startPos, endPos, chunkIndex)
			chunks = append(chunks, chunk)
		}
	}

	return chunks
}

// splitParagraphIntoChunks splits a single large paragraph into multiple chunks
func (cs *ChunkerService) splitParagraphIntoChunks(doc *domain.Document, paragraph string, chunkSize int, overlap int, startOffset int, chunkIndexOffset int) []*domain.DocumentChunk {
	var chunks []*domain.DocumentChunk
	paraLen := len(paragraph)

	// For very large paragraphs, split by sentences if possible
	sentences := strings.Split(paragraph, ". ")

	// If paragraph doesn't have clear sentences or has very few, use character chunking
	if len(sentences) < 3 || paraLen/len(sentences) > chunkSize {
		// Character-based chunking
		for i := 0; i < paraLen; i += (chunkSize - overlap) {
			end := i + chunkSize
			if end > paraLen {
				end = paraLen
			}

			// Try not to break words
			if end < paraLen {
				for end > i && paragraph[end] != ' ' && paragraph[end] != '\n' {
					end--
				}
			}

			chunkContent := paragraph[i:end]
			absoluteStart := startOffset + i
			absoluteEnd := absoluteStart + len(chunkContent)
			chunkIndex := chunkIndexOffset + len(chunks)

			chunk := domain.NewDocumentChunk(doc, chunkContent, absoluteStart, absoluteEnd, chunkIndex)
			chunks = append(chunks, chunk)
		}
	} else {
		// Sentence-based chunking for more semantic coherence
		var currentChunk strings.Builder
		currentSize := 0
		chunkIndex := chunkIndexOffset
		sentenceStartPos := startOffset

		for i, sentence := range sentences {
			sentenceSize := len(sentence) + 2 // +2 for ". "

			// If adding this sentence exceeds the chunk size and we have content
			if currentSize+sentenceSize > chunkSize && currentSize > 0 {
				chunkContent := currentChunk.String()
				absoluteEnd := sentenceStartPos + len(chunkContent)
				chunk := domain.NewDocumentChunk(doc, chunkContent, sentenceStartPos, absoluteEnd, chunkIndex)
				chunks = append(chunks, chunk)
				chunkIndex++

				// Calculate new start position
				sentenceStartPos = absoluteEnd
				currentChunk.Reset()
				currentSize = 0
			}

			// Add the sentence to the current chunk
			if currentSize > 0 {
				currentChunk.WriteString(". ")
			}
			currentChunk.WriteString(sentence)
			currentSize += sentenceSize

			// Handle the last sentence
			if i == len(sentences)-1 && currentSize > 0 {
				chunkContent := currentChunk.String()
				absoluteEnd := sentenceStartPos + len(chunkContent)
				chunk := domain.NewDocumentChunk(doc, chunkContent, sentenceStartPos, absoluteEnd, chunkIndex)
				chunks = append(chunks, chunk)
			}
		}
	}

	return chunks
}
