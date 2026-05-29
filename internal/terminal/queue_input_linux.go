//go:build linux

package terminal

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"golang.org/x/sys/unix"
)

func QueueInput(_ *os.File, command string) error {
	text := sanitizeQueuedCommand(command)
	if strings.TrimSpace(text) == "" {
		return nil
	}

	tty, err := OpenTTY()
	if err != nil {
		return err
	}
	defer tty.Close()

	fd := int(tty.In.Fd())
	for _, value := range []byte(text) {
		if err := unix.IoctlSetPointerInt(fd, unix.TIOCSTI, int(value)); err != nil {
			if errors.Is(err, unix.EPERM) || errors.Is(err, unix.EACCES) {
				return fmt.Errorf("inject queued command into tty: %w (TIOCSTI is blocked by this Linux system)", err)
			}
			return fmt.Errorf("inject queued command into tty: %w", err)
		}
	}

	return nil
}

func RunInsertHelper(_ []string) error {
	return nil
}

func sanitizeQueuedCommand(command string) string {
	text := strings.ReplaceAll(command, "\r\n", " ")
	text = strings.ReplaceAll(text, "\n", " ")
	text = strings.ReplaceAll(text, "\r", " ")
	return strings.TrimRight(text, " ")
}
