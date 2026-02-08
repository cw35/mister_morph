package telegramcmd

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/quailyquaily/mistermorph/llm"
)

func TestRenderMAEPFeedbackPrompts(t *testing.T) {
	recent := []llm.Message{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi"},
	}
	inbound := "sounds good"

	sys, user, err := renderMAEPFeedbackPrompts(recent, inbound)
	if err != nil {
		t.Fatalf("renderMAEPFeedbackPrompts() error = %v", err)
	}
	if !strings.Contains(sys, "Classify conversational feedback") {
		t.Fatalf("unexpected system prompt: %q", sys)
	}

	var payload struct {
		RecentTurns  []llm.Message `json:"recent_turns"`
		InboundText  string        `json:"inbound_text"`
		AllowedNext  []string      `json:"allowed_next"`
		SignalBounds string        `json:"signal_bounds"`
	}
	if err := json.Unmarshal([]byte(user), &payload); err != nil {
		t.Fatalf("user prompt is not valid json: %v", err)
	}
	if payload.InboundText != inbound {
		t.Fatalf("inbound_text = %q, want %q", payload.InboundText, inbound)
	}
	if len(payload.RecentTurns) != len(recent) {
		t.Fatalf("recent_turns len = %d, want %d", len(payload.RecentTurns), len(recent))
	}
	if len(payload.AllowedNext) == 0 {
		t.Fatalf("allowed_next is empty")
	}
	if payload.SignalBounds != "[0,1]" {
		t.Fatalf("signal_bounds = %q, want %q", payload.SignalBounds, "[0,1]")
	}
}
