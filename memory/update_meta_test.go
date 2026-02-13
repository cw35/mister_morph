package memory

import (
	"os"
	"testing"
	"time"
)

func TestWriteShortTermStoresContactMeta(t *testing.T) {
	root := t.TempDir()
	mgr := NewManager(root, 7)
	date := time.Date(2026, 2, 7, 12, 0, 0, 0, time.UTC)

	_, err := mgr.WriteShortTerm(date, ShortTermContent{
		SummaryItems: []SummaryItem{{Created: "2026-02-07 12:00", Content: "Alice discussed the plan."}},
	}, WriteMeta{
		SessionID:        "tg:1",
		ContactIDs:       []string{"tg:@alice"},
		ContactNicknames: []string{"Alice"},
	})
	if err != nil {
		t.Fatalf("WriteShortTerm() error = %v", err)
	}

	path, _ := mgr.ShortTermSessionPath(date, "tg:1")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", path, err)
	}
	fm, _, ok := ParseFrontmatter(string(raw))
	if !ok {
		t.Fatalf("expected frontmatter in %s", path)
	}
	if len(fm.ContactIDs) != 1 || fm.ContactIDs[0] != "tg:@alice" {
		t.Fatalf("frontmatter contact_id mismatch: got %#v want [%q]", []string(fm.ContactIDs), "tg:@alice")
	}
	if len(fm.ContactNicknames) != 1 || fm.ContactNicknames[0] != "Alice" {
		t.Fatalf("frontmatter contact_nickname mismatch: got %#v want [%q]", []string(fm.ContactNicknames), "Alice")
	}
}
