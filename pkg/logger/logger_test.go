package logger

import (
    "context"
    "testing"

    "github.com/sirupsen/logrus"
)

func TestWithFieldsCtx(t *testing.T) {
    ctx := context.WithValue(context.Background(), "request_id", "req-1")
    ctx = context.WithValue(ctx, "user_id", "u-1")
    Init()
    e := WithFieldsCtx(ctx, logrus.Fields{"event_type": "test"})
    if e.Data["request_id"] != "req-1" {
        t.Fatalf("missing request_id")
    }
    if e.Data["user_id"] != "u-1" {
        t.Fatalf("missing user_id")
    }
    if e.Data["event_type"] != "test" {
        t.Fatalf("missing event_type")
    }
}
