package tool

import (
	"context"

	"github.com/openai/openai-go/v2"
)

type Tool interface {
	Name() string
	Definition() openai.ChatCompletionToolUnionParam
	Execute(ctx context.Context, args string) (string, error)
}
