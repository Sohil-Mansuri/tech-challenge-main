package rag

import (
	"context"
	"os"

	chromem "github.com/philippgille/chromem-go"
)

const collectionName = "acai-travel"

// DB wraps chromem-go and provides simple access to the vector database.
type DB struct {
	db         *chromem.DB
	collection *chromem.Collection
}

// NewDB creates or loads the vector database from disk.
// Data is persisted to the given directory so you can see the files.
func NewDB(ctx context.Context, persistDir string) (*DB, error) {
	// create persist directory if it doesn't exist
	if err := os.MkdirAll(persistDir, 0755); err != nil {
		return nil, err
	}

	// NewPersistentDB saves every document as a file in persistDir
	// you can open these files and see what's stored!
	chromemDB, err := chromem.NewPersistentDB(persistDir, false)
	if err != nil {
		return nil, err
	}

	// get or create collection — uses OpenAI embeddings automatically
	apiKey := os.Getenv("OPENAI_API_KEY")
	collection, err := chromemDB.GetOrCreateCollection(
		collectionName,
		nil,
		chromem.NewEmbeddingFuncOpenAI(apiKey, chromem.EmbeddingModelOpenAI3Small),
	)
	if err != nil {
		return nil, err
	}

	return &DB{
		db:         chromemDB,
		collection: collection,
	}, nil
}

// Add stores a document with its embedding in the vector DB.
func (d *DB) Add(ctx context.Context, id, content string, metadata map[string]string) error {
	return d.collection.AddDocument(ctx, chromem.Document{
		ID:       id,
		Content:  content,
		Metadata: metadata,
	})
}

// Search finds the most relevant documents for a query.
func (d *DB) Search(ctx context.Context, query string, topK int) ([]chromem.Result, error) {
	return d.collection.Query(ctx, query, topK, nil, nil)
}

// Count returns how many documents are stored.
func (d *DB) Count() int {
	return d.collection.Count()
}

func DBPath() string {
	if v := os.Getenv("VECTOR_DB_PATH"); v != "" {
		return v
	}
	return "./data/vectordb"
}
