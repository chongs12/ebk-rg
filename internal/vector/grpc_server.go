package vector

import (
	"context"

	pb "github.com/chongs12/enterprise-knowledge-base/api/proto/vector"
	"github.com/chongs12/enterprise-knowledge-base/internal/common/models"
)

type GRPCServer struct {
	pb.UnimplementedVectorServiceServer
	service *VectorService
}

func NewGRPCServer(service *VectorService) *GRPCServer {
	return &GRPCServer{service: service}
}

func (s *GRPCServer) Search(ctx context.Context, req *pb.SearchRequest) (*pb.SearchResponse, error) {
	// Convert gRPC request to service request (if needed, here just arguments)
	chunks, err := s.service.Search(ctx, req.Query, int(req.Limit))
	if err != nil {
		return nil, err
	}

	// Convert chunks to protobuf response
	respChunks := make([]*pb.Chunk, len(chunks))
	for i, c := range chunks {
		respChunks[i] = convertChunkToProto(c)
	}

	return &pb.SearchResponse{Chunks: respChunks}, nil
}

func convertChunkToProto(c models.TextChunkWithDistance) *pb.Chunk {
	return &pb.Chunk{
		Id:         c.ID.String(),
		DocumentId: c.DocumentID.String(),
		Content:    c.Content,
		ChunkIndex: int32(c.ChunkIndex),
		StartPos:   int32(c.StartPos),
		EndPos:     int32(c.EndPos),
		WordCount:  int32(c.WordCount),
		Score:      c.Distance,
	}
}
