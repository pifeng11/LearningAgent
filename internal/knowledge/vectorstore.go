package knowledge

import "context"

type VectorStore interface {
	Upsert(ctx context.Context, chunks []Chunk) error
	Search(ctx context.Context, query string, limit int) ([]Chunk, error)
}
