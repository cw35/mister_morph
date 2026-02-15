package chathistory

import (
	"strings"
)

func BuildMessages(channel string, items []ChatHistoryItem) []ChatHistoryItem {
	channel = strings.TrimSpace(channel)
	out := make([]ChatHistoryItem, 0, len(items))
	for _, item := range items {
		cp := item
		if strings.TrimSpace(cp.Channel) == "" {
			cp.Channel = channel
		}
		out = append(out, cp)
	}
	return out
}
