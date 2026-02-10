package agent

import "testing"

func TestToolArgsSummary_ContactsSendSafeSummary(t *testing.T) {
	opts := DefaultLogOptions()
	params := map[string]any{
		"contact_id":     "tg:1001",
		"topic":          "share.proactive.v1",
		"message_text":   "private content should not be logged",
		"source_chat_id": float64(12345),
	}

	got := toolArgsSummary("contacts_send", params, opts)
	if got == nil {
		t.Fatalf("summary should not be nil")
	}
	if got["contact_id"] != "tg:1001" {
		t.Fatalf("unexpected contact_id summary: %#v", got["contact_id"])
	}
	if got["topic"] != "share.proactive.v1" {
		t.Fatalf("unexpected topic summary: %#v", got["topic"])
	}
	if v, ok := got["has_message_text"].(bool); !ok || !v {
		t.Fatalf("expected has_message_text=true, got %#v", got["has_message_text"])
	}
	if _, exists := got["message_text"]; exists {
		t.Fatalf("must not log raw message_text")
	}
}
