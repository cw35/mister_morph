package llminspect

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/quailyquaily/mistermorph/llm"
)

func TestModelSceneContext(t *testing.T) {
	if got := ModelSceneFromContext(nil); got != defaultModelScene {
		t.Fatalf("scene for nil ctx = %q, want %q", got, defaultModelScene)
	}
	ctx := WithModelScene(context.Background(), " telegram.addressing_decision ")
	if got := ModelSceneFromContext(ctx); got != "telegram.addressing_decision" {
		t.Fatalf("scene = %q, want telegram.addressing_decision", got)
	}
}

func readSingleDumpFile(t *testing.T, path string) (string, string) {
	t.Helper()
	path = strings.TrimSpace(path)
	if path == "" {
		t.Fatalf("empty dump path")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		t.Fatalf("filepath.Abs() error = %v", err)
	}
	raw, err := os.ReadFile(abs)
	if err != nil {
		t.Fatalf("os.ReadFile() error = %v", err)
	}
	return abs, string(raw)
}

type staticChatClient struct{}

func (staticChatClient) Chat(ctx context.Context, req llm.Request) (llm.Result, error) {
	return llm.Result{Text: `{"type":"final","final":{"output":"ok"}}`}, nil
}
