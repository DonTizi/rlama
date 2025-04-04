
# RLAMA Agents and Crews Documentation

## Table of Contents

- [Overview](#overview)
- [Agents](#agents)
  - [Creating Agents](#creating-agents)
  - [Agent Roles](#agent-roles)
  - [Agent Tools](#agent-tools)
  - [Running Agents](#running-agents)
  - [JSON Output Format](#json-output-format)
- [Crews](#crews)
  - [Creating Crews](#creating-crews)
  - [Workflow Types](#workflow-types)
  - [Adding Agents to Crews](#adding-agents-to-crews)
  - [Defining Workflow Steps](#defining-workflow-steps)
  - [Running Crews](#running-crews)
- [RAG Integration](#rag-integration)
- [Command Reference](#command-reference)

## Overview

RLAMA provides a powerful agent and crew system that allows you to create specialized AI agents that can perform specific tasks. These agents can work individually or collaborate as part of a crew to solve complex problems. The system supports various roles, integrated tools, and customizable workflows.

## Agents

Agents are specialized AI instances that can perform specific tasks based on their role and configuration. Each agent uses a specific language model (e.g., gemma3:4b) and can optionally be linked to a RAG (Retrieval-Augmented Generation) system for enhanced knowledge.

### Creating Agents

Agents can be created using the CLI command:

```bash
rlama agent create <agent-name> --role <role> --model <model-name> [--rag <rag-name>] [--description "<description>"]
```

Example:
```bash
rlama agent create researcher-agent --role researcher --model gemma3:4b --rag rlama-docs --description "Research specialized agent for detailed information retrieval and analysis"
```

### Agent Roles

RLAMA supports several predefined roles for agents, each with specific capabilities and system prompts:

- **researcher**: Focuses on information retrieval and analysis. Automatically has RAG search capabilities.
- **writer**: Specialized in creating content for different platforms (LinkedIn, Twitter/X, etc.)
- **formatter**: Formats content using Markdown and other structured formats
- **coder**: Generates and analyzes code, with optional code execution tools
- **analyst**: Processes numerical data and provides analytical insights

Each role influences the agent's system prompt, tool availability, and JSON output format.

### Agent Tools

Agents can use various tools to extend their capabilities:

| Tool Type | Description | Auto-assigned to |
|-----------|-------------|-----------------|
| `rag_search` | Search information in the associated RAG system | researcher |
| `code_exec` | Execute code and return the result | coder |
| `calculator` | Perform mathematical calculations | analyst |
| `web_search` | Search for information on the web | - |
| `api_call` | Make API calls to external services | - |
| `file_manager` | Read and write files | - |

Tools are automatically assigned based on the agent's role or can be manually configured.

### Running Agents

To run an agent with a specific instruction:

```bash
rlama agent run <agent-id-or-name> "<instruction>"
```

Example:
```bash
rlama agent run researcher-agent "Find information about the latest features in Ollama"
```

### JSON Output Format

Agents produce structured JSON output based on their role. This format is enforced using Ollama's structured outputs feature.

Example formats:

**Researcher agent:**
```json
{
  "findings": [
    { "key_point": "Main discovery 1", "details": "Detailed explanation" },
    { "key_point": "Main discovery 2", "details": "Detailed explanation" }
  ],
  "summary": "Overall summary of findings"
}
```

**Writer agent (LinkedIn):**
```json
{
  "post": {
    "title": "Post title",
    "content": "Main post content"
  }
}
```

**Formatter agent:**
```json
{
  "markdown": "# Formatted content\n\nThis is formatted using markdown syntax..."
}
```

When using tools, responses include `tool_calls`:

```json
{
  "response": "I need to search for information",
  "tool_calls": [
    {
      "tool_name": "rag_search",
      "parameters": {
        "query": "latest Ollama features"
      }
    }
  ]
}
```

## Crews

Crews are groups of agents that collaborate to solve complex problems. They follow a defined workflow where each agent performs a specific step.

### Creating Crews

To create a crew:

```bash
rlama agent crew create <crew-name> --workflow <workflow-type> [--description "<description>"]
```

Example:
```bash
rlama agent crew create content-team --workflow sequential --description "Team for researching topics and creating social media content"
```

### Workflow Types

RLAMA supports different workflow types:

- **sequential**: Agents work one after another, with output from one agent potentially feeding into the next
- **parallel**: Agents work simultaneously (limited implementation in current version)
- **custom**: Custom workflow patterns (advanced usage)

### Adding Agents to Crews

To add an agent to a crew:

```bash
rlama agent crew add-agent <crew-name> <agent-name>
```

Example:
```bash
rlama agent crew add-agent content-team researcher-agent
rlama agent crew add-agent content-team writer-agent
rlama agent crew add-agent content-team formatter-agent
```

### Defining Workflow Steps

To define steps in a crew's workflow:

```bash
rlama agent crew add-step <crew-id> <agent-id> --instruction "<instruction>" [--depends-on <step-indices>] [--output-to-next]
```

Example for a sequential workflow:
```bash
# Step 0: Research
rlama agent crew add-step content-team researcher-agent --instruction "Research information about the topic and extract key points" --output-to-next

# Step 1: Write content
rlama agent crew add-step content-team writer-agent --instruction "Use the research to create a LinkedIn post and a tweet" --depends-on 0 --output-to-next

# Step 2: Format content
rlama agent crew add-step content-team formatter-agent --instruction "Format the content in Markdown with proper headings" --depends-on 1
```

### Running Crews

To run a crew with an instruction:

```bash
rlama agent crew run <crew-id> "<instruction>"
```

Example:
```bash
rlama agent crew run content-team "Explain the main features and benefits of RLAMA"
```

The system will execute each step according to the workflow definition:
1. The researcher agent will extract information from the RAG
2. The writer agent will create LinkedIn and Twitter content
3. The formatter agent will format it all in Markdown

## RAG Integration

Agents can leverage RAG systems to enhance their knowledge:

- When an agent is created with a `--rag` parameter, it can query that knowledge base
- In researcher agents, the RAG context is automatically retrieved and included
- Crews can also use RAG-enabled agents to provide information to other agents in the workflow

To link a RAG with an agent:
```bash
rlama agent create researcher-agent --role researcher --model gemma3:4b --rag rlama-docs
```

## Command Reference

### Agent Commands
- `rlama agent create <name> [options]` - Create a new agent
- `rlama agent list` - List all available agents
- `rlama agent delete <agent-id>` - Delete an agent
- `rlama agent run <agent-id> "<instruction>"` - Run an agent

### Crew Commands
- `rlama agent crew create <name> [options]` - Create a new crew
- `rlama agent crew add-agent <crew-name> <agent-name>` - Add an agent to a crew
- `rlama agent crew add-step <crew-id> <agent-id> [options]` - Add a workflow step
- `rlama agent crew run <crew-id> "<instruction>"` - Run a crew with an instruction

---

This documentation covers the core concepts and functionality of the RLAMA agent and crew system. For more advanced usage and customization options, refer to the full API documentation.
