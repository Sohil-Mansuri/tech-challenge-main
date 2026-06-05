package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/acai-travel/tech-challenge/internal/rag"
)

func main() {
	// flags
	docsDir := flag.String("docs", "./data/docs", "directory containing documents to ingest")
	dbDir := flag.String("db", "./data/vectordb", "directory to store vector DB files")
	flag.Parse()

	ctx := context.Background()

	// connect to vector DB
	db, err := rag.NewDB(ctx, *dbDir)
	if err != nil {
		slog.Error("Failed to connect to vector DB", "error", err)
		os.Exit(1)
	}

	slog.Info("Vector DB loaded", "documents", db.Count())

	// find all .txt files in docs directory
	files, err := filepath.Glob(filepath.Join(*docsDir, "*.txt"))
	if err != nil || len(files) == 0 {
		slog.Error("No .txt files found in docs directory", "dir", *docsDir)
		os.Exit(1)
	}

	// ingest each file
	for _, file := range files {
		if err := rag.IngestFile(ctx, db, file); err != nil {
			slog.Error("Failed to ingest file", "file", file, "error", err)
			continue
		}
	}

	slog.Info("Ingestion complete!", "total_documents", db.Count())
	fmt.Printf("\nVector DB files saved to: %s\n", *dbDir)
	fmt.Println("You can open these files to see how data is stored!")
}
