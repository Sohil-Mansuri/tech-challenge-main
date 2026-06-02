package tool

import (
	"context"
	"time"

	"github.com/openai/openai-go/v2"
)

// DateTool returns the current date and time.
type DateTool struct{}

func (t *DateTool) Name() string { return "get_today_date" }

// Definition tells OpenAI about this tool — name, description, no parameters needed.
func (t *DateTool) Definition() openai.ChatCompletionToolUnionParam {
	return openai.ChatCompletionFunctionTool(openai.FunctionDefinitionParam{
		Name:        "get_today_date",
		Description: openai.String("Get today's date and time in RFC3339 format"),
	})
}

// Execute returns the current time — no args needed for this tool.
func (t *DateTool) Execute(ctx context.Context, args string) (string, error) {
	return time.Now().Format(time.RFC3339), nil
}
