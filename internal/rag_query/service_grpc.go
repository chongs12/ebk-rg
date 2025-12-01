package rag_query

import (
	"context"

	pb "github.com/chongs12/enterprise-knowledge-base/api/proto/vector"
)

// searchChunksViaGRPC uses gRPC to call vector service directly
func (s *RAGQueryService) searchChunksViaGRPC(ctx context.Context, query string, limit int) ([]gatewayChunk, error) {
	resp, err := s.vectorCli.Search(ctx, &pb.SearchRequest{
		Query: query,
		Limit: int32(limit),
	})
	if err != nil {
		return nil, err
	}

	chunks := make([]gatewayChunk, len(resp.Chunks))
	for i, c := range resp.Chunks {
		chunks[i] = gatewayChunk{
			ID:         c.Id,
			DocumentID: c.DocumentId,
			Content:    c.Content,
			ChunkIndex: int(c.ChunkIndex),
			StartPos:   int(c.StartPos),
			EndPos:     int(c.EndPos),
			WordCount:  int(c.WordCount),
		}
	}
	return chunks, nil
}
