package rag

import (
	"context"
	"crypto/md5"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// IngestFile loads a text file, chunks it, and stores it in the vector DB.
func IngestFile(ctx context.Context, db *DB, filePath string) error {
	slog.Info("Ingesting file", "path", filePath)

	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	// split into chunks
	chunks := chunkText(string(content), 200, 20)
	slog.Info("Split into chunks", "file", filePath, "chunks", len(chunks))

	fileName := filepath.Base(filePath)

	for i, chunk := range chunks {
		if strings.TrimSpace(chunk) == "" {
			continue
		}

		// deterministic ID — same file + chunk index = same ID
		// this means re-ingesting updates rather than duplicates
		id := fmt.Sprintf("%x", md5.Sum([]byte(fmt.Sprintf("%s_%d", filePath, i))))

		metadata := map[string]string{
			"source": fileName,
			"chunk":  fmt.Sprintf("%d", i),
		}

		if err := db.Add(ctx, id, chunk, metadata); err != nil {
			// skip if already exists — chromem-go returns error on duplicate ID
			if strings.Contains(err.Error(), "already exists") {
				slog.Info("Chunk already exists, skipping", "id", id)
				continue
			}
			return fmt.Errorf("failed to add chunk %d: %w", i, err)
		}

		slog.Info("Stored chunk", "file", fileName, "chunk", i, "id", id)
	}

	return nil
}

// chunkText splits text into overlapping chunks by word count.
func chunkText(text string, size int, overlap int) []string {
	words := strings.Fields(text)
	var chunks []string

	for i := 0; i < len(words); i += (size - overlap) {
		end := i + size
		if end > len(words) {
			end = len(words)
		}
		chunks = append(chunks, strings.Join(words[i:end], " "))
		if end == len(words) {
			break
		}
	}

	return chunks
}