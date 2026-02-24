package refid

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	markdownReferencePattern = regexp.MustCompile(`\[[^\[\]\n]+\]\(([^()]+)\)`)
	markdownLinkPattern      = regexp.MustCompile(`\[[^\[\]\n]+\]\([^()]+\)`)
)

// ExtractMarkdownReferenceIDs returns unique reference IDs from "[Label](protocol:id)" mentions.
func ExtractMarkdownReferenceIDs(content string) ([]string, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, fmt.Errorf("content is required")
	}
	matches := markdownReferencePattern.FindAllStringSubmatch(content, -1)
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
		if !IsValid(ref) {
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

// StripMarkdownReferenceLinks removes markdown links like "[John](tg:1001)".
func StripMarkdownReferenceLinks(content string) string {
	return markdownLinkPattern.ReplaceAllString(content, "")
}

// MarkdownReferenceLinkRanges returns byte ranges of markdown links "[...](...)".
func MarkdownReferenceLinkRanges(content string) [][2]int {
	indices := markdownLinkPattern.FindAllStringIndex(content, -1)
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

// WithinMarkdownReferenceLink reports whether [start,end) is fully inside any markdown link range.
func WithinMarkdownReferenceLink(ranges [][2]int, start int, end int) bool {
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

// FormatMarkdownReference formats "[Label](protocol:id)" and validates the reference id.
func FormatMarkdownReference(label string, refID string) (string, error) {
	label = strings.TrimSpace(label)
	refID = strings.TrimSpace(refID)
	if label == "" {
		return "", fmt.Errorf("label is required")
	}
	if !IsValid(refID) {
		return "", fmt.Errorf("invalid reference id: %s", refID)
	}
	return "[" + label + "](" + refID + ")", nil
}
