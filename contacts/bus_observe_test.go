package contacts

import (
	"context"
	"testing"
	"time"

	busruntime "github.com/quailyquaily/mistermorph/internal/bus"
)

func TestObserveInboundBusMessage_TelegramSenderAndMention(t *testing.T) {
	ctx := context.Background()
	store := NewFileStore(t.TempDir())
	svc := NewService(store)
	now := time.Date(2026, 2, 10, 9, 0, 0, 0, time.UTC)

	_, err := svc.UpsertContact(ctx, Contact{
		ContactID:         "tg:@alice",
		Kind:              KindHuman,
		Channel:           ChannelTelegram,
		ContactNickname:   "Old Alice",
		TGUsername:        "alice",
		TGPrivateChatID:   11001,
		LastInteractionAt: timePtr(now.Add(-24 * time.Hour)),
	}, now)
	if err != nil {
		t.Fatalf("UpsertContact(existing) error = %v", err)
	}

	msg := busruntime.BusMessage{
		Direction:       busruntime.DirectionInbound,
		Channel:         busruntime.ChannelTelegram,
		ConversationKey: "tg:-100500",
		Extensions: busruntime.MessageExtensions{
			ChatType:        "group",
			FromUserID:      42,
			FromUsername:    "alice",
			FromDisplayName: "Alice New",
			MentionUsers:    []string{"@alice", "bob"},
		},
	}
	if err := svc.ObserveInboundBusMessage(ctx, msg, nil, now); err != nil {
		t.Fatalf("ObserveInboundBusMessage() error = %v", err)
	}

	alice, ok, err := svc.GetContact(ctx, "tg:@alice")
	if err != nil {
		t.Fatalf("GetContact(alice) error = %v", err)
	}
	if !ok {
		t.Fatalf("GetContact(alice) expected ok=true")
	}
	if alice.ContactNickname != "Alice New" {
		t.Fatalf("nickname mismatch: got %q want %q", alice.ContactNickname, "Alice New")
	}
	if alice.TGPrivateChatID != 11001 {
		t.Fatalf("tg_private_chat_id should keep old value: got %d want 11001", alice.TGPrivateChatID)
	}
	if len(alice.TGGroupChatIDs) != 1 || alice.TGGroupChatIDs[0] != -100500 {
		t.Fatalf("tg_group_chat_ids mismatch: got=%v", alice.TGGroupChatIDs)
	}
	if alice.LastInteractionAt == nil || !alice.LastInteractionAt.Equal(now) {
		t.Fatalf("last_interaction_at mismatch: got=%v want=%v", alice.LastInteractionAt, now)
	}

	bob, ok, err := svc.GetContact(ctx, "tg:@bob")
	if err != nil {
		t.Fatalf("GetContact(bob) error = %v", err)
	}
	if !ok {
		t.Fatalf("GetContact(bob) expected ok=true")
	}
	if len(bob.TGGroupChatIDs) != 1 || bob.TGGroupChatIDs[0] != -100500 {
		t.Fatalf("tg_group_chat_ids mismatch: got=%v", bob.TGGroupChatIDs)
	}
	if bob.TGPrivateChatID != 0 {
		t.Fatalf("tg_private_chat_id should not be set for mention contact: got %d", bob.TGPrivateChatID)
	}
}

func TestObserveInboundBusMessage_TelegramPrivateChatSetOnce(t *testing.T) {
	ctx := context.Background()
	store := NewFileStore(t.TempDir())
	svc := NewService(store)
	now := time.Date(2026, 2, 10, 9, 30, 0, 0, time.UTC)

	msg := busruntime.BusMessage{
		Direction:       busruntime.DirectionInbound,
		Channel:         busruntime.ChannelTelegram,
		ConversationKey: "tg:90001",
		Extensions: busruntime.MessageExtensions{
			ChatType:     "private",
			FromUserID:   3001,
			FromUsername: "neo",
		},
	}
	if err := svc.ObserveInboundBusMessage(ctx, msg, nil, now); err != nil {
		t.Fatalf("ObserveInboundBusMessage(first) error = %v", err)
	}
	item, ok, err := svc.GetContact(ctx, "tg:@neo")
	if err != nil {
		t.Fatalf("GetContact(first) error = %v", err)
	}
	if !ok {
		t.Fatalf("GetContact(first) expected ok=true")
	}
	if item.TGPrivateChatID != 90001 {
		t.Fatalf("tg_private_chat_id mismatch: got %d want 90001", item.TGPrivateChatID)
	}

	msg.ConversationKey = "tg:90099"
	if err := svc.ObserveInboundBusMessage(ctx, msg, nil, now.Add(1*time.Minute)); err != nil {
		t.Fatalf("ObserveInboundBusMessage(second) error = %v", err)
	}
	item, ok, err = svc.GetContact(ctx, "tg:@neo")
	if err != nil {
		t.Fatalf("GetContact(second) error = %v", err)
	}
	if !ok {
		t.Fatalf("GetContact(second) expected ok=true")
	}
	if item.TGPrivateChatID != 90001 {
		t.Fatalf("tg_private_chat_id should not be overwritten: got %d want 90001", item.TGPrivateChatID)
	}
}

func TestObserveInboundBusMessage_MAEPSenderAndMention(t *testing.T) {
	ctx := context.Background()
	store := NewFileStore(t.TempDir())
	svc := NewService(store)
	now := time.Date(2026, 2, 10, 10, 0, 0, 0, time.UTC)

	payloadBase64, err := busruntime.EncodeMessageEnvelope(
		busruntime.TopicChatMessage,
		busruntime.MessageEnvelope{
			MessageID: "maep:test:1",
			Text:      "hello maep:12D3KooWPeerB and maep:12D3KooWPeerB",
			SentAt:    now.Format(time.RFC3339),
			SessionID: "0194f5c0-8f6e-7d9d-a4d7-6d8d4f35f456",
		},
	)
	if err != nil {
		t.Fatalf("EncodeMessageEnvelope() error = %v", err)
	}
	msg := busruntime.BusMessage{
		Direction:       busruntime.DirectionInbound,
		Channel:         busruntime.ChannelMAEP,
		Topic:           busruntime.TopicChatMessage,
		ConversationKey: "maep:12D3KooWPeerA",
		ParticipantKey:  "12D3KooWPeerA",
		IdempotencyKey:  "msg:maep_test_1",
		PayloadBase64:   payloadBase64,
		CreatedAt:       now,
	}
	if err := svc.ObserveInboundBusMessage(ctx, msg, nil, now); err != nil {
		t.Fatalf("ObserveInboundBusMessage() error = %v", err)
	}

	for _, peerID := range []string{"12D3KooWPeerA", "12D3KooWPeerB"} {
		contactID := "maep:" + peerID
		item, ok, err := svc.GetContact(ctx, contactID)
		if err != nil {
			t.Fatalf("GetContact(%s) error = %v", contactID, err)
		}
		if !ok {
			t.Fatalf("GetContact(%s) expected ok=true", contactID)
		}
		if item.Channel != ChannelMAEP {
			t.Fatalf("channel mismatch for %s: got %s want %s", contactID, item.Channel, ChannelMAEP)
		}
		if item.Kind != KindAgent {
			t.Fatalf("kind mismatch for %s: got %s want %s", contactID, item.Kind, KindAgent)
		}
		if item.LastInteractionAt == nil || !item.LastInteractionAt.Equal(now) {
			t.Fatalf("last_interaction_at mismatch for %s: got=%v want=%v", contactID, item.LastInteractionAt, now)
		}
	}
}

func timePtr(ts time.Time) *time.Time {
	t := ts.UTC()
	return &t
}
