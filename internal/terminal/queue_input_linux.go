//go:build linux

package terminal

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"golang.org/x/sys/unix"
)

func QueueInput(in *os.File, command string) error {
	text := sanitizeQueuedCommand(command)
	if strings.TrimSpace(text) == "" {
		return nil
	}

	if err := injectQueuedCommand(in, linuxTTYPath(in), text); err != nil {
		if errors.Is(err, unix.EPERM) || errors.Is(err, unix.EACCES) || errors.Is(err, unix.EIO) || errors.Is(err, unix.ENOTTY) {
			return fmt.Errorf("inject queued command into tty: %w (run 'ennacmd shell-install' once to enable supported shell integration)", err)
		}
		return fmt.Errorf("inject queued command into tty: %w", err)
	}

	return nil
}

func RunInsertHelper(_ []string) error {
	return nil
}

func injectQueuedCommand(in *os.File, ttyPath string, text string) error {
	var lastErr error

	if in != nil {
		if err := injectFileInput(in, text); err == nil {
			return nil
		} else {
			lastErr = err
		}
	}

	if ttyPath != "" {
		if err := injectTTYPath(ttyPath, text); err == nil {
			return nil
		} else {
			lastErr = err
		}
	}

	if err := injectTTYPath("/dev/tty", text); err == nil {
		return nil
	} else {
		lastErr = err
	}

	if lastErr == nil {
		return errors.New("no tty available for queued input")
	}
	return lastErr
}

func injectTTYPath(path string, text string) error {
	if path == "" {
		return errors.New("empty tty path")
	}

	tty, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer tty.Close()

	return injectFileInput(tty, text)
}

func injectFileInput(file *os.File, text string) error {
	fd := int(file.Fd())
	for _, value := range []byte(text) {
		if err := unix.IoctlSetPointerInt(fd, unix.TIOCSTI, int(value)); err != nil {
			return err
		}
	}
	return nil
}

func linuxTTYPath(in *os.File) string {
	paths := []string{}
	if in != nil {
		paths = append(paths, linuxFDPath(int(in.Fd())))
	}
	paths = append(paths, linuxFDPath(0), linuxFDPath(1), linuxFDPath(2))

	for _, path := range paths {
		if path == "" || path == "/dev/null" {
			continue
		}
		return path
	}

	return "/dev/tty"
}

func linuxFDPath(fd int) string {
	if fd < 0 {
		return ""
	}

	path, err := os.Readlink(fmt.Sprintf("/proc/self/fd/%d", fd))
	if err != nil {
		return ""
	}
	return path
}

func sanitizeQueuedCommand(command string) string {
	text := strings.ReplaceAll(command, "\r\n", " ")
	text = strings.ReplaceAll(text, "\n", " ")
	text = strings.ReplaceAll(text, "\r", " ")
	return strings.TrimRight(text, " ")
}
