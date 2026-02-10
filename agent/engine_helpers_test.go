package agent

import "testing"

func TestToolArgsSummary_ContactsSendSafeSummary(t *testing.T) {
	opts := DefaultLogOptions()
	params := map[string]any{
		"contact_id":   "tg:1001",
		"content_type": "application/json",
		"message_text": "private content should not be logged",
	}

	got := toolArgsSummary("contacts_send", params, opts)
	if got == nil {
		t.Fatalf("summary should not be nil")
	}
	if got["contact_id"] != "tg:1001" {
		t.Fatalf("unexpected contact_id summary: %#v", got["contact_id"])
	}
	if got["content_type"] != "application/json" {
		t.Fatalf("unexpected content_type summary: %#v", got["content_type"])
	}
	if v, ok := got["has_message_text"].(bool); !ok || !v {
		t.Fatalf("expected has_message_text=true, got %#v", got["has_message_text"])
	}
	if _, exists := got["message_text"]; exists {
		t.Fatalf("must not log raw message_text")
	}
}
