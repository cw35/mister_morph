package llminspect

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/quailyquaily/mistermorph/llm"
)

type Options struct {
	Mode            string
	Task            string
	TimestampFormat string
	DumpDir         string
}

type PromptInspector struct {
	mu           sync.Mutex
	file         *os.File
	startedAt    time.Time
	mode         string
	task         string
	requestCount int
}

type modelSceneContextKey struct{}

const defaultModelScene = "unknown"

//go:embed tmpl/prompt.tmpl
var promptInspectorTemplateSource string

var promptInspectorTemplate = template.Must(template.New("prompt_inspector").Parse(promptInspectorTemplateSource))

type promptInspectorHeaderView struct {
	Mode     string
	Task     string
	Datetime string
}

type promptInspectorRequestView struct {
	RequestNumber int
	Scene         string
	Messages      []promptInspectorMessageView
}

type promptInspectorMessageView struct {
	Number        int
	Role          string
	HasToolCallID bool
	ToolCallID    string
	HasToolCalls  bool
	ToolCalls     string
	Content       string
}

func NewPromptInspector(opts Options) (*PromptInspector, error) {
	startedAt := time.Now()
	dumpDir := strings.TrimSpace(opts.DumpDir)
	if dumpDir == "" {
		dumpDir = "dump"
	}
	if err := os.MkdirAll(dumpDir, 0o755); err != nil {
		return nil, fmt.Errorf("create dump dir: %w", err)
	}
	path := filepath.Join(dumpDir, buildFilename("prompt", opts.Mode, startedAt, opts.TimestampFormat))
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open prompt dump file: %w", err)
	}
	inspector := &PromptInspector{
		file:      file,
		startedAt: startedAt,
		mode:      strings.TrimSpace(opts.Mode),
		task:      strings.TrimSpace(opts.Task),
	}
	if err := inspector.writeHeader(); err != nil {
		_ = file.Close()
		return nil, err
	}
	return inspector, nil
}

func (p *PromptInspector) Close() error {
	if p == nil || p.file == nil {
		return nil
	}
	return p.file.Close()
}

func WithModelScene(ctx context.Context, scene string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	scene = strings.TrimSpace(scene)
	if scene == "" {
		scene = defaultModelScene
	}
	return context.WithValue(ctx, modelSceneContextKey{}, scene)
}

func ModelSceneFromContext(ctx context.Context) string {
	if ctx == nil {
		return defaultModelScene
	}
	if v := ctx.Value(modelSceneContextKey{}); v != nil {
		if scene, ok := v.(string); ok {
			scene = strings.TrimSpace(scene)
			if scene != "" {
				return scene
			}
		}
	}
	return defaultModelScene
}

func (p *PromptInspector) Dump(messages []llm.Message) error {
	return p.DumpWithScene(defaultModelScene, messages)
}

func (p *PromptInspector) DumpWithScene(scene string, messages []llm.Message) error {
	if p == nil || p.file == nil {
		return nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()

	scene = strings.TrimSpace(scene)
	if scene == "" {
		scene = defaultModelScene
	}
	p.requestCount++

	view := promptInspectorRequestView{
		RequestNumber: p.requestCount,
		Scene:         scene,
		Messages:      make([]promptInspectorMessageView, 0, len(messages)),
	}
	for i, msg := range messages {
		mv := promptInspectorMessageView{
			Number:        i + 1,
			Role:          msg.Role,
			HasToolCallID: strings.TrimSpace(msg.ToolCallID) != "",
			ToolCallID:    msg.ToolCallID,
			Content:       msg.Content,
		}
		if len(msg.ToolCalls) > 0 {
			toolCallsJSON, err := json.MarshalIndent(msg.ToolCalls, "", "  ")
			if err != nil {
				mv.HasToolCalls = true
				mv.ToolCalls = fmt.Sprintf("<error: %s>", err.Error())
			} else {
				mv.HasToolCalls = true
				mv.ToolCalls = string(toolCallsJSON)
			}
		}
		view.Messages = append(view.Messages, mv)
	}

	var b strings.Builder
	if err := promptInspectorTemplate.ExecuteTemplate(&b, "request", view); err != nil {
		return fmt.Errorf("render prompt request dump: %w", err)
	}

	if _, err := p.file.WriteString(b.String()); err != nil {
		return err
	}
	return p.file.Sync()
}

func (p *PromptInspector) writeHeader() error {
	view := promptInspectorHeaderView{
		Mode:     strconv.Quote(p.mode),
		Task:     strconv.Quote(p.task),
		Datetime: strconv.Quote(p.startedAt.Format(time.RFC3339)),
	}
	var b strings.Builder
	if err := promptInspectorTemplate.ExecuteTemplate(&b, "header", view); err != nil {
		return fmt.Errorf("render prompt header dump: %w", err)
	}
	if _, err := p.file.WriteString(b.String()); err != nil {
		return err
	}
	return p.file.Sync()
}

type RequestInspector struct {
	mu        sync.Mutex
	file      *os.File
	startedAt time.Time
	mode      string
	task      string
	count     int
}

func NewRequestInspector(opts Options) (*RequestInspector, error) {
	startedAt := time.Now()
	dumpDir := strings.TrimSpace(opts.DumpDir)
	if dumpDir == "" {
		dumpDir = "dump"
	}
	if err := os.MkdirAll(dumpDir, 0o755); err != nil {
		return nil, fmt.Errorf("create dump dir: %w", err)
	}
	path := filepath.Join(dumpDir, buildFilename("request", opts.Mode, startedAt, opts.TimestampFormat))
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open request dump file: %w", err)
	}
	inspector := &RequestInspector{
		file:      file,
		startedAt: startedAt,
		mode:      strings.TrimSpace(opts.Mode),
		task:      strings.TrimSpace(opts.Task),
	}
	if err := inspector.writeHeader(); err != nil {
		_ = file.Close()
		return nil, err
	}
	return inspector, nil
}

func (r *RequestInspector) Close() error {
	if r == nil || r.file == nil {
		return nil
	}
	return r.file.Close()
}

func (r *RequestInspector) Dump(label, payload string) {
	if r == nil || r.file == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	r.count++
	var b strings.Builder
	fmt.Fprintf(&b, "\n## Event #%d\n\n", r.count)
	fmt.Fprintf(&b, "### %s\n\n", label)
	b.WriteString("```\n")
	b.WriteString(payload)
	if !strings.HasSuffix(payload, "\n") {
		b.WriteString("\n")
	}
	b.WriteString("```\n\n")

	_, _ = r.file.WriteString(b.String())
	_ = r.file.Sync()
}

func (r *RequestInspector) writeHeader() error {
	header := fmt.Sprintf(
		"---\nmode: %s\ntask: %s\ndatetime: %s\n---\n\n",
		strconv.Quote(r.mode),
		strconv.Quote(r.task),
		strconv.Quote(r.startedAt.Format(time.RFC3339)),
	)
	if _, err := r.file.WriteString(header); err != nil {
		return err
	}
	return r.file.Sync()
}

type PromptClient struct {
	Base      llm.Client
	Inspector *PromptInspector
}

func (c *PromptClient) Chat(ctx context.Context, req llm.Request) (llm.Result, error) {
	if c == nil || c.Base == nil {
		return llm.Result{}, fmt.Errorf("inspect client is not initialized")
	}
	if c.Inspector != nil {
		if err := c.Inspector.DumpWithScene(ModelSceneFromContext(ctx), req.Messages); err != nil {
			return llm.Result{}, err
		}
	}
	return c.Base.Chat(ctx, req)
}

func SetDebugHook(client llm.Client, dumpFn func(label, payload string)) error {
	setter, ok := client.(interface {
		SetDebugFn(func(label, payload string))
	})
	if !ok {
		return fmt.Errorf("client does not support debug hook")
	}
	setter.SetDebugFn(dumpFn)
	return nil
}

func buildFilename(kind string, mode string, t time.Time, tsFormat string) string {
	mode = strings.TrimSpace(mode)
	if tsFormat == "" {
		tsFormat = "20060102_1504"
	}
	ts := t.Format(tsFormat)
	if mode == "" {
		return fmt.Sprintf("%s_%s.md", kind, ts)
	}
	return fmt.Sprintf("%s_%s_%s.md", kind, mode, ts)
}
