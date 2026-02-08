package telegramutil

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type fileCacheEntry struct {
	Path    string
	ModTime time.Time
	Size    int64
}

func EnsureSecureCacheDir(dir string) error {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return fmt.Errorf("empty dir")
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	dir = abs

	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}

	fi, err := os.Lstat(dir)
	if err != nil {
		return err
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refusing symlink path: %s", dir)
	}
	if !fi.IsDir() {
		return fmt.Errorf("not a directory: %s", dir)
	}

	return ensureCacheDirOwnershipAndPerms(dir, fi)
}

func CleanupFileCacheDir(dir string, maxAge time.Duration, maxFiles int, maxTotalBytes int64) error {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return fmt.Errorf("missing dir")
	}
	if maxAge <= 0 && maxFiles <= 0 && maxTotalBytes <= 0 {
		return nil
	}
	now := time.Now()

	var kept []fileCacheEntry
	total := int64(0)

	walkErr := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		// Never follow symlinks.
		if d.Type()&os.ModeSymlink != 0 {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		if maxAge > 0 && now.Sub(info.ModTime()) > maxAge {
			_ = os.Remove(path)
			return nil
		}
		kept = append(kept, fileCacheEntry{
			Path:    path,
			ModTime: info.ModTime(),
			Size:    info.Size(),
		})
		total += info.Size()
		return nil
	})
	if walkErr != nil && !os.IsNotExist(walkErr) {
		return walkErr
	}

	// Enforce max_files and max_total_bytes by removing oldest files first.
	sort.Slice(kept, func(i, j int) bool { return kept[i].ModTime.Before(kept[j].ModTime) })
	needPrune := func() bool {
		if maxFiles > 0 && len(kept) > maxFiles {
			return true
		}
		if maxTotalBytes > 0 && total > maxTotalBytes {
			return true
		}
		return false
	}
	for needPrune() && len(kept) > 0 {
		old := kept[0]
		kept = kept[1:]
		total -= old.Size
		_ = os.Remove(old.Path)
	}

	// Best-effort remove empty dirs (bottom-up).
	var dirs []string
	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			dirs = append(dirs, path)
		}
		return nil
	})
	sort.Slice(dirs, func(i, j int) bool { return len(dirs[i]) > len(dirs[j]) })
	for _, d := range dirs {
		if filepath.Clean(d) == filepath.Clean(dir) {
			continue
		}
		_ = os.Remove(d)
	}
	return nil
}
