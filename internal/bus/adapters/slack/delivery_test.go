package slack

import (
	"context"
	"strings"
	"testing"
	"time"

	busruntime "github.com/quailyquaily/mistermorph/internal/bus"
)

func TestDeliveryAdapterDeliver(t *testing.T) {
	t.Parallel()

	var gotTarget DeliveryTarget
	var gotText string
	var gotThreadTS string
	calls := 0
	adapter, err := NewDeliveryAdapter(DeliveryAdapterOptions{
		SendText: func(ctx context.Context, target any, text string, opts SendTextOptions) error {
			typed, ok := target.(DeliveryTarget)
			if !ok {
				t.Fatalf("target type mismatch: got %T want DeliveryTarget", target)
			}
			gotTarget = typed
			gotText = text
			gotThreadTS = opts.ThreadTS
			calls++
			return nil
		},
	})
	if err != nil {
		t.Fatalf("NewDeliveryAdapter() error = %v", err)
	}

	payloadBase64, err := busruntime.EncodeMessageEnvelope(busruntime.TopicChatMessage, busruntime.MessageEnvelope{
		MessageID: "msg_5001",
		Text:      "hello slack",
		SentAt:    "2026-02-16T00:00:00Z",
		SessionID: "0194e9d5-2f8f-7000-8000-000000000001",
		ReplyTo:   "1739667000.000050",
	})
	if err != nil {
		t.Fatalf("EncodeMessageEnvelope() error = %v", err)
	}
	conversationKey, err := busruntime.BuildSlackChannelConversationKey("T111:C222")
	if err != nil {
		t.Fatalf("BuildSlackChannelConversationKey() error = %v", err)
	}
	msg := busruntime.BusMessage{
		Direction:       busruntime.DirectionOutbound,
		Channel:         busruntime.ChannelSlack,
		Topic:           busruntime.TopicChatMessage,
		ConversationKey: conversationKey,
		IdempotencyKey:  "msg:msg_5001",
		CorrelationID:   "corr_5001",
		PayloadBase64:   payloadBase64,
		CreatedAt:       time.Now().UTC(),
		Extensions: busruntime.MessageExtensions{
			ThreadTS: "1739667000.000050",
		},
	}
	accepted, deduped, err := adapter.Deliver(context.Background(), msg)
	if err != nil {
		t.Fatalf("Deliver() error = %v", err)
	}
	if !accepted {
		t.Fatalf("accepted mismatch: got %v want true", accepted)
	}
	if deduped {
		t.Fatalf("deduped mismatch: got %v want false", deduped)
	}
	if calls != 1 {
		t.Fatalf("send calls mismatch: got %d want 1", calls)
	}
	if gotTarget.TeamID != "T111" {
		t.Fatalf("team_id mismatch: got %q want %q", gotTarget.TeamID, "T111")
	}
	if gotTarget.ChannelID != "C222" {
		t.Fatalf("channel_id mismatch: got %q want %q", gotTarget.ChannelID, "C222")
	}
	if gotText != "hello slack" {
		t.Fatalf("text mismatch: got %q want %q", gotText, "hello slack")
	}
	if gotThreadTS != "1739667000.000050" {
		t.Fatalf("thread_ts mismatch: got %q want %q", gotThreadTS, "1739667000.000050")
	}
}

func TestDeliveryAdapterRejectsInvalidConversationKey(t *testing.T) {
	t.Parallel()

	calls := 0
	adapter, err := NewDeliveryAdapter(DeliveryAdapterOptions{
		SendText: func(ctx context.Context, target any, text string, opts SendTextOptions) error {
			calls++
			return nil
		},
	})
	if err != nil {
		t.Fatalf("NewDeliveryAdapter() error = %v", err)
	}

	payloadBase64, err := busruntime.EncodeMessageEnvelope(busruntime.TopicChatMessage, busruntime.MessageEnvelope{
		MessageID: "msg_5002",
		Text:      "bad target",
		SentAt:    "2026-02-16T00:00:00Z",
		SessionID: "0194e9d5-2f8f-7000-8000-000000000002",
	})
	if err != nil {
		t.Fatalf("EncodeMessageEnvelope() error = %v", err)
	}
	msg := busruntime.BusMessage{
		Direction:       busruntime.DirectionOutbound,
		Channel:         busruntime.ChannelSlack,
		Topic:           busruntime.TopicChatMessage,
		ConversationKey: "slack:C222",
		IdempotencyKey:  "msg:msg_5002",
		CorrelationID:   "corr_5002",
		PayloadBase64:   payloadBase64,
		CreatedAt:       time.Now().UTC(),
	}
	accepted, deduped, err := adapter.Deliver(context.Background(), msg)
	if err == nil {
		t.Fatalf("Deliver() expected error for invalid conversation key")
	}
	if !strings.Contains(err.Error(), "slack conversation key is invalid") {
		t.Fatalf("Deliver() error mismatch: got %q", err.Error())
	}
	if accepted {
		t.Fatalf("accepted mismatch: got %v want false", accepted)
	}
	if deduped {
		t.Fatalf("deduped mismatch: got %v want false", deduped)
	}
	if calls != 0 {
		t.Fatalf("send calls mismatch: got %d want 0", calls)
	}
}
