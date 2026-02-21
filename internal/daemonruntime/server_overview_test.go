package daemonruntime

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOverviewAddsVersionAndRuntimeWhenMissing(t *testing.T) {
	mux := http.NewServeMux()
	RegisterRoutes(mux, RoutesOptions{
		Mode:      "serve",
		AuthToken: "token",
		Overview: func(context.Context) (map[string]any, error) {
			return map[string]any{
				"llm": map[string]any{"provider": "openai"},
			}, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/overview", nil)
	req.Header.Set("Authorization", "Bearer token")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d (%s)", rec.Code, rec.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("invalid json: %v", err)
	}

	version, _ := payload["version"].(string)
	if strings.TrimSpace(version) == "" {
		t.Fatalf("expected non-empty version, got %v", payload["version"])
	}

	runtimePayload, ok := payload["runtime"].(map[string]any)
	if !ok || runtimePayload == nil {
		t.Fatalf("expected runtime object, got %T", payload["runtime"])
	}
	for _, key := range []string{"go_version", "goroutines", "heap_alloc_bytes", "heap_sys_bytes", "heap_objects", "gc_cycles"} {
		if _, exists := runtimePayload[key]; !exists {
			t.Fatalf("expected runtime.%s in payload", key)
		}
	}
}

func TestOverviewPreservesProvidedVersionAndRuntime(t *testing.T) {
	mux := http.NewServeMux()
	RegisterRoutes(mux, RoutesOptions{
		Mode:      "serve",
		AuthToken: "token",
		Overview: func(context.Context) (map[string]any, error) {
			return map[string]any{
				"version": "custom-version",
				"runtime": map[string]any{
					"go_version": "custom-go",
				},
			}, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/overview", nil)
	req.Header.Set("Authorization", "Bearer token")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d (%s)", rec.Code, rec.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("invalid json: %v", err)
	}

	if got, _ := payload["version"].(string); got != "custom-version" {
		t.Fatalf("expected version custom-version, got %v", payload["version"])
	}

	runtimePayload, ok := payload["runtime"].(map[string]any)
	if !ok || runtimePayload == nil {
		t.Fatalf("expected runtime object, got %T", payload["runtime"])
	}
	if got, _ := runtimePayload["go_version"].(string); got != "custom-go" {
		t.Fatalf("expected runtime.go_version custom-go, got %v", runtimePayload["go_version"])
	}
	if _, exists := runtimePayload["goroutines"]; !exists {
		t.Fatalf("expected runtime.goroutines to be backfilled")
	}
}
