# Telegram Runtime: Trigger, Reaction, and Text Policy

This document describes how Telegram runtime handles one inbound message.

Code areas:
- `internal/channelruntime/telegram/runtime.go`
- `internal/channelruntime/telegram/trigger.go`
- `internal/channelruntime/telegram/trigger_addressing.go`
- `internal/channelruntime/telegram/runtime_task.go`
- `tools/telegram/react_tool.go`
- `internal/grouptrigger/decision.go`

## 1) Mental Model

There are three separate decisions:

1. Trigger decision: should this group message enter the main agent run?
2. Reaction execution: did `telegram_react` actually run successfully?
3. Text publish decision: should runtime publish `final.output` as text?

These are related, but not the same switch.

## 2) End-to-End Flow

```text
Inbound group message
  -> prefilter (reply/mention guard)
  -> addressing LLM + grouptrigger.Decide(...)
     -> may call telegram_react in pre-run
     -> decide trigger yes/no
  -> if triggered: runTelegramTask (main run)
     -> main run may call telegram_react again
     -> final.is_lightweight decides whether text is published
```

### 2.1 Pre Filter: `shouldSkipGroupReplyWithoutBodyMention`

Before addressing LLM, runtime applies a prefilter in group chats:

- function: `shouldSkipGroupReplyWithoutBodyMention`
- file: `internal/channelruntime/telegram/trigger.go`

Current skip rule:

- message is from a human (not bot)
- message has `ReplyTo` (looks like a reply)
- reply target is not this bot
- message body does not mention this bot (`groupBodyMentionReason(...) == false`)

If all match, runtime skips this message before `groupTriggerDecision(...)`.

This prefilter is exactly where topic/thread reply-shape edge cases can affect triggering in supergroups.

## 3) Trigger Stage (Pre-Run)

### 3.1 Modes

`group_trigger_mode` behavior:

- `strict`: only explicit triggers (reply/mention/command path) run.
- `smart`: requires `addressed=true` and `confidence >= threshold`.
- `talkative`: requires `wanna_interject=true` and `interject > threshold`.

### 3.2 Explicit signals

Explicit signals include:

- reply to bot
- mention entity
- body `@bot_username` mention

If explicit match is true, trigger is accepted without LLM addressing threshold checks.

### 3.3 Pre-run `telegram_react`

Addressing stage can now receive `telegram_react` as a callable tool.
Runtime creates a temporary `ReactTool` bound to the current inbound `chat_id` and `message_id`.

If addressing LLM calls it successfully:

- Telegram reaction is sent immediately.
- Runtime logs `telegram_group_addressing_reaction_applied`.

Important:

- Pre-run reaction does not terminate the pipeline.
- Trigger decision still continues.
- If trigger is accepted, main run still starts.

### 3.4 Trigger output metadata

`Decision.Addressing` contains fields like:

- `addressed`
- `confidence`
- `wanna_interject`
- `interject`
- `impulse`
- `is_lightweight`
- `reason`

`Decision.Addressing.is_lightweight` is metadata from addressing classification.
It is not the final text publish switch.

## 4) Main Run (Generation Stage)

Inside `runTelegramTask(...)`, runtime builds a fresh tool registry and registers `telegram_react` again (if API and `message_id` are available).

After `agent.Engine.Run(...)`, runtime reads `reactTool.LastReaction()`:

- `nil`: no successful main-run reaction
- non-`nil`: successful main-run reaction, log `telegram_reaction_applied`, append outbound reaction history

This `LastReaction()` reflects only the main-run `ReactTool` instance.

## 5) Text Publish Rule

Text publishing is controlled only by `final.is_lightweight`:

- `final.is_lightweight == true` -> do not publish text
- `final.is_lightweight == false` -> publish text

Runtime function:

- `shouldPublishTelegramText(final) == !final.IsLightweight`

Pre-run reaction does not change this rule.

## 6) Pre-Run vs Main-Run Reaction Interaction

Pre-run and main-run use different `ReactTool` instances.

Result:

- pre-run may react once
- main-run may react again
- same inbound message may receive two reaction API calls

This is expected with current design.

## 7) Logging Signals

Useful logs for debugging:

- `telegram_group_ignored_reply_without_at_mention`
- `telegram_group_ignored`
- `telegram_group_trigger`
- `telegram_group_addressing_reaction_applied`
- `telegram_reaction_applied`

## 8) Quick Behavior Matrix

- pre-run reacted, main-run no reaction, `final.is_lightweight=false`: reaction + text
- pre-run reacted, main-run reacted, `final.is_lightweight=false`: possibly two reactions + text
- pre-run reacted, main-run no reaction, `final.is_lightweight=true`: reaction only
- pre-run no reaction, main-run reacted, `final.is_lightweight=true`: reaction only
