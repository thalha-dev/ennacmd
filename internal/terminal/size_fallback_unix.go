//go:build !windows

package terminal

import (
	"os"

	"github.com/creack/pty"
)

func fallbackSize(out *os.File) (int, int, bool) {
	if out == nil {
		return 0, 0, false
	}

	windowSize, err := pty.GetsizeFull(out)
	if err != nil || windowSize == nil || windowSize.Cols == 0 || windowSize.Rows == 0 {
		return 0, 0, false
	}

	return int(windowSize.Cols), int(windowSize.Rows), true
}
