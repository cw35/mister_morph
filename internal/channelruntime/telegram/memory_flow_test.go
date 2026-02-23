package telegram

import (
	"testing"
)

func TestBuildMemoryWriteMeta(t *testing.T) {
	t.Run("heartbeat uses fixed session path", func(t *testing.T) {
		meta := buildMemoryWriteMeta(telegramJob{
			IsHeartbeat:  true,
			ChatID:       42,
			FromUserID:   1001,
			FromUsername: "alice",
		})
		if meta.SessionID != heartbeatMemorySessionID {
			t.Fatalf("session_id = %q, want %q", meta.SessionID, heartbeatMemorySessionID)
		}
		if len(meta.ContactIDs) != 0 {
			t.Fatalf("contact_ids = %#v, want empty", meta.ContactIDs)
		}
		if len(meta.ContactNicknames) != 0 {
			t.Fatalf("contact_nicknames = %#v, want empty", meta.ContactNicknames)
		}
	})

	t.Run("normal chat keeps tg session and contact meta", func(t *testing.T) {
		meta := buildMemoryWriteMeta(telegramJob{
			ChatID:          777,
			FromUserID:      1001,
			FromUsername:    "@alice",
			FromDisplayName: "Alice",
		})
		if meta.SessionID != "tg:777" {
			t.Fatalf("session_id = %q, want %q", meta.SessionID, "tg:777")
		}
		if len(meta.ContactIDs) != 1 || meta.ContactIDs[0] != "tg:@alice" {
			t.Fatalf("contact_ids = %#v, want [\"tg:@alice\"]", meta.ContactIDs)
		}
		if len(meta.ContactNicknames) != 1 || meta.ContactNicknames[0] != "Alice" {
			t.Fatalf("contact_nicknames = %#v, want [\"Alice\"]", meta.ContactNicknames)
		}
	})
}
