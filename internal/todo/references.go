package todo

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	refid "github.com/quailyquaily/mistermorph/internal/entryutil/refid"
)

var (
	englishSelfWordPattern = regexp.MustCompile(`(?i)\b(i|me|my|myself|we|us|our|ourselves)\b`)
)

func ExtractReferenceIDs(content string) ([]string, error) {
	return refid.ExtractMarkdownReferenceIDs(content)
}

// ValidateRequiredReferenceMentions enforces that first-person object mentions
// are explicitly referenceable (e.g. "[我](tg:1001)" / "[me](tg:1001)").
func ValidateRequiredReferenceMentions(content string, snapshot ContactSnapshot) error {
	content = strings.TrimSpace(content)
	if content == "" {
		return fmt.Errorf("content is required")
	}
	stripped := refid.StripMarkdownReferenceLinks(content)
	mention := firstPersonMention(stripped)
	if mention == "" {
		return nil
	}

	item := MissingReference{Mention: mention}
	if ref := suggestSelfReferenceID(snapshot); ref != "" {
		if suggestion, err := refid.FormatMarkdownReference(mention, ref); err == nil {
			item.Suggestion = suggestion
		}
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

	stripped := refid.StripMarkdownReferenceLinks(content)
	if firstPersonMention(stripped) == "" {
		return content, false, nil
	}

	linkRanges := refid.MarkdownReferenceLinkRanges(content)
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
		if refid.WithinMarkdownReferenceLink(ranges, start, end) {
			searchPos = end
			continue
		}
		annotated, err := refid.FormatMarkdownReference(token, refID)
		if err != nil {
			return content, false
		}
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
		if refid.WithinMarkdownReferenceLink(ranges, start, end) {
			continue
		}
		word := content[start:end]
		annotated, err := refid.FormatMarkdownReference(word, refID)
		if err != nil {
			return content, false
		}
		return content[:start] + annotated + content[end:], true
	}
	return content, false
}

func suggestSelfReferenceID(snapshot ContactSnapshot) string {
	ids := dedupeSortedStrings(snapshot.ReachableIDs)
	if len(ids) == 0 {
		return ""
	}
	if len(ids) == 1 {
		return ids[0]
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
