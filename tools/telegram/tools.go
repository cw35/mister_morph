package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/quailyquaily/mistermorph/internal/pathutil"
)

// API is the minimal Telegram transport surface needed by telegram tools.
type API interface {
	SendDocument(ctx context.Context, chatID int64, filePath string, filename string, caption string) error
	SendVoice(ctx context.Context, chatID int64, filePath string, filename string, caption string) error
	SetEmojiReaction(ctx context.Context, chatID int64, messageID int64, emoji string, isBig *bool) error
}

type Reaction struct {
	ChatID    int64
	MessageID int64
	Emoji     string
	Source    string
}

type SendFileTool struct {
	api      API
	chatID   int64
	cacheDir string
	maxBytes int64
}

func NewSendFileTool(api API, chatID int64, cacheDir string, maxBytes int64) *SendFileTool {
	if maxBytes <= 0 {
		maxBytes = 20 * 1024 * 1024
	}
	return &SendFileTool{
		api:      api,
		chatID:   chatID,
		cacheDir: strings.TrimSpace(cacheDir),
		maxBytes: maxBytes,
	}
}

func (t *SendFileTool) Name() string { return "telegram_send_file" }

func (t *SendFileTool) Description() string {
	return "Sends a local file (from file_cache_dir) back to the current chat as a document. If you need more advanced behavior, describe it in text instead."
}

func (t *SendFileTool) ParameterSchema() string {
	s := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Path to a local file under file_cache_dir (absolute or relative to that directory).",
			},
			"filename": map[string]any{
				"type":        "string",
				"description": "Optional filename shown to the user (default: basename of path).",
			},
			"caption": map[string]any{
				"type":        "string",
				"description": "Optional caption text.",
			},
		},
		"required": []string{"path"},
	}
	b, _ := json.MarshalIndent(s, "", "  ")
	return string(b)
}

func (t *SendFileTool) Execute(ctx context.Context, params map[string]any) (string, error) {
	if t == nil || t.api == nil {
		return "", fmt.Errorf("telegram_send_file is disabled")
	}
	rawPath, _ := params["path"].(string)
	rawPath = strings.TrimSpace(rawPath)
	if rawPath == "" {
		return "", fmt.Errorf("missing required param: path")
	}
	rawPath = pathutil.NormalizeFileCacheDirPath(rawPath)
	cacheDir := strings.TrimSpace(t.cacheDir)
	if cacheDir == "" {
		return "", fmt.Errorf("file cache dir is not configured")
	}

	p := rawPath
	if !filepath.IsAbs(p) {
		p = filepath.Join(cacheDir, p)
	}
	p = filepath.Clean(p)

	cacheAbs, err := filepath.Abs(cacheDir)
	if err != nil {
		return "", err
	}
	pathAbs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(cacheAbs, pathAbs)
	if err != nil {
		return "", err
	}
	if rel == "." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) || rel == ".." {
		return "", fmt.Errorf("refusing to send file outside file_cache_dir: %s", pathAbs)
	}

	st, err := os.Stat(pathAbs)
	if err != nil {
		return "", err
	}
	if st.IsDir() {
		return "", fmt.Errorf("path is a directory: %s", pathAbs)
	}
	if t.maxBytes > 0 && st.Size() > t.maxBytes {
		return "", fmt.Errorf("file too large to send (>%d bytes): %s", t.maxBytes, pathAbs)
	}

	filename, _ := params["filename"].(string)
	filename = strings.TrimSpace(filename)
	if filename == "" {
		filename = filepath.Base(pathAbs)
	}
	filename = sanitizeFilename(filename)

	caption, _ := params["caption"].(string)
	caption = strings.TrimSpace(caption)

	if err := t.api.SendDocument(ctx, t.chatID, pathAbs, filename, caption); err != nil {
		return "", err
	}
	return fmt.Sprintf("sent file: %s", filename), nil
}

func sanitizeFilename(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "file"
	}
	name = filepath.Base(name)
	var b strings.Builder
	b.Grow(len(name))
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '.' || r == '_' || r == '-' || r == '+':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	out := strings.Trim(b.String(), "._- ")
	if out == "" {
		return "file"
	}
	const max = 120
	if len(out) > max {
		out = out[:max]
	}
	return out
}
