package vector

import (
    "testing"
)

func TestChunkText_Basic(t *testing.T) {
    svc := &VectorService{}
    chunks, err := svc.ChunkText(t.Context(), "doc-1", "a b c d e f g h i j", 3)
    if err != nil { t.Fatalf("chunk error: %v", err) }
    if len(chunks) != 4 { t.Fatalf("expect 4 chunks, got %d", len(chunks)) }
    if chunks[0].ChunkIndex != 0 || chunks[3].ChunkIndex != 3 { t.Fatalf("chunk index incorrect") }
    if chunks[0].WordCount != 3 { t.Fatalf("wordcount incorrect") }
}