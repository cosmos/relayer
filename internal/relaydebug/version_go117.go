//go:build !go1.18

package relaydebug

import (
	"fmt"
	"runtime"
)

// BuildCommit reports an unknown commit,
// due to building with a version of Go older than 1.18.
func BuildCommit() string {
	return fmt.Sprintf("unknown (built with %s)", runtime.Version())
}
