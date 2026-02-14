# Demo: Embed `mistermorph` as a Go library

This shows how another Go project can import `mistermorph/integration` and run the agent engine in-process, with built-in wiring plus project-specific tools.

## Run

From `demo/embed-go/`:

```bash
export OPENAI_API_KEY="..."
GOCACHE=/tmp/gocache GOPATH=/tmp/gopath GOMODCACHE=/tmp/gomodcache \
  go run . --task "List files in the current directory and summarize what this project is." --model gpt-5.2
```

Notes:
- This demo uses the OpenAI-compatible provider, so it needs network access to actually run.
- It supports `--inspect-prompt` and `--inspect-request`.
- Built-in tools are enabled by default; the demo selects a subset in code via `cfg.BuiltinToolNames`.
- Final JSON goes to stdout.
