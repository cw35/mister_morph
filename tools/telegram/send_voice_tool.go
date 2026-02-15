package telegram

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/quailyquaily/mistermorph/internal/pathutil"
)

type SendVoiceTool struct {
	api        API
	defaultTo  int64
	cacheDir   string
	maxBytes   int64
	allowedIDs map[int64]bool
}

func NewSendVoiceTool(api API, defaultChatID int64, cacheDir string, maxBytes int64, allowedIDs map[int64]bool) *SendVoiceTool {
	if maxBytes <= 0 {
		maxBytes = 20 * 1024 * 1024
	}
	return &SendVoiceTool{
		api:        api,
		defaultTo:  defaultChatID,
		cacheDir:   strings.TrimSpace(cacheDir),
		maxBytes:   maxBytes,
		allowedIDs: allowedIDs,
	}
}

func (t *SendVoiceTool) Name() string { return "telegram_send_voice" }

func (t *SendVoiceTool) Description() string {
	return "Sends a Telegram voice message. Provide either a local .ogg/.opus file under file_cache_dir, or omit path and provide text to synthesize locally. Use chat_id when not running in an active chat context."
}

func (t *SendVoiceTool) ParameterSchema() string {
	s := map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"properties": map[string]any{
			"chat_id": map[string]any{
				"type":        "integer",
				"description": "Target Telegram chat_id. Optional in interactive chat context; required for scheduled runs unless default chat_id is set.",
			},
			"path": map[string]any{
				"type":        "string",
				"description": "Path to a local voice file under file_cache_dir (absolute or relative to that directory). Recommended: .ogg with Opus audio. If omitted, the tool can synthesize a voice file from `text`.",
			},
			"text": map[string]any{
				"type":        "string",
				"description": "Text to synthesize into a voice message when `path` is omitted. If omitted, falls back to `caption`.",
			},
			"lang": map[string]any{
				"type":        "string",
				"description": "Optional language tag for TTS (BCP-47, e.g., en-US, zh-CN). If omitted, auto-detect.",
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
		"required": []string{},
	}
	b, _ := json.MarshalIndent(s, "", "  ")
	return string(b)
}

func (t *SendVoiceTool) Execute(ctx context.Context, params map[string]any) (string, error) {
	if t == nil || t.api == nil {
		return "", fmt.Errorf("telegram_send_voice is disabled")
	}

	chatID := t.defaultTo
	if v, ok := params["chat_id"]; ok {
		switch x := v.(type) {
		case int64:
			chatID = x
		case int:
			chatID = int64(x)
		case float64:
			chatID = int64(x)
		}
	}
	if chatID == 0 {
		return "", fmt.Errorf("missing required param: chat_id")
	}
	if len(t.allowedIDs) > 0 && !t.allowedIDs[chatID] {
		return "", fmt.Errorf("unauthorized chat_id: %d", chatID)
	}

	cacheDir := strings.TrimSpace(t.cacheDir)
	if cacheDir == "" {
		return "", fmt.Errorf("file cache dir is not configured")
	}

	cacheAbs, err := filepath.Abs(cacheDir)
	if err != nil {
		return "", err
	}
	caption, _ := params["caption"].(string)
	caption = strings.TrimSpace(caption)

	rawPath, _ := params["path"].(string)
	rawPath = strings.TrimSpace(rawPath)
	rawPath = pathutil.NormalizeFileCacheDirPath(rawPath)

	var pathAbs string
	if rawPath != "" {
		p := rawPath
		if !filepath.IsAbs(p) {
			p = filepath.Join(cacheDir, p)
		}
		p = filepath.Clean(p)

		pathAbs, err = filepath.Abs(p)
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
	} else {
		text, _ := params["text"].(string)
		text = strings.TrimSpace(text)
		if text == "" {
			text = caption
		}
		lang, _ := params["lang"].(string)
		lang = strings.TrimSpace(lang)
		synthCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		pathAbs, err = synthesizeVoiceToOggOpusWithLang(synthCtx, cacheAbs, text, lang)
		if err != nil {
			return "", err
		}
	}

	filename, _ := params["filename"].(string)
	filename = strings.TrimSpace(filename)
	if filename == "" {
		filename = filepath.Base(pathAbs)
	}
	filename = sanitizeFilename(filename)

	if err := t.api.SendVoice(ctx, chatID, pathAbs, filename, caption); err != nil {
		return "", err
	}
	return fmt.Sprintf("sent voice: %s", filename), nil
}

func commandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

type ttsLang struct {
	Pico   string
	Espeak string
}

func resolveTTSLang(lang string, text string) ttsLang {
	lang = strings.TrimSpace(lang)
	if lang == "" {
		lang = detectLangFromText(text)
	}
	base := strings.ToLower(strings.Split(strings.ReplaceAll(lang, "_", "-"), "-")[0])
	pico := normalizePicoLang(base)
	espeak := normalizeEspeakLang(base)
	if pico == "" && base == "en" {
		pico = "en-US"
	}
	return ttsLang{Pico: pico, Espeak: espeak}
}

func detectLangFromText(text string) string {
	for _, r := range text {
		switch {
		case unicode.In(r, unicode.Han):
			return "zh-CN"
		case unicode.In(r, unicode.Hiragana, unicode.Katakana):
			return "ja-JP"
		case unicode.In(r, unicode.Hangul):
			return "ko-KR"
		case unicode.In(r, unicode.Cyrillic):
			return "ru-RU"
		case unicode.In(r, unicode.Arabic):
			return "ar-SA"
		case unicode.In(r, unicode.Devanagari):
			return "hi-IN"
		}
	}
	return "en-US"
}

func normalizePicoLang(base string) string {
	switch base {
	case "en":
		return "en-US"
	case "de":
		return "de-DE"
	case "es":
		return "es-ES"
	case "fr":
		return "fr-FR"
	case "it":
		return "it-IT"
	default:
		return ""
	}
}

func normalizeEspeakLang(base string) string {
	if base == "" {
		return ""
	}
	return base
}

func selectTTSCmd(ctx context.Context, wavPath string, text string, lang ttsLang) *exec.Cmd {
	if commandExists("pico2wave") && lang.Pico != "" {
		// pico2wave writes the WAV file directly.
		return exec.CommandContext(ctx, "pico2wave", "-l", lang.Pico, "-w", wavPath, text)
	}
	if commandExists("espeak-ng") {
		if lang.Espeak != "" {
			return exec.CommandContext(ctx, "espeak-ng", "-v", lang.Espeak, "-w", wavPath, text)
		}
		return exec.CommandContext(ctx, "espeak-ng", "-w", wavPath, text)
	}
	if commandExists("espeak") {
		if lang.Espeak != "" {
			return exec.CommandContext(ctx, "espeak", "-v", lang.Espeak, "-w", wavPath, text)
		}
		return exec.CommandContext(ctx, "espeak", "-w", wavPath, text)
	}
	if commandExists("flite") {
		return exec.CommandContext(ctx, "flite", "-t", text, "-o", wavPath)
	}
	return nil
}

func synthesizeVoiceToOggOpusWithLang(ctx context.Context, cacheDir string, text string, lang string) (string, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "", fmt.Errorf("missing voice synthesis text")
	}
	// Keep this bounded: huge TTS is slow and can exceed Telegram limits.
	if len(text) > 1200 {
		text = strings.TrimSpace(text[:1200])
	}

	cacheDir = strings.TrimSpace(cacheDir)
	if cacheDir == "" {
		return "", fmt.Errorf("file cache dir is not configured")
	}
	cacheAbs, err := filepath.Abs(cacheDir)
	if err != nil {
		return "", err
	}
	ttsDir := filepath.Join(cacheAbs, "tts")
	if err := os.MkdirAll(ttsDir, 0o700); err != nil {
		return "", err
	}
	_ = os.Chmod(ttsDir, 0o700)

	sum := sha256.Sum256([]byte(text))
	base := fmt.Sprintf("voice_%d_%s", time.Now().UTC().Unix(), hex.EncodeToString(sum[:8]))
	wavPath := filepath.Join(ttsDir, base+".wav")
	oggPath := filepath.Join(ttsDir, base+".ogg")

	ttsLang := resolveTTSLang(lang, text)
	synthCmd := selectTTSCmd(ctx, wavPath, text, ttsLang)
	if synthCmd == nil {
		return "", fmt.Errorf("no local TTS engine found (install one of: pico2wave, espeak-ng, espeak, flite)")
	}
	out, err := synthCmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("tts synth failed: %w: %s", err, strings.TrimSpace(string(out)))
	}

	// Convert to OGG/Opus for Telegram voice.
	if commandExists("ffmpeg") {
		conv := exec.CommandContext(ctx, "ffmpeg", "-y", "-loglevel", "error", "-i", wavPath, "-c:a", "libopus", "-b:a", "24k", "-vbr", "on", "-compression_level", "10", oggPath)
		out, err := conv.CombinedOutput()
		if err != nil {
			return "", fmt.Errorf("ffmpeg convert failed: %w: %s", err, strings.TrimSpace(string(out)))
		}
	} else if commandExists("opusenc") {
		conv := exec.CommandContext(ctx, "opusenc", "--quiet", wavPath, oggPath)
		out, err := conv.CombinedOutput()
		if err != nil {
			return "", fmt.Errorf("opusenc convert failed: %w: %s", err, strings.TrimSpace(string(out)))
		}
	} else {
		return "", fmt.Errorf("no audio converter found (install ffmpeg or opusenc)")
	}

	_ = os.Remove(wavPath)
	_ = os.Chmod(oggPath, 0o600)
	return oggPath, nil
}
