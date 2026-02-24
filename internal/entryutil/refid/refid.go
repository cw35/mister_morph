package refid

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Protocol names are alphanumeric only (letters/digits).
var protocolPattern = regexp.MustCompile(`^[a-z][a-z0-9]*$`)

// Parse parses a generic reference id in "protocol:id" form.
func Parse(raw string) (protocol string, id string, ok bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", "", false
	}
	idx := strings.IndexByte(raw, ':')
	if idx <= 0 || idx >= len(raw)-1 {
		return "", "", false
	}
	protocol = strings.ToLower(strings.TrimSpace(raw[:idx]))
	id = strings.TrimSpace(raw[idx+1:])
	if protocol == "" || id == "" {
		return "", "", false
	}
	if !protocolPattern.MatchString(protocol) {
		return "", "", false
	}
	if strings.ContainsAny(id, " \t\r\n()") {
		return "", "", false
	}
	return protocol, id, true
}

// IsValid reports whether raw matches "protocol:id".
func IsValid(raw string) bool {
	_, _, ok := Parse(raw)
	return ok
}

// Normalize lowercases protocol and keeps id as-is.
func Normalize(raw string) (string, bool) {
	protocol, id, ok := Parse(raw)
	if !ok {
		return "", false
	}
	return protocol + ":" + id, true
}

// ParseTelegramChatIDHint parses "tg:<int64>" or plain "<int64>" chat hints.
// Empty input returns (0, false, nil).
func ParseTelegramChatIDHint(raw string) (int64, bool, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return 0, false, nil
	}
	if protocol, id, ok := Parse(value); ok {
		if protocol != "tg" {
			return 0, false, fmt.Errorf("invalid chat_id: %s", strings.TrimSpace(raw))
		}
		value = id
	}
	chatID, err := strconv.ParseInt(value, 10, 64)
	if err != nil || chatID == 0 {
		return 0, false, fmt.Errorf("invalid chat_id: %s", strings.TrimSpace(raw))
	}
	return chatID, true, nil
}

// ParseSlackChatIDHint parses "slack:<team_id>:<channel_id>" chat hints.
// Empty input returns ("", "", false, nil).
func ParseSlackChatIDHint(raw string) (string, string, bool, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", "", false, nil
	}
	if !strings.HasPrefix(strings.ToLower(value), "slack:") {
		return "", "", false, nil
	}
	suffix := strings.TrimSpace(value[len("slack:"):])
	parts := strings.Split(suffix, ":")
	if len(parts) != 2 {
		return "", "", true, fmt.Errorf("invalid chat_id: %s", strings.TrimSpace(raw))
	}
	teamID := strings.TrimSpace(parts[0])
	channelID := strings.TrimSpace(parts[1])
	if teamID == "" || channelID == "" {
		return "", "", true, fmt.Errorf("invalid chat_id: %s", strings.TrimSpace(raw))
	}
	return teamID, channelID, true, nil
}
