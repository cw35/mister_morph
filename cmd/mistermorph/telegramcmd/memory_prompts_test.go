package telegramcmd

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/quailyquaily/mistermorph/internal/chathistory"
	"github.com/quailyquaily/mistermorph/memory"
)

func TestRenderMemoryDraftPrompts(t *testing.T) {
	sys, user, err := renderMemoryDraftPrompts(
		MemoryDraftContext{SessionID: "tg:1", ChatType: "private"},
		[]chathistory.ChatHistoryItem{
			{
				Channel: chathistory.ChannelTelegram,
				Kind:    chathistory.KindInboundUser,
				SentAt:  time.Date(2026, 2, 11, 9, 30, 0, 0, time.UTC),
				Text:    "hi",
			},
		},
		"task",
		"output",
		memory.ShortTermContent{
			SummaryItems: []memory.SummaryItem{{Created: "2026-02-11 09:30", Content: "The agent discussed progress with [Alice](tg:@alice)."}},
		},
	)
	if err != nil {
		t.Fatalf("renderMemoryDraftPrompts() error = %v", err)
	}
	if !strings.Contains(sys, "single agent session") {
		t.Fatalf("unexpected system prompt: %q", sys)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(user), &payload); err != nil {
		t.Fatalf("user prompt is not valid json: %v", err)
	}
	if payload["session_context"] == nil {
		t.Fatalf("missing session_context")
	}
	if payload["chat_history"] == nil {
		t.Fatalf("missing chat_history")
	}
	if payload["current_task"] == nil {
		t.Fatalf("missing current_task")
	}
	if payload["current_output"] == nil {
		t.Fatalf("missing current_output")
	}
	if payload["existing_summary_items"] == nil {
		t.Fatalf("missing existing memory payload")
	}
}
