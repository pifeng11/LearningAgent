package knowledge

type Chunk struct {
	ID       string
	Content  string
	Metadata map[string]string
}

type Chunker interface {
	Chunk(content string) ([]Chunk, error)
}
