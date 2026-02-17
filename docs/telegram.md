# Telegram Runtime: Trigger and Reaction Flow

This document explains how Telegram runtime decides:

1. whether a group message should start an agent run
2. whether to send a text reply or only react with emoji

Code references are under `internal/channelruntime/telegram/*` and `tools/telegram/*`.

## 1) Two Separate Decisions

Telegram runtime uses two independent decisions:

1. Trigger decision (enter agent run or ignore)
2. Output modality decision (text reply vs emoji reaction)

They are not the same check.

## 2) Trigger Decision (Group Message)

Entry point:
- `internal/channelruntime/telegram/runtime.go`
- `internal/channelruntime/telegram/trigger.go`
- `internal/grouptrigger/decision.go`

### 2.1 Quick rules

- `strict`: only explicit mention/reply paths can trigger.
- `smart`: trigger when addressing LLM returns `addressed=true` and `confidence >= threshold`.
- `talkative`: trigger when addressing LLM returns `wanna_interject=true` and `interject > threshold`.

### 2.2 Explicit mention/reply shortcuts

Before LLM addressing, runtime checks explicit signals such as:
- reply to bot message
- mention entity / `@bot_username` mention in body

If explicit match succeeds, trigger is accepted directly.

### 2.3 Important boundary

Trigger layer only decides whether to run the agent.
It does not decide text vs reaction modality.

## 3) Reaction vs Text Decision

Entry point:
- `internal/channelruntime/telegram/runtime_task.go`
- `tools/telegram/react_tool.go`
- `internal/channelruntime/telegram/runtime.go`

### 3.1 When reaction is possible

`telegram_react` is registered only when:
- Telegram API is available
- inbound `message_id` is non-zero

### 3.2 What actually decides "react"

Reaction is considered applied only if the model called `telegram_react` successfully.

Runtime checks:
- `reactTool.LastReaction() != nil`

If true:
- no outbound text message is published for that inbound message
- reaction history item is appended

If false:
- normal text reply path is used

## 4) `is_lightweight` semantics

Addressing output includes `is_lightweight`.
Current runtime treats it as model context/log signal, not as a hard runtime switch.

Hard switch for output modality is still:
- whether `telegram_react` was executed successfully in that run

## 5) Runtime Signals

Useful logs:

- group ignored:
  - `telegram_group_ignored`
- group triggered:
  - `telegram_group_trigger`
- reaction applied:
  - `telegram_reaction_applied`

`telegram_reaction_applied` is an info log, not an error.

## 6) ASCII Flow

```text
Telegram group inbound
  -> explicit mention/reply check
  -> grouptrigger.Decide(mode=strict|smart|talkative)
     -> not triggered: ignore
     -> triggered: runTelegramTask
          -> agent.Engine.Run
             -> optional tool call: telegram_react
          -> reactTool.LastReaction()
             -> reaction != nil: keep reaction only (no text outbound)
             -> reaction == nil: publish normal text reply
```

