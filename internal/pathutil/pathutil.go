package pathutil

import (
	"os"
	"path/filepath"
	"strings"
)

func ExpandHomePath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return ""
	}
	if p == "~" || strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir()
		if err != nil || strings.TrimSpace(home) == "" {
			return filepath.Clean(p)
		}
		if p == "~" {
			return filepath.Clean(home)
		}
		return filepath.Clean(filepath.Join(home, strings.TrimPrefix(p, "~/")))
	}
	return filepath.Clean(p)
}

func NormalizeFileCacheDirPath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return p
	}
	trimmed := strings.TrimLeft(p, "/\\")
	if strings.HasPrefix(trimmed, "file_cache_dir/") || strings.HasPrefix(trimmed, "file_cache_dir\\") {
		trimmed = strings.TrimPrefix(trimmed, "file_cache_dir/")
		trimmed = strings.TrimPrefix(trimmed, "file_cache_dir\\")
		return strings.TrimLeft(trimmed, "/\\")
	}
	return p
}
