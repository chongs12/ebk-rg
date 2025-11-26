package vector

import (
	"context"
	"os"
	"testing"

	milvus "github.com/milvus-io/milvus-sdk-go/v2/client"
)

func TestMilvusStore_Init_SkipIfNoEnv(t *testing.T) {
	addr := os.Getenv("MILVUS_ADDR")
	api := os.Getenv("ARK_API_KEY")
	model := os.Getenv("ARK_MODEL")
	if addr == "" || api == "" || model == "" {
		t.Skip("skip integration: missing MILVUS_ADDR/ARK_API_KEY/ARK_MODEL")
	}
	cli, err := milvus.NewClient(context.Background(), milvus.Config{Address: addr})
	if err != nil {
		t.Fatalf("milvus client: %v", err)
	}
	defer cli.Close()
	// _, err = NewMilvusStore(context.Background(), cli, os.Getenv("MILVUS_COLLECTION"), api, model, os.Getenv("ARK_BASE_URL"), os.Getenv("ARK_REGION"), 1024, "Cosine")
	// if err != nil { t.Fatalf("milvus store: %v", err) }
}
