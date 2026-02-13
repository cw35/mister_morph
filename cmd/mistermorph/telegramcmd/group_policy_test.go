package telegramcmd

import (
	"context"
	"strings"
	"testing"

	"github.com/quailyquaily/mistermorph/agent"
)

func TestGroupTriggerDecision_ReplyPath(t *testing.T) {
	msg := &telegramMessage{
		Text: "please continue",
		ReplyTo: &telegramMessage{
			From: &telegramUser{ID: 42},
		},
	}
	dec, ok, err := groupTriggerDecision(context.Background(), nil, "", msg, "morphbot", 42, nil, "strict", 24, 0, 0.55, 0.55, nil)
	if err != nil {
		t.Fatalf("groupTriggerDecision() error = %v", err)
	}
	if !ok {
		t.Fatalf("expected trigger for reply-to-bot")
	}
	if dec.Reason != "reply" {
		t.Fatalf("unexpected reason: %q", dec.Reason)
	}
	if dec.AddressingImpulse != 1 {
		t.Fatalf("impulse = %v, want 1", dec.AddressingImpulse)
	}
}

func TestQuoteReplyMessageIDForGroupTrigger(t *testing.T) {
	msg := &telegramMessage{MessageID: 1234}
	high := quoteReplyMessageIDForGroupTrigger(msg, telegramGroupTriggerDecision{
		AddressingImpulse: 0.81,
	})
	if high != 1234 {
		t.Fatalf("high impulse reply_to mismatch: got %d want 1234", high)
	}

	low := quoteReplyMessageIDForGroupTrigger(msg, telegramGroupTriggerDecision{
		AddressingImpulse: 0.8,
	})
	if low != 0 {
		t.Fatalf("low impulse reply_to mismatch: got %d want 0", low)
	}
}

func TestGroupTriggerDecision_StrictIgnoresAlias(t *testing.T) {
	msg := &telegramMessage{
		Text: "morph can you check this",
	}
	dec, ok, err := groupTriggerDecision(context.Background(), nil, "", msg, "morphbot", 42, []string{"morph"}, "strict", 24, 0, 0.55, 0.55, nil)
	if err != nil {
		t.Fatalf("groupTriggerDecision() error = %v", err)
	}
	if ok {
		t.Fatalf("strict mode should ignore alias-only trigger")
	}
	_ = dec
}

func TestGroupTriggerDecision_TalkativeAlwaysRequestsAddressingLLM(t *testing.T) {
	msg := &telegramMessage{
		Text: "just discussing among people",
	}
	dec, ok, err := groupTriggerDecision(context.Background(), nil, "", msg, "morphbot", 42, nil, "talkative", 24, 0, 0.55, 0.55, nil)
	if err != nil {
		t.Fatalf("groupTriggerDecision() error = %v", err)
	}
	if ok {
		t.Fatalf("talkative mode should defer triggering to addressing llm")
	}
	if !dec.AddressingLLMAttempted {
		t.Fatalf("talkative mode should always attempt addressing llm")
	}
	if dec.Reason != "talkative" {
		t.Fatalf("unexpected reason: %q", dec.Reason)
	}
}

func TestGroupTriggerDecision_MentionEntityTriggers(t *testing.T) {
	msg := &telegramMessage{
		Text: "@morphbot please check",
		Entities: []telegramEntity{
			{Type: "mention", Offset: 0, Length: 9},
		},
	}
	dec, ok, err := groupTriggerDecision(context.Background(), nil, "", msg, "morphbot", 42, nil, "strict", 24, 0, 0.55, 0.55, nil)
	if err != nil {
		t.Fatalf("groupTriggerDecision() error = %v", err)
	}
	if !ok {
		t.Fatalf("mention entity should trigger")
	}
	if dec.Reason != "mention_entity" {
		t.Fatalf("unexpected reason: %q", dec.Reason)
	}
	if dec.AddressingImpulse != 1 {
		t.Fatalf("impulse = %v, want 1", dec.AddressingImpulse)
	}
}

func TestGroupTriggerDecision_ExplicitMentionBypassesLLMEvenInTalkative(t *testing.T) {
	msg := &telegramMessage{
		Text: "@morphbot hello",
		Entities: []telegramEntity{
			{Type: "mention", Offset: 0, Length: 9},
		},
	}
	dec, ok, err := groupTriggerDecision(context.Background(), nil, "", msg, "morphbot", 42, []string{"morph"}, "talkative", 24, 0, 0.55, 0.55, nil)
	if err != nil {
		t.Fatalf("groupTriggerDecision() error = %v", err)
	}
	if !ok {
		t.Fatalf("explicit mention should trigger directly")
	}
	if dec.Reason != "mention_entity" {
		t.Fatalf("unexpected reason: %q", dec.Reason)
	}
	if dec.AddressingLLMAttempted {
		t.Fatalf("explicit mention should bypass addressing llm")
	}
	if dec.AddressingImpulse != 1 {
		t.Fatalf("impulse = %v, want 1", dec.AddressingImpulse)
	}
}

func TestGroupTriggerDecision_SmartMentionRoutesThroughAddressingLLM(t *testing.T) {
	msg := &telegramMessage{
		Text: "let us use morphism to describe this",
	}
	dec, ok, err := groupTriggerDecision(context.Background(), nil, "", msg, "morphbot", 42, []string{"morph"}, "smart", 24, 0, 0.55, 0.55, nil)
	if err != nil {
		t.Fatalf("groupTriggerDecision() error = %v", err)
	}
	if ok {
		t.Fatalf("without llm client, smart mode should not trigger")
	}
	if !dec.AddressingLLMAttempted {
		t.Fatalf("expected addressing llm to be attempted in smart mode")
	}
	if !strings.HasPrefix(dec.Reason, "alias_") {
		t.Fatalf("unexpected reason: %q", dec.Reason)
	}
}

func TestApplyTelegramGroupRuntimePromptRules_GroupOnly(t *testing.T) {
	groupSpec := agent.PromptSpec{}
	applyTelegramRuntimePromptBlocks(&groupSpec, "group", []string{"@alice"})
	if !hasPromptBlockTitle(groupSpec.Blocks, "Telegram Runtime Rules") {
		t.Fatalf("group chat should include telegram runtime rules block")
	}
	if !hasBlockContaining(groupSpec.Blocks, "Telegram Runtime Rules", "anti triple-tap") {
		t.Fatalf("group chat should include anti triple-tap guidance")
	}
	if !hasBlockContaining(groupSpec.Blocks, "Telegram Runtime Rules", "call `telegram_react` tool instead of text") {
		t.Fatalf("group chat should include reaction preference guidance")
	}

	privateSpec := agent.PromptSpec{}
	applyTelegramRuntimePromptBlocks(&privateSpec, "private", []string{"@alice"})
	if !hasPromptBlockTitle(privateSpec.Blocks, "Telegram Runtime Rules") {
		t.Fatalf("private chat should include telegram runtime rules block")
	}
	if hasBlockContaining(privateSpec.Blocks, "Telegram Runtime Rules", "anti triple-tap") {
		t.Fatalf("non-group chat must not include group-only guidance")
	}
}

func hasPromptBlockTitle(blocks []agent.PromptBlock, want string) bool {
	for _, block := range blocks {
		if strings.EqualFold(strings.TrimSpace(block.Title), strings.TrimSpace(want)) {
			return true
		}
	}
	return false
}

func hasBlockContaining(blocks []agent.PromptBlock, title string, snippet string) bool {
	title = strings.TrimSpace(title)
	snippet = strings.ToLower(strings.TrimSpace(snippet))
	for _, block := range blocks {
		if !strings.EqualFold(strings.TrimSpace(block.Title), title) {
			continue
		}
		if strings.Contains(strings.ToLower(block.Content), snippet) {
			return true
		}
	}
	return false
}
