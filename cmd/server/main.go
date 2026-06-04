package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/acai-travel/tech-challenge/internal/chat"
	"github.com/acai-travel/tech-challenge/internal/chat/assistant"
	"github.com/acai-travel/tech-challenge/internal/chat/model"
	"github.com/acai-travel/tech-challenge/internal/httpx"
	"github.com/acai-travel/tech-challenge/internal/mongox"
	"github.com/acai-travel/tech-challenge/internal/otelx"
	"github.com/acai-travel/tech-challenge/internal/pb"
	"github.com/gorilla/mux"
	"github.com/twitchtv/twirp"
)

func main() {

	ctx := context.Background()

	shutdown, err := otelx.Setup(ctx)
	if err != nil {
		panic(fmt.Sprintf("failed to initialise OpenTelemetry: %v", err))
	}
	defer func() {
		if err := shutdown(ctx); err != nil {
			slog.Error("OpenTelemetry shutdown error", "error", err)
		}
	}()

	mongo := mongox.MustConnect()

	repo := model.New(mongo)
	assist := assistant.New()

	server := chat.NewServer(repo, assist)

	// Configure handler
	handler := mux.NewRouter()
	handler.Use(
		httpx.RateLimiter(2, 5),
		httpx.Tracing(),
		httpx.Metrics(),
		httpx.Logger(),
		httpx.Recovery(),
	)

	handler.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, "Hi, my name is Clippy!")
	})

	handler.PathPrefix("/twirp/").Handler(pb.NewChatServiceServer(server, twirp.WithServerJSONSkipDefaults(true)))

	// Start the server
	slog.Info("Starting the server...")
	if err := http.ListenAndServe(":8080", handler); err != nil {
		panic(err)
	}
}
