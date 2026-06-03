package assistant

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"github.com/acai-travel/tech-challenge/internal/chat/assistant/tool"
	"github.com/acai-travel/tech-challenge/internal/chat/model"
	"github.com/openai/openai-go/v2"
)

// Assistant handles AI conversations using OpenAI and a set of tools.
type Assistant struct {
	cli      openai.Client
	tools    []tool.Tool
	toolDefs []openai.ChatCompletionToolUnionParam
}

// New creates a new Assistant with all available tools registered.
func New() *Assistant {

	tools := []tool.Tool{
		&tool.DateTool{},
		&tool.WeatherTool{},
		&tool.HolidaysTool{},
	}

	// build tool definitions ONCE at startup
	toolDefs := make([]openai.ChatCompletionToolUnionParam, len(tools))
	for i, t := range tools {
		toolDefs[i] = t.Definition()
	}

	return &Assistant{
		cli:      openai.NewClient(),
		tools:    tools,
		toolDefs: toolDefs, // ← stored, reused every request
	}
}

// Title generates a short descriptive title for a conversation.
func (a *Assistant) Title(ctx context.Context, conv *model.Conversation) (string, error) {
	if len(conv.Messages) == 0 {
		return "An empty conversation", nil
	}

	slog.InfoContext(ctx, "Generating title for conversation", "conversation_id", conv.ID)

	// System message instructs AI to summarize, not answer
	msgs := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage("Generate a concise, descriptive title for the conversation based on the user message. The title should summarize the topic, not answer the question. Single line, max 80 characters, no special characters or emojis."),
	}

	for _, m := range conv.Messages {
		msgs = append(msgs, openai.UserMessage(m.Content))
	}

	resp, err := a.cli.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model:     openai.ChatModelGPT4oMini,
		Messages:  msgs,
		MaxTokens: openai.Int(20),
	})

	if err != nil {
		return "", err
	}

	if len(resp.Choices) == 0 || strings.TrimSpace(resp.Choices[0].Message.Content) == "" {
		return "", errors.New("empty response from OpenAI for title generation")
	}

	title := resp.Choices[0].Message.Content
	title = strings.ReplaceAll(title, "\n", " ")
	title = strings.Trim(title, " \t\r\n-\"'")

	if len(title) > 80 {
		title = title[:80]
	}

	return title, nil
}

func (a *Assistant) Reply(ctx context.Context, conv *model.Conversation) (string, error) {

	if len(conv.Messages) == 0 {
		return "", errors.New("conversation has no messages")
	}

	slog.InfoContext(ctx, "Generating reply for conversation", "conversation_id", conv.ID)

	msgs := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage("You are a helpful, concise AI assistant. Provide accurate, safe, and clear responses."),
	}
	for _, m := range conv.Messages {
		switch m.Role {
		case model.RoleUser:
			msgs = append(msgs, openai.UserMessage(m.Content))
		case model.RoleAssistant:
			msgs = append(msgs, openai.AssistantMessage(m.Content))
		}
	}

	// Tool call loop — AI may call tools multiple times before final answer
	for i := 0; i < 15; i++ {
		resp, err := a.cli.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
			Model:     openai.ChatModelGPT4_1,
			Messages:  msgs,
			Tools:     a.toolDefs,
			MaxTokens: openai.Int(500),
		})
		if err != nil {
			return "", err
		}

		if len(resp.Choices) == 0 {
			return "", errors.New("no choices returned by OpenAI")
		}

		message := resp.Choices[0].Message

		// No tool calls — this is the final answer
		if len(message.ToolCalls) == 0 {
			return message.Content, nil
		}

		// Process each tool call
		msgs = append(msgs, message.ToParam())
		for _, call := range message.ToolCalls {
			slog.InfoContext(ctx, "Tool call received", "name", call.Function.Name, "args", call.Function.Arguments)

			result := a.executeTool(ctx, call.Function.Name, call.Function.Arguments)
			msgs = append(msgs, openai.ToolMessage(result, call.ID))
		}
	}

	return "", errors.New("too many tool calls, unable to generate reply")
}

// executeTool finds the right tool by name and runs it.
// Returns the result string — errors are returned as strings so the AI can handle them gracefully.
func (a *Assistant) executeTool(ctx context.Context, name string, args string) string {
	for _, t := range a.tools {
		if t.Name() == name {
			result, err := t.Execute(ctx, args)
			if err != nil {
				slog.ErrorContext(ctx, "Tool execution failed", "tool", name, "error", err)
				return "tool error: " + err.Error()
			}
			return result
		}
	}
	slog.ErrorContext(ctx, "Unknown tool called", "tool", name)
	return "unknown tool: " + name
}
