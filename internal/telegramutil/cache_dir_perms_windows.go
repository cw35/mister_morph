//go:build windows

package telegramutil

import "os"

func ensureCacheDirOwnershipAndPerms(_ string, _ os.FileInfo) error {
	// Windows does not expose POSIX uid/mode bits in a portable way.
	return nil
}
