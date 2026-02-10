package heartbeatutil

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/quailyquaily/mistermorph/agent"
	"github.com/quailyquaily/mistermorph/internal/statepaths"
	"github.com/quailyquaily/mistermorph/internal/todo"
	"github.com/quailyquaily/mistermorph/memory"
)

const (
	heartbeatFailureThreshold = 3
)

var heartbeatHTMLComment = regexp.MustCompile(`(?s)<!--.*?-->`)

func FormatFinalOutput(final *agent.Final) string {
	if final == nil {
		return ""
	}
	switch v := final.Output.(type) {
	case string:
		return strings.TrimSpace(v)
	default:
		b, _ := json.MarshalIndent(v, "", "  ")
		return strings.TrimSpace(string(b))
	}
}

func BuildHeartbeatTask(checklistPath string, memorySnapshot string) (string, bool, error) {
	checklist, empty, err := readHeartbeatChecklist(checklistPath)
	if err != nil {
		return "", true, err
	}

	var b strings.Builder
	b.WriteString("You are running a heartbeat checkpoint for the agent.\n")
	b.WriteString("Review the provided checklist and context. Always respond with a short summary of what you checked/did.\n")
	b.WriteString("If anything requires user attention or action, make that explicit in the summary.\n")
	b.WriteString("Do NOT output placeholders like HEARTBEAT_OK.\n")
	b.WriteString("Do NOT output mention it is a heartbeat.\n")
	b.WriteString("If the checklist is missing or empty, review recent short-term memory (if enabled) and current context to find things to do before summarizing.\n")
	b.WriteString("Prefer to resolve things yourself; avoid asking the user unless genuinely blocked.\n")
	b.WriteString("If the progress snapshot shows pending TODO items, treat that as needing attention: pick ONE pending item and take the smallest next step now (tools optional, but use them if needed).\n")
	b.WriteString("You MUST take at least one concrete action step before returning a final response when pending items exist. Do not only acknowledge pending items.\n")
	if !empty {
		b.WriteString("\nChecklist:\n")
		b.WriteString(checklist)
		b.WriteString("\n")
	}
	if strings.TrimSpace(memorySnapshot) != "" {
		b.WriteString("\nRecent memory progress:\n")
		b.WriteString(strings.TrimSpace(memorySnapshot))
		b.WriteString("\n")
	}

	return b.String(), empty, nil
}

func BuildHeartbeatMeta(source string, interval time.Duration, checklistPath string, checklistEmpty bool, state *State, extra map[string]any) map[string]any {
	hb := map[string]any{
		"source":           source,
		"scheduled_at_utc": time.Now().UTC().Format(time.RFC3339),
		"interval":         interval.String(),
	}
	if strings.TrimSpace(checklistPath) != "" {
		hb["checklist_path"] = checklistPath
	}
	if checklistEmpty {
		hb["checklist_empty"] = true
	}
	if state != nil {
		failures, lastSuccess, lastError, _ := state.Snapshot()
		if failures > 0 {
			hb["failures"] = failures
		}
		if !lastSuccess.IsZero() {
			hb["last_success_utc"] = lastSuccess.UTC().Format(time.RFC3339)
		}
		if strings.TrimSpace(lastError) != "" {
			hb["last_error"] = lastError
		}
	}
	for k, v := range extra {
		if strings.TrimSpace(k) == "" {
			continue
		}
		hb[k] = v
	}
	return map[string]any{
		"trigger":   "heartbeat",
		"heartbeat": hb,
	}
}

func BuildHeartbeatProgressSnapshot(mgr *memory.Manager, maxItems int) (string, error) {
	_ = mgr
	if maxItems <= 0 {
		maxItems = 50
	}

	store := todo.NewStore(statepaths.TODOWIPPath(), statepaths.TODODONEPath())
	list, err := store.List("wip")
	if err != nil {
		return "", err
	}
	if len(list.WIPItems) == 0 {
		return "", nil
	}
	items := list.WIPItems
	if len(items) > maxItems {
		items = items[:maxItems]
	}
	lines := make([]string, 0, len(items)+1)
	lines = append(lines, "[TODO:WIP:Progress]")
	for _, item := range items {
		createdAt := strings.TrimSpace(item.CreatedAt)
		content := strings.TrimSpace(item.Content)
		chatID := strings.TrimSpace(item.ChatID)
		line := fmt.Sprintf("- [ ] CreatedAt: %s - %s", createdAt, content)
		if chatID != "" {
			line = fmt.Sprintf("- [ ] CreatedAt: %s - ChatID: %s - %s", createdAt, chatID, content)
		}
		lines = append(lines, line)
	}
	return strings.TrimSpace(strings.Join(lines, "\n")), nil
}

type State struct {
	mu          sync.Mutex
	running     bool
	failures    int
	lastSuccess time.Time
	lastError   string
}

func (s *State) Start() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running {
		return false
	}
	s.running = true
	return true
}

func (s *State) EndSkipped() {
	s.mu.Lock()
	s.running = false
	s.mu.Unlock()
}

func (s *State) EndSuccess(now time.Time) {
	s.mu.Lock()
	s.running = false
	s.failures = 0
	s.lastError = ""
	s.lastSuccess = now
	s.mu.Unlock()
}

func (s *State) EndFailure(err error) (bool, string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.running = false
	s.failures++
	if err != nil {
		s.lastError = strings.TrimSpace(err.Error())
	}
	if s.failures >= heartbeatFailureThreshold {
		msg := "heartbeat_failed"
		if s.lastError != "" {
			msg = fmt.Sprintf("heartbeat_failed (%s)", s.lastError)
		}
		s.failures = 0
		return true, "ALERT: " + msg
	}
	return false, ""
}

func (s *State) Snapshot() (failures int, lastSuccess time.Time, lastError string, running bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.failures, s.lastSuccess, s.lastError, s.running
}

func readHeartbeatChecklist(path string) (string, bool, error) {
	path = strings.TrimSpace(path)
	if strings.TrimSpace(path) == "" {
		return "", true, nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", true, nil
		}
		return "", true, err
	}
	content := string(raw)
	if isChecklistEmptyContent(content) {
		return "", true, nil
	}
	return strings.TrimSpace(content), false, nil
}

func isChecklistEmptyContent(content string) bool {
	stripped := heartbeatHTMLComment.ReplaceAllString(content, "")
	lines := strings.Split(stripped, "\n")
	for _, line := range lines {
		l := strings.TrimSpace(line)
		if l == "" {
			continue
		}
		if strings.HasPrefix(l, "#") {
			continue
		}
		return false
	}
	return true
}
