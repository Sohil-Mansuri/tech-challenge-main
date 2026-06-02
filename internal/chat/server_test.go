package chat

import (
	"context"
	"errors"
	"testing"

	"github.com/acai-travel/tech-challenge/internal/chat/model"
	. "github.com/acai-travel/tech-challenge/internal/chat/testing"
	"github.com/acai-travel/tech-challenge/internal/pb"
	"github.com/google/go-cmp/cmp"
	"github.com/twitchtv/twirp"
	"google.golang.org/protobuf/testing/protocmp"
)

func TestServer_DescribeConversation(t *testing.T) {
	ctx := context.Background()
	srv := NewServer(model.New(ConnectMongo()), nil)

	t.Run("describe existing conversation", WithFixture(func(t *testing.T, f *Fixture) {
		c := f.CreateConversation()

		out, err := srv.DescribeConversation(ctx, &pb.DescribeConversationRequest{ConversationId: c.ID.Hex()})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		got, want := out.GetConversation(), c.Proto()
		if !cmp.Equal(got, want, protocmp.Transform()) {
			t.Errorf("DescribeConversation() mismatch (-got +want):\n%s", cmp.Diff(got, want, protocmp.Transform()))
		}
	}))

	t.Run("describe non existing conversation should return 404", WithFixture(func(t *testing.T, f *Fixture) {
		_, err := srv.DescribeConversation(ctx, &pb.DescribeConversationRequest{ConversationId: "08a59244257c872c5943e2a2"})
		if err == nil {
			t.Fatal("expected error for non-existing conversation, got nil")
		}

		if te, ok := err.(twirp.Error); !ok || te.Code() != twirp.NotFound {
			t.Fatalf("expected twirp.NotFound error, got %v", err)
		}
	}))
}

func TestServer_StartConversation(t *testing.T) {
	ctx := context.Background()

	t.Run("creates a new conversation", WithFixture(func(t *testing.T, f *Fixture) {
		srv := NewServer(f.Repository, &MockAssistant{})

		out, err := srv.StartConversation(ctx, &pb.StartConversationRequest{
			Message: "Hello!",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if out.GetConversationId() == "" {
			t.Error("expected conversation_id to be set, got empty string")
		}

		saved, err := f.Repository.DescribeConversation(ctx, out.GetConversationId())
		if err != nil {
			t.Fatalf("conversation not found in database: %v", err)
		}

		f.Repository.DeleteConversation(ctx, saved.ID.Hex())
	}))

	t.Run("populates the title", WithFixture(func(t *testing.T, f *Fixture) {
		srv := NewServer(f.Repository, &MockAssistant{
			TitleFunc: func(ctx context.Context, conv *model.Conversation) (string, error) {
				return "Weather in Barcelona", nil
			},
		})

		out, err := srv.StartConversation(ctx, &pb.StartConversationRequest{
			Message: "What is the weather in Barcelona?",
		})

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if out.GetTitle() != "Weather in Barcelona" {
			t.Errorf("expected title %q, got %q", "Weather in Barcelona", out.GetTitle())
		}

		saved, err := f.Repository.DescribeConversation(ctx, out.GetConversationId())
		if err != nil {
			t.Fatalf("conversation not found in database: %v", err)
		}

		if saved.Title != "Weather in Barcelona" {
			t.Errorf("expected saved title %q, got %q", "Weather in Barcelona", saved.Title)
		}

		f.Repository.DeleteConversation(ctx, saved.ID.Hex())
	}))

	t.Run("triggers the assistant reply", WithFixture(func(t *testing.T, f *Fixture) {
		srv := NewServer(f.Repository, &MockAssistant{
			ReplyFunc: func(ctx context.Context, conv *model.Conversation) (string, error) {
				return "I am your assistant!", nil
			},
		})

		out, err := srv.StartConversation(ctx, &pb.StartConversationRequest{
			Message: "Who are you?",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// reply in response must match what assistant returned
		if out.GetReply() != "I am your assistant!" {
			t.Errorf("expected reply %q, got %q", "I am your assistant!", out.GetReply())
		}

		// reply must be saved as the last message in MongoDB
		saved, err := f.Repository.DescribeConversation(ctx, out.GetConversationId())
		if err != nil {
			t.Fatalf("conversation not found in database: %v", err)
		}

		lastMsg := saved.Messages[len(saved.Messages)-1]
		if lastMsg.Role != model.RoleAssistant {
			t.Errorf("expected last message role %q, got %q", model.RoleAssistant, lastMsg.Role)
		}
		if lastMsg.Content != "I am your assistant!" {
			t.Errorf("expected last message content %q, got %q", "I am your assistant!", lastMsg.Content)
		}
	}))

	t.Run("uses fallback title when assistant title fails", WithFixture(func(t *testing.T, f *Fixture) {
		srv := NewServer(f.Repository, &MockAssistant{
			TitleFunc: func(ctx context.Context, conv *model.Conversation) (string, error) {
				return "", errors.New("openai is down")
			},
		})

		out, err := srv.StartConversation(ctx, &pb.StartConversationRequest{
			Message: "Hello!",
		})

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// must fall back to the default title
		if out.GetTitle() != "Untitled conversation" {
			t.Errorf("expected fallback title %q, got %q", "Untitled conversation", out.GetTitle())
		}
	}))

	t.Run("returns error when message is empty", WithFixture(func(t *testing.T, f *Fixture) {
		srv := NewServer(f.Repository, &MockAssistant{})

		_, err := srv.StartConversation(ctx, &pb.StartConversationRequest{
			Message: "",
		})

		if err == nil {
			t.Fatal("expected error for empty message, got nil")
		}

		if te, ok := err.(twirp.Error); !ok || te.Code() != twirp.Malformed {
			t.Fatalf("expected twirp.Malformed error, got %v", err)
		}
	}))
}
