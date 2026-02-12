package telegramcmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func TestRenderTelegramAddressingPrompts(t *testing.T) {
	stateDir := t.TempDir()
	prevStateDir := viper.GetString("file_state_dir")
	viper.Set("file_state_dir", stateDir)
	t.Cleanup(func() {
		viper.Set("file_state_dir", prevStateDir)
	})
	soul := "---\nstatus: done\n---\n\n# SOUL.md\n\n## Boundaries\n- Keep it concise.\n"
	if err := os.WriteFile(filepath.Join(stateDir, "SOUL.md"), []byte(soul), 0o644); err != nil {
		t.Fatalf("write SOUL.md: %v", err)
	}

	sys, user, err := renderTelegramAddressingPrompts("my_bot", []string{"mybot", "morph"}, "hey mybot do this", "alias heuristic uncertain")
	if err != nil {
		t.Fatalf("renderTelegramAddressingPrompts() error = %v", err)
	}
	if !strings.Contains(sys, "handled by me according following rules") {
		t.Fatalf("unexpected system prompt: %q", sys)
	}
	if !strings.Contains(sys, "## Boundaries") {
		t.Fatalf("system prompt should include SOUL content: %q", sys)
	}

	var payload struct {
		BotUsername string   `json:"bot_username"`
		Aliases     []string `json:"aliases"`
		Message     string   `json:"message"`
		Note        string   `json:"note"`
	}
	if err := json.Unmarshal([]byte(user), &payload); err != nil {
		t.Fatalf("user prompt is not valid json: %v", err)
	}
	if payload.BotUsername != "my_bot" {
		t.Fatalf("bot_username = %q, want my_bot", payload.BotUsername)
	}
	if len(payload.Aliases) != 2 {
		t.Fatalf("aliases len = %d, want 2", len(payload.Aliases))
	}
	if strings.TrimSpace(payload.Message) == "" {
		t.Fatalf("message should not be empty")
	}
	if payload.Note != "alias heuristic uncertain" {
		t.Fatalf("note = %q, want custom note", payload.Note)
	}
}
