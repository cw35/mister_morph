package consolecmd

import (
	"path/filepath"
	"testing"
	"time"
)

func TestSessionStorePersistsAcrossRestart(t *testing.T) {
	t.Parallel()

	storePath := filepath.Join(t.TempDir(), "sessions.json")
	store1 := newSessionStore(storePath)

	token, expiresAt, err := store1.Create(2 * time.Hour)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	if token == "" {
		t.Fatalf("expected non-empty token")
	}
	if !expiresAt.After(time.Now().UTC()) {
		t.Fatalf("expected future expiry, got %s", expiresAt.Format(time.RFC3339Nano))
	}

	store2 := newSessionStore(storePath)
	gotExpiresAt, ok := store2.Validate(token)
	if !ok {
		t.Fatalf("expected token to remain valid after restart")
	}
	if !gotExpiresAt.Equal(expiresAt) {
		t.Fatalf("unexpected expiry: got=%s want=%s", gotExpiresAt.Format(time.RFC3339Nano), expiresAt.Format(time.RFC3339Nano))
	}
}

func TestSessionStoreDeletePersistsAcrossRestart(t *testing.T) {
	t.Parallel()

	storePath := filepath.Join(t.TempDir(), "sessions.json")
	store1 := newSessionStore(storePath)

	token, _, err := store1.Create(2 * time.Hour)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	store1.Delete(token)

	store2 := newSessionStore(storePath)
	if _, ok := store2.Validate(token); ok {
		t.Fatalf("expected token to stay deleted after restart")
	}
}
