package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/quailyquaily/mistermorph/agent"
	"github.com/quailyquaily/mistermorph/integration"
)

// ListDirTool is an example of a project-specific tool you provide to the agent.
// It intentionally stays simple for demo purposes.
type ListDirTool struct {
	Root string
}

func (t *ListDirTool) Name() string { return "list_dir" }

func (t *ListDirTool) Description() string {
	return "Lists files under a directory (relative to a configured root)."
}

func (t *ListDirTool) ParameterSchema() string {
	return `{
  "type": "object",
  "properties": {
    "path": {"type": "string", "description": "Relative path under the configured root (default: .)."}
  }
}`
}

func (t *ListDirTool) Execute(_ context.Context, params map[string]any) (string, error) {
	rel, _ := params["path"].(string)
	if rel == "" {
		rel = "."
	}
	root, err := filepath.Abs(t.Root)
	if err != nil {
		return "", err
	}
	p := filepath.Join(root, rel)
	p, err = filepath.Abs(p)
	if err != nil {
		return "", err
	}
	// Basic containment check.
	if len(p) < len(root) || p[:len(root)] != root {
		return "", fmt.Errorf("path escapes root")
	}

	entries, err := os.ReadDir(p)
	if err != nil {
		return "", err
	}
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() {
			name += "/"
		}
		out = append(out, name)
	}
	b, _ := json.MarshalIndent(map[string]any{
		"root":  root,
		"path":  rel,
		"files": out,
	}, "", "  ")
	return string(b), nil
}

type GetWeatherTool struct {
}

func (t *GetWeatherTool) Name() string { return "get_weather" }

func (t *GetWeatherTool) Description() string {
	return "Gets current weather for a city from a configured weather API endpoint."
}

func (t *GetWeatherTool) ParameterSchema() string {
	return `{
  "type": "object",
  "properties": {
    "city": {"type": "string", "description": "City name, e.g. San Francisco."}
  },
  "required": ["city"]
}`
}

func (t *GetWeatherTool) Execute(ctx context.Context, params map[string]any) (string, error) {
	_ = ctx
	city, _ := params["city"].(string)
	city = strings.TrimSpace(city)
	if city == "" {
		return "", fmt.Errorf("city is required")
	}
	out, _ := json.MarshalIndent(map[string]any{
		"source":         "mock_weather_tool",
		"city":           city,
		"condition":      "raining",
		"temperature_c":  18,
		"humidity_pct":   82,
		"updated_at_utc": "2026-02-13T09:00:00Z",
	}, "", "  ")
	return string(out), nil
}

func main() {
	var (
		task           = flag.String("task", "List files and summarize the project.", "Task to run.")
		model          = flag.String("model", "gpt-5.2", "Model name.")
		endpoint       = flag.String("endpoint", "https://api.openai.com", "OpenAI-compatible base URL.")
		apiKey         = flag.String("api-key", os.Getenv("OPENAI_API_KEY"), "API key (defaults to OPENAI_API_KEY).")
		inspectPrompt  = flag.Bool("inspect-prompt", false, "Dump prompts to ./dump.")
		inspectRequest = flag.Bool("inspect-request", false, "Dump request/response payloads to ./dump.")
	)
	flag.Parse()

	cfg := integration.DefaultConfig()
	cfg.Inspect.Prompt = *inspectPrompt
	cfg.Inspect.Request = *inspectRequest
	cfg.BuiltinToolNames = []string{"read_file", "url_fetch", "todo_update"}
	cfg.Set("llm.provider", "openai")
	cfg.Set("llm.endpoint", strings.TrimSpace(*endpoint))
	cfg.Set("llm.api_key", strings.TrimSpace(*apiKey))
	cfg.Set("llm.model", strings.TrimSpace(*model))
	cfg.Set("llm.request_timeout", 60*time.Second)
	cfg.Set("tools.url_fetch.timeout", 20*time.Second)
	cfg.Set("tools.todo.enabled", true)

	rt, err := integration.New(cfg)
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	reg := rt.NewRegistry()
	reg.Register(&ListDirTool{Root: "."})
	reg.Register(&GetWeatherTool{})

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	prepared, err := rt.NewRunEngineWithRegistry(ctx, *task, reg)
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	defer func() { _ = prepared.Cleanup() }()

	runModel := strings.TrimSpace(*model)
	if runModel == "" {
		runModel = strings.TrimSpace(prepared.Model)
	}
	final, _, err := prepared.Engine.Run(ctx, *task, agent.RunOptions{Model: runModel})
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(final)
}
