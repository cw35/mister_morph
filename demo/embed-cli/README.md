# Demo: Embed `mistermorph` as a CLI subprocess

This shows how another Go project can call the `mistermorph` binary as a subprocess and parse its JSON output.

## Run

1) Build `mistermorph` (from repo root):

```bash
cd mistermorph
GOCACHE=/tmp/gocache GOPATH=/tmp/gopath GOMODCACHE=/tmp/gomodcache go build ./cmd/mistermorph
```

2) Run the demo (from `demo/embed-cli/`):

```bash
export MISTER_MORPH_BIN=../../mistermorph/mistermorph
export OPENAI_API_KEY="..."
GOCACHE=/tmp/gocache GOPATH=/tmp/gopath GOMODCACHE=/tmp/gomodcache \
  go run . --task "Search for OpenAI and fetch the first result"
```

Notes:
- Agent logs go to stderr; the final JSON output is captured from stdout and printed as pretty JSON.
