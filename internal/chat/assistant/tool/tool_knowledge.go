package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/acai-travel/tech-challenge/internal/rag"
	"github.com/openai/openai-go/v2"
)

// KnowledgeBaseTool searches Acai Travel's internal documents.
type KnowledgeBaseTool struct {
	DB *rag.DB
}

func (t *KnowledgeBaseTool) Name() string { return "search_knowledge_base" }

func (t *KnowledgeBaseTool) Definition() openai.ChatCompletionToolUnionParam {
	return openai.ChatCompletionFunctionTool(openai.FunctionDefinitionParam{
		Name:        "search_knowledge_base",
		Description: openai.String("Search Acai Travel's internal knowledge base for information about travel packages, pricing, policies, hotels and destinations. Use this for any questions about what Acai Travel offers or Rahul das details."),
		Parameters: openai.FunctionParameters{
			"type": "object",
			"properties": map[string]any{
				"query": map[string]string{
					"type":        "string",
					"description": "Search query e.g. Barcelona package price, cancellation policy, Japan itinerary, Rahul das details",
				},
			},
			"required": []string{"query"},
		},
	})
}

func (t *KnowledgeBaseTool) Execute(ctx context.Context, args string) (string, error) {
	var payload struct {
		Query string `json:"query"`
	}
	if err := json.Unmarshal([]byte(args), &payload); err != nil {
		return "", fmt.Errorf("failed to parse arguments: %w", err)
	}

	if t.DB.Count() == 0 {
		return "Knowledge base is empty. Please run the ingest command first.", nil
	}

	// search top 3 most relevant chunks
	results, err := t.DB.Search(ctx, payload.Query, 3)
	if err != nil {
		return "", fmt.Errorf("failed to search knowledge base: %w", err)
	}

	if len(results) == 0 {
		return "No relevant information found in the knowledge base.", nil
	}

	// format results for OpenAI
	var lines []string
	for i, result := range results {
		lines = append(lines, fmt.Sprintf(
			"[Source: %s, Relevance: %.0f%%]\n%s",
			result.Metadata["source"],
			result.Similarity*100,
			result.Content,
		))
		if i < len(results)-1 {
			lines = append(lines, "---")
		}
	}

	return strings.Join(lines, "\n"), nil
}
