package memory

import (
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
)

const (
	sectionSessionSummary = "Session Summary"
	sectionTemporaryFacts = "Temporary Facts"
	sectionTasks          = "Tasks"
	sectionFollowUps      = "Follow Ups"
	sectionRelatedLinks   = "Related Links"

	sectionLongGoals = "Long-Term Goals / Projects"
	sectionLongFacts = "Key Facts"

	addedStampPrefix = "(added "
)

func ParseShortTermContent(body string) ShortTermContent {
	sections := splitSections(body)
	return ShortTermContent{
		SessionSummary: parseKVSection(sections[sectionSessionSummary]),
		TemporaryFacts: parseKVSection(sections[sectionTemporaryFacts]),
		Tasks:          parseTodoSection(sections[sectionTasks]),
		FollowUps:      parseTodoSection(sections[sectionFollowUps]),
		RelatedLinks:   parseLinkSection(sections[sectionRelatedLinks]),
	}
}

func ParseLongTermContent(body string) LongTermContent {
	sections := splitSections(body)
	return LongTermContent{
		Goals: parseKVSection(sections[sectionLongGoals]),
		Facts: parseKVSection(sections[sectionLongFacts]),
	}
}

func MergeShortTerm(existing ShortTermContent, draft SessionDraft) ShortTermContent {
	incomingSummary := normalizeKVItems(draft.SessionSummary)
	incomingFacts := normalizeKVItems(draft.TemporaryFacts)
	incomingTasks := normalizeTasks(draft.Tasks)
	incomingFollowUps := normalizeTasks(draft.FollowUps)

	merged := ShortTermContent{
		SessionSummary: MergeSessionSummary(existing.SessionSummary, incomingSummary),
		TemporaryFacts: MergeTemporaryFacts(existing.TemporaryFacts, incomingFacts),
		Tasks:          mergeTasks(existing.Tasks, incomingTasks),
		FollowUps:      mergeTasks(existing.FollowUps, incomingFollowUps),
		RelatedLinks:   mergeLinks(existing.RelatedLinks, nil),
	}
	return NormalizeShortTermContent(merged)
}

// MergeSessionSummary merges session summary items with fuzzy title matching.
func MergeSessionSummary(existing []KVItem, incoming []KVItem) []KVItem {
	merged := mergeKV(existing, incoming)
	return mergeSimilarSessionSummary(merged)
}

// MergeTemporaryFacts merges temporary facts items with fuzzy title matching and URL overlap.
func MergeTemporaryFacts(existing []KVItem, incoming []KVItem) []KVItem {
	merged := mergeKV(existing, incoming)
	return mergeSimilarTemporaryFacts(merged)
}

func MergeLongTerm(existing LongTermContent, draft PromoteDraft, now time.Time) LongTermContent {
	incomingGoals := normalizeKVItems(draft.GoalsProjects)
	incomingFacts := normalizeKVItems(draft.KeyFacts)
	date := ""
	if !now.IsZero() {
		date = now.UTC().Format("2006-01-02")
	}
	return LongTermContent{
		Goals: mergeLongTermKV(existing.Goals, incomingGoals, date),
		Facts: mergeLongTermKV(existing.Facts, incomingFacts, date),
	}
}

// NormalizeShortTermContent deduplicates repeated key lines within session summary
// and temporary facts values after merges.
func NormalizeShortTermContent(content ShortTermContent) ShortTermContent {
	if len(content.SessionSummary) > 0 {
		out := make([]KVItem, 0, len(content.SessionSummary))
		for _, it := range content.SessionSummary {
			val := dedupeSessionSummaryValue(it.Value)
			out = append(out, KVItem{Title: it.Title, Value: val})
		}
		content.SessionSummary = out
	}
	if len(content.TemporaryFacts) > 0 {
		out := make([]KVItem, 0, len(content.TemporaryFacts))
		for _, it := range content.TemporaryFacts {
			val := dedupeKVValueLines(it.Value)
			out = append(out, KVItem{Title: it.Title, Value: val})
		}
		content.TemporaryFacts = out
	}
	return content
}

func BuildShortTermBody(date string, content ShortTermContent) string {
	var b strings.Builder
	b.WriteString("# ")
	b.WriteString(date)
	b.WriteString(" Short-Term Memory\n\n")

	writeShortTermKVSection(&b, sectionSessionSummary, content.SessionSummary)
	writeShortTermKVSection(&b, sectionTemporaryFacts, content.TemporaryFacts)
	writeTodoSection(&b, sectionTasks, content.Tasks)
	writeTodoSection(&b, sectionFollowUps, content.FollowUps)
	writeLinkSection(&b, sectionRelatedLinks, content.RelatedLinks)
	return strings.TrimSpace(b.String()) + "\n"
}

func BuildLongTermBody(content LongTermContent) string {
	var b strings.Builder
	b.WriteString("# Long-Term Memory\n\n")
	writeKVSection(&b, sectionLongGoals, content.Goals)
	writeKVSection(&b, sectionLongFacts, content.Facts)
	return strings.TrimSpace(b.String()) + "\n"
}

func splitSections(body string) map[string][]string {
	sections := make(map[string][]string)
	var current string
	lines := strings.Split(body, "\n")
	for _, line := range lines {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "## ") {
			current = strings.TrimSpace(strings.TrimPrefix(trim, "## "))
			if _, ok := sections[current]; !ok {
				sections[current] = nil
			}
			continue
		}
		if current == "" {
			continue
		}
		if trim == "" {
			continue
		}
		sections[current] = append(sections[current], trim)
	}
	return sections
}

func parseKVSection(lines []string) []KVItem {
	items := make([]KVItem, 0, len(lines))
	var currentTitle string
	var currentLines []string

	flush := func() {
		if strings.TrimSpace(currentTitle) == "" && len(currentLines) == 0 {
			currentTitle = ""
			currentLines = nil
			return
		}
		value := strings.TrimSpace(strings.Join(currentLines, "\n"))
		items = append(items, KVItem{Title: strings.TrimSpace(currentTitle), Value: value})
		currentTitle = ""
		currentLines = nil
	}

	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}

		if title, inline, ok := parseNumberedKVLine(line); ok {
			flush()
			currentTitle = title
			if inline != "" {
				currentLines = append(currentLines, inline)
			}
			continue
		}

		if item, ok := parseKVLine(line); ok {
			flush()
			items = append(items, item)
			continue
		}

		if strings.HasPrefix(line, "-") && currentTitle != "" {
			sub := strings.TrimSpace(strings.TrimPrefix(line, "-"))
			if sub != "" {
				currentLines = append(currentLines, sub)
			}
			continue
		}

		if currentTitle != "" {
			currentLines = append(currentLines, line)
		}
	}
	flush()

	out := make([]KVItem, 0, len(items))
	seen := map[string]bool{}
	for _, item := range items {
		key := strings.ToLower(strings.TrimSpace(item.Title))
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, item)
	}
	return out
}

func parseTodoSection(lines []string) []TaskItem {
	items := make([]TaskItem, 0, len(lines))
	seen := map[string]bool{}
	for _, line := range lines {
		item, ok := parseTodoLine(line)
		if !ok {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(item.Text))
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		items = append(items, item)
	}
	return items
}

func parseLinkSection(lines []string) []LinkItem {
	items := make([]LinkItem, 0, len(lines))
	seen := map[string]bool{}
	for _, line := range lines {
		item, ok := parseLinkLine(line)
		if !ok {
			continue
		}
		key := strings.TrimSpace(item.Target)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		items = append(items, item)
	}
	return items
}

func parseKVLine(line string) (KVItem, bool) {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "- **") {
		return KVItem{}, false
	}
	rest := strings.TrimPrefix(line, "- **")
	idx := strings.Index(rest, "**")
	if idx < 0 {
		return KVItem{}, false
	}
	title := strings.TrimSpace(rest[:idx])
	after := strings.TrimSpace(rest[idx+2:])
	if strings.HasPrefix(after, ":") {
		after = strings.TrimSpace(strings.TrimPrefix(after, ":"))
	}
	if title == "" && after == "" {
		return KVItem{}, false
	}
	return KVItem{Title: title, Value: after}, true
}

func parseNumberedKVLine(line string) (string, string, bool) {
	line = strings.TrimSpace(line)
	if line == "" {
		return "", "", false
	}
	dot := strings.Index(line, ".")
	if dot <= 0 {
		return "", "", false
	}
	for _, r := range line[:dot] {
		if r < '0' || r > '9' {
			return "", "", false
		}
	}
	rest := strings.TrimSpace(line[dot+1:])
	if rest == "" {
		return "", "", false
	}
	if strings.HasPrefix(rest, "**") {
		end := strings.Index(rest[2:], "**")
		if end < 0 {
			return "", "", false
		}
		label := strings.TrimSpace(rest[2 : 2+end])
		after := strings.TrimSpace(rest[2+end+2:])
		if strings.HasPrefix(after, ":") {
			after = strings.TrimSpace(strings.TrimPrefix(after, ":"))
		}
		if strings.EqualFold(label, "topic") {
			if after == "" {
				return "", "", false
			}
			return after, "", true
		}
		if label == "" {
			return "", "", false
		}
		return label, strings.TrimSpace(after), true
	}

	restLower := strings.ToLower(rest)
	if strings.HasPrefix(restLower, "topic") {
		after := strings.TrimSpace(rest[len("topic"):])
		if strings.HasPrefix(after, ":") {
			after = strings.TrimSpace(strings.TrimPrefix(after, ":"))
		}
		if after == "" {
			return "", "", false
		}
		return after, "", true
	}

	parts := strings.SplitN(rest, ":", 2)
	title := strings.TrimSpace(parts[0])
	if title == "" {
		return "", "", false
	}
	inline := ""
	if len(parts) > 1 {
		inline = strings.TrimSpace(parts[1])
	}
	return title, inline, true
}

func parseTodoLine(line string) (TaskItem, bool) {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "- [") {
		return TaskItem{}, false
	}
	rest := strings.TrimPrefix(line, "- [")
	idx := strings.Index(rest, "]")
	if idx < 0 {
		return TaskItem{}, false
	}
	status := strings.ToLower(strings.TrimSpace(rest[:idx]))
	text := strings.TrimSpace(rest[idx+1:])
	if text == "" {
		return TaskItem{}, false
	}
	done := status == "x"
	return TaskItem{Text: text, Done: done}, true
}

func parseLinkLine(line string) (LinkItem, bool) {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "-") {
		return LinkItem{}, false
	}
	start := strings.Index(line, "[")
	mid := strings.Index(line, "](")
	end := strings.LastIndex(line, ")")
	if start < 0 || mid < 0 || end < 0 || end <= mid+1 {
		return LinkItem{}, false
	}
	text := strings.TrimSpace(line[start+1 : mid])
	target := strings.TrimSpace(line[mid+2 : end])
	if target == "" {
		return LinkItem{}, false
	}
	if text == "" {
		text = target
	}
	return LinkItem{Text: text, Target: target}, true
}

func normalizeKVItems(items []KVItem) []KVItem {
	out := make([]KVItem, 0, len(items))
	seen := map[string]bool{}
	for _, it := range items {
		title := strings.TrimSpace(it.Title)
		value := strings.TrimSpace(it.Value)
		if title == "" && value == "" {
			continue
		}
		if title == "" {
			title = "Item"
		}
		key := strings.ToLower(title)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, KVItem{Title: title, Value: value})
	}
	return out
}

func normalizeTasks(items []TaskItem) []TaskItem {
	out := make([]TaskItem, 0, len(items))
	seen := map[string]bool{}
	for _, it := range items {
		text := strings.TrimSpace(it.Text)
		if text == "" {
			continue
		}
		key := strings.ToLower(text)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, TaskItem{Text: text, Done: it.Done})
	}
	return out
}

func mergeKV(existing []KVItem, incoming []KVItem) []KVItem {
	if len(incoming) == 0 {
		return existing
	}
	order := make([]KVItem, 0, len(existing)+len(incoming))
	index := map[string]int{}
	for _, it := range existing {
		key := strings.ToLower(strings.TrimSpace(it.Title))
		if key == "" {
			continue
		}
		index[key] = len(order)
		order = append(order, it)
	}
	for _, it := range incoming {
		key := strings.ToLower(strings.TrimSpace(it.Title))
		if key == "" {
			continue
		}
		if idx, ok := index[key]; ok {
			order[idx] = it
			continue
		}
		index[key] = len(order)
		order = append(order, it)
	}
	return order
}

func mergeSimilarSessionSummary(items []KVItem) []KVItem {
	out := make([]KVItem, 0, len(items))
	for _, it := range items {
		title := strings.TrimSpace(it.Title)
		if title == "" {
			continue
		}
		merged := false
		for i := range out {
			if titlesSimilar(out[i].Title, title) {
				out[i].Value = mergeSessionSummaryValue(out[i].Value, it.Value)
				merged = true
				break
			}
		}
		if !merged {
			out = append(out, it)
		}
	}
	return out
}

func mergeSimilarTemporaryFacts(items []KVItem) []KVItem {
	out := make([]KVItem, 0, len(items))
	for _, it := range items {
		title := strings.TrimSpace(it.Title)
		if title == "" {
			continue
		}
		merged := false
		for i := range out {
			if titlesSimilar(out[i].Title, title) || urlsOverlap(out[i].Value, it.Value) {
				out[i].Value = mergeKVValueLines(out[i].Value, it.Value)
				merged = true
				break
			}
		}
		if !merged {
			out = append(out, it)
		}
	}
	return out
}

func titlesSimilar(a string, b string) bool {
	na := normalizeTitleForMerge(a)
	nb := normalizeTitleForMerge(b)
	if na == "" || nb == "" {
		return false
	}
	if na == nb {
		return true
	}
	if strings.Contains(na, nb) || strings.Contains(nb, na) {
		return true
	}
	return titleSimilarity(na, nb) >= 0.82
}

func normalizeTitleForMerge(title string) string {
	title = strings.ToLower(strings.TrimSpace(title))
	if title == "" {
		return ""
	}
	for _, token := range []string{
		"latest", "local", "source", "sources", "list", "lists", "website", "websites", "site", "sites", "links", "link", "entry", "query", "lookup",
		"\u6700\u65b0", "\u672c\u5730", "\u6765\u6e90", "\u5217\u8868", "\u7f51\u7ad9", "\u5165\u53e3", "\u67e5\u8be2", "\u67e5\u770b", "\u94fe\u63a5",
	} {
		title = strings.ReplaceAll(title, token, "")
	}
	var b strings.Builder
	for _, r := range title {
		switch {
		case unicode.IsLetter(r):
			b.WriteRune(r)
		case unicode.IsNumber(r):
			b.WriteRune(r)
		}
	}
	return b.String()
}

func titleSimilarity(a string, b string) float64 {
	if a == "" || b == "" {
		return 0
	}
	ga := bigrams([]rune(a))
	gb := bigrams([]rune(b))
	if len(ga) == 0 || len(gb) == 0 {
		if a == b {
			return 1
		}
		return 0
	}
	inter := 0
	union := len(ga)
	seen := map[string]bool{}
	for k := range ga {
		seen[k] = true
	}
	for k := range gb {
		if seen[k] {
			inter++
		} else {
			union++
		}
	}
	if union == 0 {
		return 0
	}
	return float64(inter) / float64(union)
}

func bigrams(runes []rune) map[string]bool {
	out := map[string]bool{}
	if len(runes) < 2 {
		if len(runes) == 1 {
			out[string(runes[0])] = true
		}
		return out
	}
	for i := 0; i < len(runes)-1; i++ {
		out[string(runes[i:i+2])] = true
	}
	return out
}

var urlRegexp = regexp.MustCompile(`https?://[^\s)]+`)

func urlsOverlap(a string, b string) bool {
	ua := extractURLs(a)
	if len(ua) == 0 {
		return false
	}
	ub := extractURLs(b)
	if len(ub) == 0 {
		return false
	}
	seen := map[string]bool{}
	for _, u := range ua {
		seen[u] = true
	}
	for _, u := range ub {
		if seen[u] {
			return true
		}
	}
	return false
}

func extractURLs(text string) []string {
	matches := urlRegexp.FindAllString(text, -1)
	if len(matches) == 0 {
		return nil
	}
	out := make([]string, 0, len(matches))
	seen := map[string]bool{}
	for _, m := range matches {
		u := strings.TrimRight(m, ".,;:)]\"'")
		u = strings.TrimSpace(u)
		if u == "" {
			continue
		}
		if seen[u] {
			continue
		}
		seen[u] = true
		out = append(out, u)
	}
	return out
}

func mergeSessionSummaryValue(existing string, incoming string) string {
	existing = strings.TrimSpace(existing)
	incoming = strings.TrimSpace(incoming)
	if existing == "" {
		return incoming
	}
	if incoming == "" {
		return existing
	}
	existingMap, existingOrder := parseSummaryLines(existing)
	incomingMap, _ := parseSummaryLines(incoming)
	merged := make(map[string]string, len(existingMap)+len(incomingMap))
	order := append([]string{}, existingOrder...)
	for k, v := range existingMap {
		merged[k] = v
	}
	for k, v := range incomingMap {
		if _, ok := merged[k]; !ok {
			order = append(order, k)
		}
		merged[k] = mergeSummaryField(k, merged[k], v)
	}
	if len(order) == 0 {
		return mergeKVValueLines(existing, incoming)
	}
	var b strings.Builder
	for _, k := range order {
		val := strings.TrimSpace(merged[k])
		if val == "" {
			continue
		}
		b.WriteString(k)
		b.WriteString(": ")
		b.WriteString(val)
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String())
}

func parseSummaryLines(value string) (map[string]string, []string) {
	lines := strings.Split(value, "\n")
	out := map[string]string{}
	order := make([]string, 0, len(lines))
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		key, val, ok := splitSummaryLine(line)
		if !ok {
			continue
		}
		if _, seen := out[key]; !seen {
			order = append(order, key)
		}
		out[key] = val
	}
	return out, order
}

func dedupeSessionSummaryValue(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return value
	}
	lines := strings.Split(value, "\n")
	merged := make(map[string]string, len(lines))
	order := make([]string, 0, len(lines))
	var extras []string
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		key, val, ok := splitSummaryLine(line)
		if !ok {
			extras = append(extras, line)
			continue
		}
		if _, seen := merged[key]; !seen {
			order = append(order, key)
		}
		merged[key] = mergeSummaryField(key, merged[key], val)
	}
	if len(order) == 0 {
		return strings.TrimSpace(strings.Join(extras, "\n"))
	}
	var b strings.Builder
	for _, k := range order {
		val := strings.TrimSpace(merged[k])
		if val == "" {
			continue
		}
		b.WriteString(k)
		b.WriteString(": ")
		b.WriteString(val)
		b.WriteString("\n")
	}
	for _, extra := range extras {
		if strings.TrimSpace(extra) == "" {
			continue
		}
		b.WriteString(extra)
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String())
}

func splitSummaryLine(line string) (string, string, bool) {
	if line == "" {
		return "", "", false
	}
	sep := strings.IndexAny(line, ":：")
	if sep <= 0 {
		return "", "", false
	}
	key := strings.TrimSpace(line[:sep])
	val := strings.TrimSpace(line[sep+1:])
	if key == "" {
		return "", "", false
	}
	key = normalizeSummaryKey(key)
	return key, val, true
}

func normalizeSummaryKey(key string) string {
	k := strings.ToLower(strings.TrimSpace(key))
	switch k {
	case "users", "\u7528\u6237", "\u4f7f\u7528\u8005":
		return "Users"
	case "datetime", "time", "\u65f6\u95f4", "\u65e5\u671f":
		return "Datetime"
	case "event", "\u4e8b\u4ef6":
		return "Event"
	case "result", "\u7ed3\u679c":
		return "Result"
	default:
		return strings.TrimSpace(key)
	}
}

func mergeSummaryField(key string, existing string, incoming string) string {
	existing = strings.TrimSpace(existing)
	incoming = strings.TrimSpace(incoming)
	if incoming == "" {
		return existing
	}
	if existing == "" {
		return incoming
	}
	switch key {
	case "Users":
		return mergeCSV(existing, incoming)
	case "Datetime":
		if incoming > existing {
			return incoming
		}
		return existing
	case "Result":
		if strings.Contains(existing, "http") || strings.Contains(incoming, "http") {
			return mergeKVValueLines(existing, incoming)
		}
		if len(incoming) >= len(existing) {
			return incoming
		}
		return existing
	case "Event":
		if len(incoming) >= len(existing) {
			return incoming
		}
		return existing
	default:
		if len(incoming) >= len(existing) {
			return incoming
		}
		return existing
	}
}

func mergeCSV(a string, b string) string {
	parts := strings.FieldsFunc(a+","+b, func(r rune) bool {
		return r == ',' || r == '，' || r == ';' || r == '；'
	})
	seen := map[string]bool{}
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	return strings.Join(out, ", ")
}

func mergeKVValueLines(a string, b string) string {
	lines := append(parseValueLines(a), parseValueLines(b)...)
	seen := map[string]bool{}
	seenURL := map[string]bool{}
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		key := strings.ToLower(strings.TrimSpace(line.Key))
		if key == "" {
			continue
		}
		if seen[key] {
			continue
		}
		if line.URL != "" && seenURL[line.URL] {
			continue
		}
		seen[key] = true
		if line.URL != "" {
			seenURL[line.URL] = true
		}
		out = append(out, line.Text)
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

type valueLine struct {
	Text string
	Key  string
	URL  string
}

func parseValueLines(val string) []valueLine {
	val = strings.TrimSpace(val)
	if val == "" {
		return nil
	}
	rawLines := strings.Split(val, "\n")
	out := make([]valueLine, 0, len(rawLines))
	for _, raw := range rawLines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		plain := strings.TrimLeft(line, "-*• ")
		plain = strings.TrimSpace(plain)
		if plain == "" {
			continue
		}
		urls := extractURLs(plain)
		url := ""
		if len(urls) > 0 {
			url = urls[0]
		}
		text := line
		if url != "" && !strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "*") && !strings.HasPrefix(line, "•") {
			text = "- " + plain
		}
		out = append(out, valueLine{Text: text, Key: plain, URL: url})
	}
	return out
}

func dedupeKVValueLines(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return value
	}
	lines := parseValueLines(value)
	seen := map[string]bool{}
	seenURL := map[string]bool{}
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		key := strings.ToLower(strings.TrimSpace(line.Key))
		if key == "" {
			continue
		}
		if seen[key] {
			continue
		}
		if line.URL != "" && seenURL[line.URL] {
			continue
		}
		seen[key] = true
		if line.URL != "" {
			seenURL[line.URL] = true
		}
		out = append(out, line.Text)
	}
	if len(out) == 0 {
		return value
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

func mergeLongTermKV(existing []KVItem, incoming []KVItem, date string) []KVItem {
	if len(incoming) == 0 {
		return existing
	}
	order := make([]KVItem, 0, len(existing)+len(incoming))
	index := map[string]int{}
	stamps := map[string]string{}
	for _, it := range existing {
		key := strings.ToLower(strings.TrimSpace(it.Title))
		if key == "" {
			continue
		}
		index[key] = len(order)
		order = append(order, it)
		if stamp, ok := extractAddedStamp(it.Value); ok {
			stamps[key] = stamp
		}
	}
	for _, it := range incoming {
		key := strings.ToLower(strings.TrimSpace(it.Title))
		if key == "" {
			continue
		}
		it.Title = strings.TrimSpace(it.Title)
		it.Value = strings.TrimSpace(it.Value)
		if idx, ok := index[key]; ok {
			if it.Value == "" {
				it.Value = strings.TrimSpace(order[idx].Value)
			}
			if !hasAddedStamp(it.Value) {
				if stamp, ok := stamps[key]; ok {
					it.Value = appendAddedStamp(it.Value, stamp)
				}
			}
			order[idx] = it
			continue
		}
		if !hasAddedStamp(it.Value) && strings.TrimSpace(date) != "" {
			it.Value = appendAddedStamp(it.Value, formatAddedStamp(date))
		}
		index[key] = len(order)
		order = append(order, it)
	}
	return order
}

func formatAddedStamp(date string) string {
	date = strings.TrimSpace(date)
	if date == "" {
		return ""
	}
	return addedStampPrefix + date + ")"
}

func appendAddedStamp(value string, stamp string) string {
	value = strings.TrimSpace(value)
	stamp = strings.TrimSpace(stamp)
	if stamp == "" || hasAddedStamp(value) {
		return value
	}
	if value == "" {
		return stamp
	}
	return value + " " + stamp
}

func hasAddedStamp(value string) bool {
	_, ok := extractAddedStamp(value)
	return ok
}

func extractAddedStamp(value string) (string, bool) {
	trimmed := strings.TrimSpace(value)
	lower := strings.ToLower(trimmed)
	if !strings.HasSuffix(lower, ")") {
		return "", false
	}
	prefix := strings.ToLower(addedStampPrefix)
	idx := strings.LastIndex(lower, prefix)
	if idx < 0 {
		return "", false
	}
	stamp := strings.TrimSpace(trimmed[idx:])
	if !strings.HasPrefix(strings.ToLower(stamp), prefix) || !strings.HasSuffix(stamp, ")") {
		return "", false
	}
	inner := strings.TrimSpace(stamp[len(addedStampPrefix) : len(stamp)-1])
	if !isDateYYYYMMDD(inner) {
		return "", false
	}
	return stamp, true
}

func isDateYYYYMMDD(val string) bool {
	if len(val) != 10 {
		return false
	}
	for i, r := range val {
		switch i {
		case 4, 7:
			if r != '-' {
				return false
			}
		default:
			if r < '0' || r > '9' {
				return false
			}
		}
	}
	return true
}

func mergeTasks(existing []TaskItem, incoming []TaskItem) []TaskItem {
	if len(incoming) == 0 {
		return existing
	}
	order := make([]TaskItem, 0, len(existing)+len(incoming))
	index := map[string]int{}
	for _, it := range existing {
		key := strings.ToLower(strings.TrimSpace(it.Text))
		if key == "" {
			continue
		}
		index[key] = len(order)
		order = append(order, it)
	}
	for _, it := range incoming {
		key := strings.ToLower(strings.TrimSpace(it.Text))
		if key == "" {
			continue
		}
		if idx, ok := index[key]; ok {
			order[idx] = it
			continue
		}
		index[key] = len(order)
		order = append(order, it)
	}
	return order
}

func mergeLinks(existing []LinkItem, incoming []LinkItem) []LinkItem {
	order := make([]LinkItem, 0, len(existing)+len(incoming))
	index := map[string]int{}
	for _, it := range existing {
		key := strings.TrimSpace(it.Target)
		if key == "" {
			continue
		}
		index[key] = len(order)
		order = append(order, it)
	}
	for _, it := range incoming {
		key := strings.TrimSpace(it.Target)
		if key == "" {
			continue
		}
		if idx, ok := index[key]; ok {
			order[idx] = it
			continue
		}
		index[key] = len(order)
		order = append(order, it)
	}
	return order
}

func writeKVSection(b *strings.Builder, title string, items []KVItem) {
	b.WriteString("## ")
	b.WriteString(title)
	b.WriteString("\n")
	for _, it := range items {
		if strings.TrimSpace(it.Title) == "" && strings.TrimSpace(it.Value) == "" {
			continue
		}
		b.WriteString("- **")
		b.WriteString(strings.TrimSpace(it.Title))
		b.WriteString("**: ")
		b.WriteString(strings.TrimSpace(it.Value))
		b.WriteString("\n")
	}
	b.WriteString("\n")
}

type subItem struct {
	Key   string
	Value string
}

func writeShortTermKVSection(b *strings.Builder, title string, items []KVItem) {
	b.WriteString("## ")
	b.WriteString(title)
	b.WriteString("\n")
	index := 1
	for _, it := range items {
		itemTitle := strings.TrimSpace(it.Title)
		itemValue := strings.TrimSpace(it.Value)
		if itemTitle == "" && itemValue == "" {
			continue
		}
		if itemTitle == "" {
			itemTitle = "Item"
		}
		if title == sectionSessionSummary {
			b.WriteString(strconv.Itoa(index))
			b.WriteString(". **Topic**: ")
			b.WriteString(itemTitle)
			b.WriteString("\n")
			subitems := parseValueSubitems(itemValue, "Event")
			subitems = orderSessionSubitems(subitems)
			writeSubitems(b, subitems)
		} else {
			b.WriteString(strconv.Itoa(index))
			b.WriteString(". ")
			b.WriteString(itemTitle)
			b.WriteString(":")
			b.WriteString("\n")
			subitems := parseValueSubitems(itemValue, "Detail")
			writeSubitems(b, subitems)
		}
		index++
	}
	b.WriteString("\n")
}

func parseValueSubitems(value string, defaultKey string) []subItem {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	lines := strings.Split(value, "\n")
	items := make([]subItem, 0, len(lines))
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "-") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "-"))
		}
		if line == "" {
			continue
		}
		key, val := splitSubitem(line)
		if key == "" {
			key = defaultKey
		}
		if strings.TrimSpace(val) == "" {
			continue
		}
		items = append(items, subItem{Key: key, Value: val})
	}
	return items
}

func splitSubitem(line string) (string, string) {
	parts := strings.SplitN(line, ":", 2)
	if len(parts) < 2 {
		return "", strings.TrimSpace(line)
	}
	key := strings.TrimSpace(parts[0])
	val := strings.TrimSpace(parts[1])
	return key, val
}

func orderSessionSubitems(items []subItem) []subItem {
	if len(items) == 0 {
		return items
	}
	order := []string{"Users", "Datetime", "Event", "Result"}
	used := make([]bool, len(items))
	ordered := make([]subItem, 0, len(items))
	for _, key := range order {
		for i, item := range items {
			if used[i] {
				continue
			}
			if strings.EqualFold(item.Key, key) {
				ordered = append(ordered, item)
				used[i] = true
			}
		}
	}
	for i, item := range items {
		if !used[i] {
			ordered = append(ordered, item)
		}
	}
	return ordered
}

func writeSubitems(b *strings.Builder, items []subItem) {
	for _, item := range items {
		key := strings.TrimSpace(item.Key)
		val := strings.TrimSpace(item.Value)
		if key == "" || val == "" {
			continue
		}
		b.WriteString("  - ")
		b.WriteString(key)
		b.WriteString(": ")
		b.WriteString(val)
		b.WriteString("\n")
	}
}

func writeTodoSection(b *strings.Builder, title string, items []TaskItem) {
	b.WriteString("## ")
	b.WriteString(title)
	b.WriteString("\n")
	for _, it := range items {
		text := strings.TrimSpace(it.Text)
		if text == "" {
			continue
		}
		if it.Done {
			b.WriteString("- [x] ")
		} else {
			b.WriteString("- [ ] ")
		}
		b.WriteString(text)
		b.WriteString("\n")
	}
	b.WriteString("\n")
}

func writeLinkSection(b *strings.Builder, title string, items []LinkItem) {
	b.WriteString("## ")
	b.WriteString(title)
	b.WriteString("\n")
	for _, it := range items {
		text := strings.TrimSpace(it.Text)
		target := strings.TrimSpace(it.Target)
		if target == "" {
			continue
		}
		if text == "" {
			text = target
		}
		b.WriteString("- [")
		b.WriteString(text)
		b.WriteString("](")
		b.WriteString(target)
		b.WriteString(")\n")
	}
	b.WriteString("\n")
}
