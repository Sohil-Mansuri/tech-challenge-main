package assistant_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/acai-travel/tech-challenge/internal/chat/assistant"
	"github.com/acai-travel/tech-challenge/internal/chat/model"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func makeConversation(message string) *model.Conversation {
	return &model.Conversation{
		ID: primitive.NewObjectID(),
		Messages: []*model.Message{{
			ID:        primitive.NewObjectID(),
			Role:      model.RoleUser,
			Content:   message,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}},
	}
}

func skipIfNoAPIKey(t *testing.T) {
	t.Helper()
	if os.Getenv("OPENAI_API_KEY") == "" {
		t.Skip("skipping: OPENAI_API_KEY is not set")
	}
}

func TestAssistant_Title(t *testing.T) {
	skipIfNoAPIKey(t)

	ctx := context.Background()
	a := assistant.New()

	t.Run("returns a non-empty title", func(t *testing.T) {
		conv := makeConversation("What is the weather like in Barcelona?")

		title, err := a.Title(ctx, conv)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if strings.TrimSpace(title) == "" {
			t.Error("expected non-empty title, got empty string")
		}
	})

	t.Run("title is no longer than 80 characters", func(t *testing.T) {
		conv := makeConversation("What is the weather like in Barcelona?")

		title, err := a.Title(ctx, conv)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(title) > 80 {
			t.Errorf("expected title to be max 80 characters, got %d: %q", len(title), title)
		}
	})

	t.Run("title does not contain newlines", func(t *testing.T) {
		conv := makeConversation("Tell me everything about the history of Spain")

		title, err := a.Title(ctx, conv)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if strings.Contains(title, "\n") {
			t.Errorf("expected title without newlines, got: %q", title)
		}
	})

	t.Run("returns default title for empty conversation", func(t *testing.T) {
		// No OpenAI call needed — early return in Title()
		conv := &model.Conversation{
			ID:       primitive.NewObjectID(),
			Messages: []*model.Message{}, // empty
		}

		title, err := a.Title(ctx, conv)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if title != "An empty conversation" {
			t.Errorf("expected %q, got %q", "An empty conversation", title)
		}
	})
}
