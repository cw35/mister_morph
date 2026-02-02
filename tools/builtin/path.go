package builtin

import (
	"github.com/quailyquaily/mistermorph/internal/pathutil"
)

func expandHomePath(p string) string {
	return pathutil.ExpandHomePath(p)
}
