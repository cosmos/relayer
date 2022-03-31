//go:build go1.18

package relaydebug

import (
	"runtime/debug"
	"strconv"
)

// BuildCommit reports the stamped vcs.revision according to debug.ReadBuildInfo,
// including a suffix indicating whether the working tree had uncommitted changes.
//
// Note that this will report "unknown" if using "go run".
// Use "go build" to compile to a binary to see the proper commit reported.
func BuildCommit() string {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown (built without module support?)"
	}

	rev := "unknown"
	dirty := false
	for _, s := range bi.Settings {
		switch s.Key {
		case "vcs.revision":
			rev = s.Value
		case "vcs.modified":
			if d, err := strconv.ParseBool(s.Value); err == nil {
				dirty = d
			}
		}
	}

	if dirty {
		return rev + " (dirty)"
	}

	return rev
}
