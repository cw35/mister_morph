package todo

import (
	"bufio"
	"fmt"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type fileFrontmatter struct {
	CreatedAt string `yaml:"created_at"`
	UpdatedAt string `yaml:"updated_at"`
	OpenCount *int   `yaml:"open_count,omitempty"`
	DoneCount *int   `yaml:"done_count,omitempty"`
}

func ParseWIP(raw string) (WIPFile, error) {
	fm, body, _, err := parseFrontmatter(raw)
	if err != nil {
		return WIPFile{}, err
	}
	lines := parseEntryLines(body, false)
	out := WIPFile{
		CreatedAt: strings.TrimSpace(fm.CreatedAt),
		UpdatedAt: strings.TrimSpace(fm.UpdatedAt),
		Entries:   lines,
	}
	if fm.OpenCount != nil {
		out.OpenCount = *fm.OpenCount
	}
	out.OpenCount = len(out.Entries)
	return out, nil
}

func ParseDONE(raw string) (DONEFile, error) {
	fm, body, _, err := parseFrontmatter(raw)
	if err != nil {
		return DONEFile{}, err
	}
	lines := parseEntryLines(body, true)
	out := DONEFile{
		CreatedAt: strings.TrimSpace(fm.CreatedAt),
		UpdatedAt: strings.TrimSpace(fm.UpdatedAt),
		Entries:   lines,
	}
	if fm.DoneCount != nil {
		out.DoneCount = *fm.DoneCount
	}
	out.DoneCount = len(out.Entries)
	return out, nil
}

func RenderWIP(file WIPFile) string {
	file.OpenCount = len(file.Entries)
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString(`created_at: "`)
	b.WriteString(strings.TrimSpace(file.CreatedAt))
	b.WriteString("\"\n")
	b.WriteString(`updated_at: "`)
	b.WriteString(strings.TrimSpace(file.UpdatedAt))
	b.WriteString("\"\n")
	b.WriteString("open_count: ")
	b.WriteString(strconv.Itoa(file.OpenCount))
	b.WriteString("\n")
	b.WriteString("---\n\n")
	b.WriteString(HeaderWIP)
	b.WriteString("\n\n")
	for _, item := range file.Entries {
		line := renderWIPEntryLine(item)
		if strings.TrimSpace(line) == "" {
			continue
		}
		b.WriteString(line)
		b.WriteString("\n")
	}
	return b.String()
}

func RenderDONE(file DONEFile) string {
	file.DoneCount = len(file.Entries)
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString(`created_at: "`)
	b.WriteString(strings.TrimSpace(file.CreatedAt))
	b.WriteString("\"\n")
	b.WriteString(`updated_at: "`)
	b.WriteString(strings.TrimSpace(file.UpdatedAt))
	b.WriteString("\"\n")
	b.WriteString("done_count: ")
	b.WriteString(strconv.Itoa(file.DoneCount))
	b.WriteString("\n")
	b.WriteString("---\n\n")
	b.WriteString(HeaderDONE)
	b.WriteString("\n\n")
	for _, item := range file.Entries {
		line := renderDONEEntryLine(item)
		if strings.TrimSpace(line) == "" {
			continue
		}
		b.WriteString(line)
		b.WriteString("\n")
	}
	return b.String()
}

func ParseEntryFromInput(raw string, now string) (Entry, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return Entry{}, fmt.Errorf("content is required")
	}
	if done, ok := parseDONEEntryLine(raw); ok {
		done.Done = false
		done.DoneAt = ""
		done.CreatedAt = strings.TrimSpace(now)
		return done, nil
	}
	if wip, ok := parseWIPEntryLine(raw); ok {
		wip.Done = false
		wip.DoneAt = ""
		wip.CreatedAt = strings.TrimSpace(now)
		return wip, nil
	}
	raw = strings.TrimPrefix(raw, "- [ ]")
	raw = strings.TrimPrefix(raw, "- [x]")
	raw = strings.TrimPrefix(raw, "- [X]")
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "CreatedAt:") {
		parts := strings.SplitN(raw, " - ", 2)
		if len(parts) == 2 {
			raw = strings.TrimSpace(parts[1])
		}
	}
	if raw == "" {
		return Entry{}, fmt.Errorf("content is required")
	}
	chatID := ""
	if parsedChatID, parsedContent, ok := splitChatIDPrefix(raw); ok {
		chatID = parsedChatID
		raw = parsedContent
	}
	return Entry{
		Done:      false,
		CreatedAt: strings.TrimSpace(now),
		ChatID:    chatID,
		Content:   raw,
	}, nil
}

func parseFrontmatter(raw string) (fileFrontmatter, string, bool, error) {
	sc := bufio.NewScanner(strings.NewReader(raw))
	if !sc.Scan() {
		return fileFrontmatter{}, raw, false, nil
	}
	if strings.TrimSpace(sc.Text()) != "---" {
		return fileFrontmatter{}, raw, false, nil
	}
	var yamlLines []string
	var bodyLines []string
	foundEnd := false
	for sc.Scan() {
		line := sc.Text()
		if !foundEnd {
			if strings.TrimSpace(line) == "---" {
				foundEnd = true
				continue
			}
			yamlLines = append(yamlLines, line)
			continue
		}
		bodyLines = append(bodyLines, line)
	}
	if !foundEnd {
		return fileFrontmatter{}, raw, false, nil
	}
	var fm fileFrontmatter
	if err := yaml.Unmarshal([]byte(strings.Join(yamlLines, "\n")), &fm); err != nil {
		return fileFrontmatter{}, strings.Join(bodyLines, "\n"), false, nil
	}
	return fm, strings.Join(bodyLines, "\n"), true, nil
}

func parseEntryLines(body string, done bool) []Entry {
	if strings.TrimSpace(body) == "" {
		return nil
	}
	lines := strings.Split(body, "\n")
	out := make([]Entry, 0, len(lines))
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#") {
			continue
		}
		if done {
			if item, ok := parseDONEEntryLine(line); ok {
				out = append(out, item)
			}
			continue
		}
		if item, ok := parseWIPEntryLine(line); ok {
			out = append(out, item)
		}
	}
	return out
}

func parseWIPEntryLine(line string) (Entry, bool) {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "- [ ] CreatedAt: ") {
		return Entry{}, false
	}
	rest := strings.TrimPrefix(line, "- [ ] CreatedAt: ")
	parts := strings.SplitN(rest, " - ", 2)
	if len(parts) != 2 {
		return Entry{}, false
	}
	createdAt := strings.TrimSpace(parts[0])
	content := strings.TrimSpace(parts[1])
	if !validTimestamp(createdAt) || content == "" {
		return Entry{}, false
	}
	chatID := ""
	if parsedChatID, parsedContent, ok := splitChatIDPrefix(content); ok {
		chatID = parsedChatID
		content = parsedContent
	}
	return Entry{
		Done:      false,
		CreatedAt: createdAt,
		ChatID:    chatID,
		Content:   content,
	}, true
}

func parseDONEEntryLine(line string) (Entry, bool) {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "- [x] CreatedAt: ") {
		return Entry{}, false
	}
	rest := strings.TrimPrefix(line, "- [x] CreatedAt: ")
	parts := strings.SplitN(rest, ", DoneAt: ", 2)
	if len(parts) != 2 {
		return Entry{}, false
	}
	createdAt := strings.TrimSpace(parts[0])
	parts = strings.SplitN(parts[1], " - ", 2)
	if len(parts) != 2 {
		return Entry{}, false
	}
	doneAt := strings.TrimSpace(parts[0])
	content := strings.TrimSpace(parts[1])
	if !validTimestamp(createdAt) || !validTimestamp(doneAt) || content == "" {
		return Entry{}, false
	}
	chatID := ""
	if parsedChatID, parsedContent, ok := splitChatIDPrefix(content); ok {
		chatID = parsedChatID
		content = parsedContent
	}
	return Entry{
		Done:      true,
		CreatedAt: createdAt,
		DoneAt:    doneAt,
		ChatID:    chatID,
		Content:   content,
	}, true
}

func renderWIPEntryLine(item Entry) string {
	content := strings.TrimSpace(item.Content)
	createdAt := strings.TrimSpace(item.CreatedAt)
	if content == "" || !validTimestamp(createdAt) {
		return ""
	}
	chatID := normalizeEntryChatID(item.ChatID)
	if chatID != "" {
		return "- [ ] CreatedAt: " + createdAt + " - ChatID: " + chatID + " - " + content
	}
	return "- [ ] CreatedAt: " + createdAt + " - " + content
}

func renderDONEEntryLine(item Entry) string {
	content := strings.TrimSpace(item.Content)
	createdAt := strings.TrimSpace(item.CreatedAt)
	doneAt := strings.TrimSpace(item.DoneAt)
	if content == "" || !validTimestamp(createdAt) || !validTimestamp(doneAt) {
		return ""
	}
	chatID := normalizeEntryChatID(item.ChatID)
	if chatID != "" {
		return "- [x] CreatedAt: " + createdAt + ", DoneAt: " + doneAt + " - ChatID: " + chatID + " - " + content
	}
	return "- [x] CreatedAt: " + createdAt + ", DoneAt: " + doneAt + " - " + content
}

func splitChatIDPrefix(content string) (string, string, bool) {
	content = strings.TrimSpace(content)
	lower := strings.ToLower(content)
	prefix := ""
	switch {
	case strings.HasPrefix(lower, "chatid:"):
		prefix = "chatid:"
	case strings.HasPrefix(lower, "channel:"):
		// Keep backward compatibility with older TODO files.
		prefix = "channel:"
	default:
		return "", content, false
	}
	rest := strings.TrimSpace(content[len(prefix):])
	parts := strings.SplitN(rest, " - ", 2)
	if len(parts) != 2 {
		return "", content, false
	}
	chatID := normalizeEntryChatID(parts[0])
	body := strings.TrimSpace(parts[1])
	if body == "" || !isValidTODOChatID(chatID) {
		return "", content, false
	}
	return chatID, body, true
}

// splitChannelPrefix is kept for backward compatibility in older call sites.
func splitChannelPrefix(content string) (string, string, bool) {
	return splitChatIDPrefix(content)
}
