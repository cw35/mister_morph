package telegram

import (
	"testing"
	"time"
)

func TestNormalizeAllowedChatIDs(t *testing.T) {
	got := normalizeAllowedChatIDs([]int64{1, 0, 2, 1})
	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2 (%#v)", len(got), got)
	}
	if got[0] != 1 || got[1] != 2 {
		t.Fatalf("got = %#v, want [1 2]", got)
	}
}

func TestNormalizeRunStringSlice(t *testing.T) {
	got := normalizeRunStringSlice([]string{" /ip4/1 ", "", " /ip4/2 ", "/ip4/1"})
	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2 (%#v)", len(got), got)
	}
	if got[0] != "/ip4/1" || got[1] != "/ip4/2" {
		t.Fatalf("got = %#v, want [/ip4/1 /ip4/2]", got)
	}
}

func TestTelegramHeartbeatTaskID(t *testing.T) {
	t1 := time.Unix(0, 1700000000000000000).UTC()
	t2 := t1.Add(time.Second)
	id1 := telegramHeartbeatTaskID(0, t1)
	id2 := telegramHeartbeatTaskID(0, t2)
	if id1 == "" || id2 == "" {
		t.Fatalf("heartbeat task id should not be empty: %q %q", id1, id2)
	}
	if id1 == id2 {
		t.Fatalf("heartbeat task ids should differ across schedules: %q %q", id1, id2)
	}
}
