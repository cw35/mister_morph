# MisterMorph Architecture

## 1. System Architecture

```text
                           +-------------------------+
                           |      User Surface       |
                           | CLI / Telegram / Slack  |
                           +------------+------------+
                                        |
                   +--------------------+--------------------+
                   |                                         |
         +---------v---------+                     +---------v---------+
         | CLI bootstrap     |                     | integration API   |
         | cmd/mistermorph/* |                     | integration/*     |
         +---------+---------+                     +---------+---------+
                   |                                         |
                   +--------------------+--------------------+
                                        |
                           +------------v------------+
                           | Runtime Assembly Layer  |
                           | config snapshot + deps  |
                           +------------+------------+
                                        |
                +-----------------------+-----------------------+
                |                                               |
      +---------v----------+                         +----------v---------+
      | One-shot runtime   |                         | Channel runtime    |
      | run / serve        |                         | telegram / slack   |
      +---------+----------+                         +----------+---------+
                |                                               |
                +-----------------------+-----------------------+
                                        |
                               +--------v--------+
                               |   agent.Engine  |
                               +---+---------+---+
                                   |         |
                          +--------v--+   +--v--------+
                          | llm.Client|   | tools.Reg |
                          +-----+-----+   +-----+-----+
                                |               |
                          +-----v-----+   +-----v------------------+
                          | providers |   | builtin/tools/adapters |
                          +-----------+   +------------------------+

Cross-cutting: guard, skills/prompt blocks, inspect dump, bus idempotency, file_state_dir
```

## 2. Main Execution Flow

```text
task/event
  -> build prompt/messages/meta
  -> agent.Engine.Run
     -> step loop
        -> LLM call
        -> parse (plan | tool_call | final)
        -> optional tool execute
        -> update plan/history/metrics
     -> final output (guard redact if needed)
```

This flow is implemented by `agent/engine.go` and `agent/engine_loop.go` and is shared across all entrypoints.

## 3. Two Runtime Families

### 3.1 One-shot (`run` / `serve`)

```text
CLI command -> config/registry/guard setup -> agent.Engine.Run -> output/json
```

- Entrypoints: `cmd/mistermorph/runcmd/run.go`, `cmd/mistermorph/daemoncmd/serve.go`
- Characteristics: single task execution or queued execution; no platform event consumer loop

### 3.2 Channel (Telegram / Slack)

```text
platform event
  -> inbound adapter
  -> inproc bus
  -> per-conversation worker (serial)
  -> run*Task -> agent.Engine
  -> outbound publish
  -> delivery adapter
  -> platform send
```

- Telegram: `internal/channelruntime/telegram/*`
- Slack: `internal/channelruntime/slack/*`

## 4. Existing Topic Docs (Links Only)

The following areas already have formal docs, so this file only links them:

- Prompt system: [`./prompt.md`](./prompt.md)
- Tools system: [`./tools.md`](./tools.md)
- Security / Guard: [`./security.md`](./security.md)
- Skills system: [`./skills.md`](./skills.md)
- MAEP protocol and implementation: [`./maep.md`](./maep.md), [`./maep_impl.md`](./maep_impl.md)
- Slack Socket Mode: [`./slack.md`](./slack.md)
- Bus design and implementation: [`./bus.md`](./bus.md), [`./bus_impl.md`](./bus_impl.md)

## 5. Key Areas Without Standalone Docs

### 5.1 Integration Embedding Layer

```text
host app
  -> integration.DefaultConfig + Set(...)
  -> rt := integration.New(cfg)      // snapshot at init
  -> rt.RunTask(...)                 // one-shot
  -> rt.NewTelegramBot/NewSlackBot   // long-running
```

Notes:

- `integration` is the third-party reuse entrypoint; host apps do not need to depend on CLI command wiring.
- `integration.New(cfg)` builds a snapshot of effective runtime config at initialization time.
- Code: `integration/runtime.go`, `integration/runtime_snapshot*.go`, `integration/channel_bots.go`

### 5.2 Telegram Runtime (no standalone doc yet)

```text
getUpdates
  -> command/filter/trigger decision
  -> enqueue chat worker
  -> runTelegramTask
  -> bus outbound
  -> telegram deliver
```

Notes:

- Telegram currently has the richest runtime logic: commands, group trigger routing, sticky skills, chat history, memory updates, and optional MAEP collaboration.
- Code: `internal/channelruntime/telegram/runtime_loop.go`, `internal/channelruntime/telegram/runtime_task.go`

### 5.3 Memory Status

```text
telegram private user
  -> resolve identity (ext:telegram:<user_id>)
  -> load/inject summaries
  -> update short-term + long-term markdown
```

Notes:

- Runtime-level memory integration is currently wired for Telegram only; Slack memory integration is not yet wired.
- Storage model lives in `memory/*`, runtime integration is in `internal/channelruntime/telegram/runtime_task.go`.

## 6. State Directory and Naming Baseline

```text
file_state_dir (default ~/.morph)
├── contacts/
│   ├── ACTIVE.md
│   ├── INACTIVE.md
│   ├── bus_inbox.json
│   └── bus_outbox.json
├── maep/
│   ├── identity.json
│   ├── contacts.json
│   ├── inbox_messages.jsonl
│   ├── outbox_messages.jsonl
│   ├── audit_events.jsonl
│   ├── dedupe_records.json
│   └── protocol_history.json
├── memory/
│   ├── index.md
│   └── YYYY-MM-DD/<sanitized-session-id>.md
├── guard/
│   ├── audit/guard_audit.jsonl
│   └── approvals/guard_approvals.json
└── skills/<skill>/SKILL.md
```

Additional notes:

- Memory short-term filenames come from sanitized `session_id` values (letters, digits, `-`, `_`).
- Contacts bus dedupe keys:
  - inbox: `(channel, platform_message_id)`
  - outbox: `(channel, idempotency_key)`

## 7. Code Navigation

Recommended reading order:

1. `cmd/mistermorph/root.go` (entrypoint assembly)
2. `integration/runtime.go` (embedding entrypoint)
3. `agent/engine.go` + `agent/engine_loop.go` (execution core)
4. `internal/channelruntime/telegram/runtime_loop.go` and `internal/channelruntime/slack/runtime.go` (channel flow)
5. `internal/bus/*` and `internal/bus/adapters/*` (message bus and adapters)
