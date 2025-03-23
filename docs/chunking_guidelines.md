# Optimization Guide for Chunking Strategies

This document provides guidelines to optimize chunking strategies based on different types of documents. Chunking is a crucial step in a RAG (Retrieval-Augmented Generation) system that directly impacts the quality of the responses.

## Table of Contents

1. [Introduction to Chunking](#introduction-to-chunking)  
2. [Available Chunking Strategies](#available-chunking-strategies)  
3. [Recommendations by Document Type](#recommendations-by-document-type)  
4. [Evaluation and Optimization](#evaluation-and-optimization)  
5. [Best Practices](#best-practices)

## Introduction to Chunking

Chunking is the process of dividing a document into smaller units (chunks) that are indexed and retrieved independently. The goal is to create chunks that:

- Contain enough context to be useful  
- Are small enough to be specific  
- Preserve semantic units (sentences, paragraphs)  
- Minimize redundancy while ensuring complete coverage  

Optimal chunking improves both retrieval accuracy and the quality of generated responses.

## Available Chunking Strategies

RLAMA offers several chunking strategies, each optimized for different types of content:

### 1. Fixed Chunking (`fixed`)

- **Description**: Splits the text into fixed-size chunks, trying not to cut words.  
- **Advantages**: Simple, predictable, works on all types of content.  
- **Disadvantages**: Does not respect semantic structure, may split sentences and paragraphs.  
- **Recommended for**: Unstructured documents, heterogeneous content.

### 2. Semantic Chunking (`semantic`)

- **Description**: Divides content by respecting natural boundaries like paragraphs and sections.  
- **Advantages**: Preserves semantic context, improves response quality.  
- **Disadvantages**: May produce chunks of highly variable size.  
- **Recommended for**: Articles, structured documents, user manuals.

### 3. Hybrid Chunking (`hybrid`)

- **Description**: Adapts the strategy based on the detected document type.  
- **Advantages**: Combines the strengths of other strategies.  
- **Disadvantages**: Increased complexity, may be less predictable.  
- **Recommended for**: Mixed corpora with various document types.

### 4. Hierarchical Chunking (`hierarchical`)

- **Description**: Creates a two-level structure with larger parent chunks and smaller child chunks.  
- **Advantages**: Captures both the big picture and finer details.  
- **Disadvantages**: More complex indexing, uses more storage.  
- **Recommended for**: Very long documents, books, full technical documentation.

## Recommendations by Document Type

### Markdown/Documentation Files

- **Recommended strategy**: `semantic` or `hybrid`  
- **Chunk size**: 1000–1500 characters (~250–375 tokens)  
- **Overlap**: 10% of chunk size  
- **Why**: Markdown documents generally have a clear structure (sections, subsections) that semantic chunking can leverage.

### Source Code

- **Recommended strategy**: `hybrid` (which uses code-aware chunking)  
- **Chunk size**: 500–1000 characters (~125–250 tokens)  
- **Overlap**: 5–10% of chunk size  
- **Why**: Code has defined structure (functions, classes) and code-aware chunking preserves these logical units.

### Long Texts/Articles

- **Recommended strategy**: `semantic` or `hierarchical`  
- **Chunk size**: 1500–2000 characters (~375–500 tokens)  
- **Overlap**: 15–20% of chunk size  
- **Why**: Long texts benefit from strategies that respect paragraphs and sections, with higher overlap to maintain context.

### HTML/Web Pages

- **Recommended strategy**: `hybrid` (which uses HTML-aware chunking)  
- **Chunk size**: 1000–1500 characters (~250–375 tokens)  
- **Overlap**: 10–15% of chunk size  
- **Why**: HTML content has structure defined by tags that specialized chunking can exploit.

### Unstructured Texts

- **Recommended strategy**: `fixed` or parameterized `semantic`  
- **Chunk size**: 800–1200 characters (~200–300 tokens)  
- **Overlap**: 20% of chunk size  
- **Why**: Without a clear structure, higher overlap helps preserve context.

## Evaluation and Optimization

RLAMA provides tools to evaluate and optimize your chunking strategies. The `chunk-eval` tool lets you test different configurations on your specific documents.

### Using the Evaluation Tool

```bash
# Evaluate a specific configuration
rlama chunk-eval --file=your_document.txt --strategy=semantic --size=1500 --overlap=150

# Compare all available strategies
rlama chunk-eval --file=your_document.txt --compare-all --detailed
```

### Evaluation Metrics

- **Semantic Coherence Score**: Overall quality of chunking (0–1, higher = better)  
- **Cut Sentences/Paragraphs**: Number of chunks that break semantic units (fewer = better)  
- **Redundancy Rate**: Percentage of duplicated content due to overlap  
- **Content Coverage**: Percentage of the original document covered by the chunks

### Recommended Optimization Process

1. Start by comparing all strategies on your corpus.  
2. Identify the top 2–3 strategies based on the metrics.  
3. Fine-tune parameters (size, overlap) for those strategies.  
4. Test optimized configurations on representative queries.  
5. Measure impact on final responses, not just the metrics.

## Best Practices

### General Tips

- **Adapt the strategy to the content**: There’s no universal configuration; tailor it to your documents.  
- **Favor semantic coherence**: Natural boundaries (paragraphs, sections) usually make better chunk breakpoints.  
- **Avoid overly small chunks**: Chunks under 100 tokens often lack context.  
- **Limit overly large chunks**: Chunks over 500 tokens may be too generic and hurt precision.  
- **Test with real queries**: Final impact is measured by response quality, not just metrics.

### What to Avoid

- **Splitting sentences mid-way**: This fragments information and leads to incoherent chunks.  
- **Ignoring document structure**: Using existing structure (headers, sections) generally improves results.  
- **Too much overlap**: Beyond 25%, redundancy can become more harmful than helpful.  
- **Highly variable chunk sizes**: Wide variation in size can bias retrieval.

---

## Conclusion

Optimizing chunking is an iterative process that requires testing and tuning. The recommendations provided here serve as a starting point, and the metrics generated by the evaluation tool will help you refine your approach for your specific use case.

For questions or suggestions, feel free to open an issue on the project’s GitHub repository.