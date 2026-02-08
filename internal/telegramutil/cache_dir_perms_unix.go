//go:build !windows

package telegramutil

import (
	"fmt"
	"os"
	"syscall"
)

func ensureCacheDirOwnershipAndPerms(dir string, fi os.FileInfo) error {
	perm := fi.Mode().Perm()
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok || st == nil {
		return fmt.Errorf("unsupported stat for: %s", dir)
	}
	curUID := uint32(os.Getuid())
	if st.Uid != curUID {
		return fmt.Errorf("cache dir not owned by current user (uid=%d, owner=%d): %s", curUID, st.Uid, dir)
	}
	if perm == 0o700 {
		return nil
	}

	// Try to fix perms when the directory is owned by current user.
	if err := os.Chmod(dir, 0o700); err != nil {
		return fmt.Errorf("cache dir has insecure perms (%#o) and chmod failed: %w", perm, err)
	}
	fi2, err := os.Stat(dir)
	if err != nil {
		return err
	}
	if fi2.Mode().Perm() != 0o700 {
		return fmt.Errorf("cache dir has insecure perms (%#o): %s", fi2.Mode().Perm(), dir)
	}
	return nil
}
