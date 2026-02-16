package slackcmd

import (
	"encoding/json"
	"testing"
	"time"
)

func TestParseSlackInboundEvent_AppMention(t *testing.T) {
	t.Parallel()

	payload, err := json.Marshal(map[string]any{
		"team_id":    "T111",
		"event_id":   "Ev01",
		"event_time": 1739667600,
		"event": map[string]any{
			"type":         "app_mention",
			"user":         "U111",
			"text":         "<@U999> hello there",
			"channel":      "C222",
			"channel_type": "channel",
			"ts":           "1739667600.000100",
		},
	})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	event, ok, err := parseSlackInboundEvent(slackSocketEnvelope{
		Type:    "events_api",
		Payload: payload,
	}, "U999")
	if err != nil {
		t.Fatalf("parseSlackInboundEvent() error = %v", err)
	}
	if !ok {
		t.Fatalf("parseSlackInboundEvent() ok=false, want true")
	}
	if event.TeamID != "T111" {
		t.Fatalf("team_id mismatch: got %q want %q", event.TeamID, "T111")
	}
	if event.ChannelID != "C222" {
		t.Fatalf("channel_id mismatch: got %q want %q", event.ChannelID, "C222")
	}
	if event.UserID != "U111" {
		t.Fatalf("user_id mismatch: got %q want %q", event.UserID, "U111")
	}
	if !event.IsAppMention {
		t.Fatalf("is_app_mention mismatch: got false want true")
	}
	if event.ChatType != "channel" {
		t.Fatalf("chat_type mismatch: got %q want %q", event.ChatType, "channel")
	}
	if event.SentAt.IsZero() {
		t.Fatalf("sent_at should not be zero")
	}
	wantSentAt := time.Unix(1739667600, 0).UTC()
	if !event.SentAt.Equal(wantSentAt) {
		t.Fatalf("sent_at mismatch: got %s want %s", event.SentAt.Format(time.RFC3339), wantSentAt.Format(time.RFC3339))
	}
}

func TestParseSlackInboundEvent_IgnoresSelfMessage(t *testing.T) {
	t.Parallel()

	payload, err := json.Marshal(map[string]any{
		"team_id":  "T111",
		"event_id": "Ev02",
		"event": map[string]any{
			"type":    "message",
			"user":    "U999",
			"text":    "hello",
			"channel": "C222",
			"ts":      "1739667600.000100",
		},
	})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	_, ok, err := parseSlackInboundEvent(slackSocketEnvelope{
		Type:    "events_api",
		Payload: payload,
	}, "U999")
	if err != nil {
		t.Fatalf("parseSlackInboundEvent() error = %v", err)
	}
	if ok {
		t.Fatalf("parseSlackInboundEvent() ok=true, want false")
	}
}

func TestDecideSlackGroupTrigger_Strict(t *testing.T) {
	t.Parallel()

	eventMention := slackInboundEvent{
		Text:            "<@U999> hello",
		IsAppMention:    true,
		IsThreadMessage: false,
	}
	dec, ok, err := decideSlackGroupTrigger(nil, nil, "", eventMention, "U999", "strict", 0, 0.6, 0.6, nil)
	if err != nil {
		t.Fatalf("decideSlackGroupTrigger(app_mention) error = %v", err)
	}
	if !ok {
		t.Fatalf("decideSlackGroupTrigger(app_mention) ok=false, want true")
	}
	if dec.AddressingImpulse != 1 {
		t.Fatalf("addressing_impulse mismatch: got %v want 1", dec.AddressingImpulse)
	}

	eventIgnored := slackInboundEvent{
		Text: "hello everyone",
	}
	_, ok, err = decideSlackGroupTrigger(nil, nil, "", eventIgnored, "U999", "strict", 0, 0.6, 0.6, nil)
	if err != nil {
		t.Fatalf("decideSlackGroupTrigger(non_mention) error = %v", err)
	}
	if ok {
		t.Fatalf("decideSlackGroupTrigger(non_mention) ok=true, want false")
	}
}
