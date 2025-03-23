# RLAMA Reranking Documentation

## Overview
Reranking in RLAMA is a feature that improves retrieval accuracy by applying a second-stage ranking to initial search results using a cross-encoder approach. This helps ensure more relevant documents are prioritized in responses to queries.

## Features
- Enabled by default for all RAG systems
- Configurable weights between vector similarity and reranking scores
- Adjustable result limits and thresholds
- Custom model support for reranking

## Default Configuration
- TopK: 5 results (maximum number of results after reranking)
- Initial retrieval: 20 documents
- Reranker weight: 0.7 (70% reranker score, 30% vector similarity)
- Score threshold: 0.0 (no minimum score requirement)
- Model: Uses the same model as the RAG system by default

## Usage

### Command Line Interface

1. **Configure Reranking for a RAG System**
```bash
rlama add-reranker my-rag [options]
```

Available options:
- `--model`: Specify a custom model for reranking (defaults to RAG model)
- `--weight`: Set the weight for reranker scores (0-1)
- `--threshold`: Set minimum score threshold for results
- `--topk`: Set maximum number of results to return
- `--disable`: Disable reranking for this RAG

Examples:
```bash
# Configure with custom model
rlama add-reranker my-rag --model reranker-model

# Adjust weights and limits
rlama add-reranker my-rag --weight 0.8 --topk 10

# Disable reranking
rlama add-reranker my-rag --disable
```

### Programmatic Usage

1. **Creating a RAG with Reranking**
```go
err := ragService.CreateRagWithOptions("llama3.2", "my-rag", documentPath, service.DocumentLoaderOptions{
    ChunkSize: 200,
    ChunkOverlap: 50,
    EnableReranker: true,  // Reranking is enabled by default
})
```

2. **Customizing Reranking Options**
```go
options := service.RerankerOptions{
    TopK: 10,                // Return top 10 results
    InitialK: 30,           // Retrieve 30 initial results
    RerankerModel: "custom-model",  // Use custom model
    ScoreThreshold: 0.5,    // Minimum relevance score
    RerankerWeight: 0.8,    // 80% reranker, 20% vector similarity
}
```

## How It Works

1. **Initial Retrieval**: The system first retrieves an initial set of documents using vector similarity search (default: top 20 documents).

2. **Reranking Process**:
   - Each retrieved document is evaluated using a cross-encoder model
   - The model scores document relevance on a scale of 0 to 1
   - Final scores combine vector similarity and reranking scores based on weights
   - Results are sorted by final score and limited to TopK

3. **Scoring Formula**:
finalScore = (rerankerWeight × rerankerScore) + ((1 - rerankerWeight) × vectorScore) 

## Performance Considerations

- Reranking adds additional processing time as each document needs to be evaluated
- The InitialK parameter affects both accuracy and performance
- Larger TopK values increase processing time
- Consider disabling reranking for applications requiring minimal latency

## Best Practices

1. **Model Selection**
   - Use the same model as your RAG system for consistency
   - Choose models that excel at cross-encoding for better results

2. **Parameter Tuning**
   - Start with default weights (0.7) and adjust based on results
   - Increase InitialK for better recall at the cost of performance
   - Set appropriate thresholds based on your use case

3. **Performance Optimization**
   - Limit TopK to necessary minimum
   - Consider chunk size impact on reranking performance
   - Monitor and adjust InitialK based on result quality

## Troubleshooting

Common issues and solutions:

1. **Slow Response Times**
   - Reduce InitialK or TopK values
   - Consider using a lighter reranking model
   - Check if chunk sizes are appropriate

2. **Poor Result Quality**
   - Increase reranker weight
   - Adjust score threshold
   - Increase InitialK for more candidate documents

3. **Model Compatibility**
   - Ensure the reranking model supports the required operations
   - Check model availability in your Ollama installation

## Advanced Configuration

For specific use cases, you can fine-tune the reranking system by:

1. **Custom Scoring**
   - Adjust weights based on document types
   - Implement custom thresholds for different queries

2. **Model Chaining**
   - Use different models for initial retrieval and reranking
   - Combine multiple reranking passes with different models

## Examples

1. **Basic Usage with Default Settings**
```bash
rlama add-reranker my-documents
```

2. **High-Precision Configuration**
```bash
rlama add-reranker research-papers --weight 0.9 --threshold 0.7 --topk 3
```

3. **Large-Scale Configuration**
```bash
rlama add-reranker large-corpus --topk 20 --weight 0.6
```
```

This README provides a comprehensive guide to understanding and using RLAMA's reranking functionality, based on the implementation shown in the provided code files.