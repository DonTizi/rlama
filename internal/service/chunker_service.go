package service

import (
	"fmt"
	"strings"
	"github.com/dontizi/rlama/internal/domain"
)

// ChunkingConfig holds configuration for the document chunking process
type ChunkingConfig struct {
	ChunkSize      int  // Target size of each chunk in characters
	ChunkOverlap   int  // Number of characters to overlap between chunks
	IncludeMetadata bool // Whether to include metadata in chunk content
}

// DefaultChunkingConfig returns a default configuration for chunking
func DefaultChunkingConfig() ChunkingConfig {
	return ChunkingConfig{
		ChunkSize:      1500,    // Smaller chunks (~375 tokens) for better retrieval
		ChunkOverlap:   150,     // 10% overlap
		IncludeMetadata: true,
	}
}

// ChunkerService handles splitting documents into manageable chunks
type ChunkerService struct {
	ChunkSize    int
	ChunkOverlap int
	config ChunkingConfig
}

// NewChunkerService creates a new chunker service with the specified config
func NewChunkerService(chunkSize, chunkOverlap int) *ChunkerService {
	// Use provided parameters or defaults
	if chunkSize <= 0 {
		chunkSize = 1500  // Default
	}
	if chunkOverlap <= 0 {
		chunkOverlap = 150  // Default
	}
	
	return &ChunkerService{
		ChunkSize:    chunkSize,
		ChunkOverlap: chunkOverlap,
		config:       DefaultChunkingConfig(),
	}
}

// ChunkDocument splits a document into smaller chunks with metadata
func (cs *ChunkerService) ChunkDocument(doc *domain.Document) []*domain.DocumentChunk {
	content := doc.Content
	chunkSize := cs.ChunkSize
	overlap := cs.ChunkOverlap
	
	// For very small documents, just return a single chunk
	if len(content) <= chunkSize {
		chunk := domain.NewDocumentChunk(doc, content, 0, len(content), 0)
		return []*domain.DocumentChunk{chunk}
	}
	
	// For large documents, ensure we create multiple chunks
	// This is a safeguard against the bug that was creating only one chunk
	if len(content) > 10000 {
		return cs.createFixedSizeChunks(doc, content, chunkSize, overlap)
	}
	
	// New settings for small LLMs
	smallerChunkSize := 800 // Reduce from current 1500
	increasedOverlap := 200 // Increase from current 150
	
	// More sentence-aware chunking to maintain coherence
	chunks := cs.createParagraphBasedChunks(doc, content, smallerChunkSize, increasedOverlap)
	
	// Safeguard: If we still only got 1 chunk for a large document, force fixed-size chunking
	if len(chunks) == 1 && len(content) > smallerChunkSize*2 {
		return cs.createFixedSizeChunks(doc, content, smallerChunkSize, increasedOverlap)
	}
	
	fmt.Printf("Split document '%s' into %d chunks\n", doc.Name, len(chunks))
	return chunks
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
		if currentSize + paraSize > chunkSize && currentSize > 0 {
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
			if currentSize + sentenceSize > chunkSize && currentSize > 0 {
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