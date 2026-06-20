package knowledge

import "context"

type Retriever interface {
	Search(ctx context.Context, query string, limit int) ([]Chunk, error)
}
