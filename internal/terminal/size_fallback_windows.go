//go:build windows

package terminal

import "os"

func fallbackSize(_ *os.File) (int, int, bool) {
	return 0, 0, false
}
