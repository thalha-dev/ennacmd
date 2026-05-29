//go:build !windows && !linux

package terminal

import "os"

func QueueInput(_ *os.File, _ string) error {
	return nil
}

func RunInsertHelper(_ []string) error {
	return nil
}
