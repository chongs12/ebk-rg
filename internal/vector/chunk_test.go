package vector

import (
	"testing"
)

// TestChunkText_Basic 基础分块用例
// 用途：
// - 验证在小块大小（maxChars=3）情况下，英文字符序列的分块行为
// 说明：
// - 当前实现按字符（rune）计数进行分块，WordCount 表示字符数而非词数
func TestChunkText_Basic(t *testing.T) {
	svc := &VectorService{}
	text := "abcdefghij" // 10 个字符
	chunks, err := svc.ChunkText(t.Context(), "00000000-0000-0000-0000-000000000000", text, 3)
	if err != nil {
		t.Fatalf("chunk error: %v", err)
	}
	if len(chunks) != 4 {
		t.Fatalf("expect 4 chunks, got %d", len(chunks))
	}
	if chunks[0].ChunkIndex != 0 || chunks[3].ChunkIndex != 3 {
		t.Fatalf("chunk index incorrect")
	}
	if chunks[0].WordCount != 3 {
		t.Fatalf("wordcount (runes) incorrect: %d", chunks[0].WordCount)
	}
}
