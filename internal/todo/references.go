package todo

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

var (
	// Strip already-annotated mentions like "[John](tg:1001)" before
	// scanning for missing first-person references.
	referenceLinkPattern    = regexp.MustCompile(`\[[^\[\]\n]+\]\(([^()]+)\)`)
	annotatedMentionPattern = regexp.MustCompile(`\[[^\[\]\n]+\]\([^()]+\)`)
	englishSelfWordPattern  = regexp.MustCompile(`(?i)\b(i|me|my|myself|we|us|our|ourselves)\b`)
)

func ExtractReferenceIDs(content string) ([]string, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, fmt.Errorf("content is required")
	}
	matches := referenceLinkPattern.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		return nil, nil
	}
	seen := make(map[string]bool, len(matches))
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		ref := strings.TrimSpace(m[1])
		if ref == "" {
			return nil, fmt.Errorf("missing reference id")
		}
		if !isValidReferenceID(ref) {
			return nil, fmt.Errorf("invalid reference id: %s", ref)
		}
		if seen[ref] {
			continue
		}
		seen[ref] = true
		out = append(out, ref)
	}
	return out, nil
}

func ValidateReachableReferences(content string, snapshot ContactSnapshot) error {
	refs, err := ExtractReferenceIDs(content)
	if err != nil {
		return err
	}
	for _, ref := range refs {
		ref = strings.TrimSpace(ref)
		if ref == "" {
			continue
		}
		if snapshot.HasReachableID(ref) {
			continue
		}
		return fmt.Errorf("reference id is not reachable: %s", ref)
	}
	return nil
}

// ValidateRequiredReferenceMentions enforces that first-person object mentions
// are explicitly referenceable (e.g. "[我](tg:1001)" / "[me](tg:1001)").
func ValidateRequiredReferenceMentions(content string, snapshot ContactSnapshot) error {
	content = strings.TrimSpace(content)
	if content == "" {
		return fmt.Errorf("content is required")
	}
	stripped := annotatedMentionPattern.ReplaceAllString(content, "")
	mention := firstPersonMention(stripped)
	if mention == "" {
		return nil
	}

	item := MissingReference{Mention: mention}
	if ref := suggestSelfReferenceID(snapshot); ref != "" {
		item.Suggestion = formatMentionLink(mention, ref)
	}
	return &MissingReferenceIDError{Items: []MissingReference{item}}
}

// AnnotateFirstPersonReference rewrites one unannotated first-person mention
// into "[mention](refID)". It returns (rewritten, changed, error).
func AnnotateFirstPersonReference(content string, refID string) (string, bool, error) {
	content = strings.TrimSpace(content)
	refID = strings.TrimSpace(refID)
	if content == "" {
		return "", false, fmt.Errorf("content is required")
	}
	if refID == "" {
		return content, false, nil
	}
	if !isValidReferenceID(refID) {
		return "", false, fmt.Errorf("invalid reference id: %s", refID)
	}

	stripped := annotatedMentionPattern.ReplaceAllString(content, "")
	if firstPersonMention(stripped) == "" {
		return content, false, nil
	}

	linkRanges := markdownLinkRanges(content)
	for _, token := range []string{"我们", "本人", "我"} {
		if rewritten, ok := annotateFirstUnannotatedLiteral(content, token, refID, linkRanges); ok {
			return rewritten, true, nil
		}
	}
	if rewritten, ok := annotateFirstUnannotatedEnglishSelf(content, refID, linkRanges); ok {
		return rewritten, true, nil
	}
	return content, false, nil
}

func firstPersonMention(content string) string {
	content = strings.TrimSpace(content)
	if content == "" {
		return ""
	}
	for _, token := range []string{"我们", "本人", "我"} {
		if strings.Contains(content, token) {
			return token
		}
	}
	if m := englishSelfWordPattern.FindString(content); strings.TrimSpace(m) != "" {
		return strings.TrimSpace(m)
	}
	return ""
}

func annotateFirstUnannotatedLiteral(content string, token string, refID string, ranges [][2]int) (string, bool) {
	token = strings.TrimSpace(token)
	if token == "" {
		return content, false
	}
	searchPos := 0
	for {
		off := strings.Index(content[searchPos:], token)
		if off < 0 {
			return content, false
		}
		start := searchPos + off
		end := start + len(token)
		if withinMarkdownLink(ranges, start, end) {
			searchPos = end
			continue
		}
		annotated := formatMentionLink(token, refID)
		return content[:start] + annotated + content[end:], true
	}
}

func annotateFirstUnannotatedEnglishSelf(content string, refID string, ranges [][2]int) (string, bool) {
	indices := englishSelfWordPattern.FindAllStringIndex(content, -1)
	for _, pair := range indices {
		if len(pair) != 2 {
			continue
		}
		start, end := pair[0], pair[1]
		if start < 0 || end > len(content) || start >= end {
			continue
		}
		if withinMarkdownLink(ranges, start, end) {
			continue
		}
		word := content[start:end]
		annotated := formatMentionLink(word, refID)
		return content[:start] + annotated + content[end:], true
	}
	return content, false
}

func markdownLinkRanges(content string) [][2]int {
	indices := annotatedMentionPattern.FindAllStringIndex(content, -1)
	if len(indices) == 0 {
		return nil
	}
	out := make([][2]int, 0, len(indices))
	for _, pair := range indices {
		if len(pair) != 2 || pair[0] < 0 || pair[1] <= pair[0] || pair[1] > len(content) {
			continue
		}
		out = append(out, [2]int{pair[0], pair[1]})
	}
	return out
}

func withinMarkdownLink(ranges [][2]int, start int, end int) bool {
	if len(ranges) == 0 || start < 0 || end <= start {
		return false
	}
	for _, pair := range ranges {
		if start >= pair[0] && end <= pair[1] {
			return true
		}
	}
	return false
}

func formatMentionLink(label string, refID string) string {
	label = strings.TrimSpace(label)
	refID = strings.TrimSpace(refID)
	return "[" + label + "](" + refID + ")"
}

func suggestSelfReferenceID(snapshot ContactSnapshot) string {
	ids := dedupeSortedStrings(snapshot.ReachableIDs)
	if len(ids) == 0 {
		return ""
	}

	tgIDs := make([]string, 0, len(ids))
	for _, id := range ids {
		if strings.HasPrefix(id, "tg:") {
			tgIDs = append(tgIDs, id)
		}
	}
	tgIDs = dedupeSortedStrings(tgIDs)
	if len(tgIDs) == 1 {
		return tgIDs[0]
	}

	preferred := make([]string, 0, len(snapshot.Contacts))
	for _, c := range snapshot.Contacts {
		id := strings.TrimSpace(c.PreferredID)
		if id == "" || !isValidReferenceID(id) {
			continue
		}
		preferred = append(preferred, id)
	}
	preferred = dedupeSortedStrings(preferred)
	if len(preferred) == 1 {
		return preferred[0]
	}
	return ""
}

func dedupeSortedStrings(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(items))
	out := make([]string, 0, len(items))
	for _, raw := range items {
		v := strings.TrimSpace(raw)
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}
