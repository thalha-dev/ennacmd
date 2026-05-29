//go:build linux

package terminal

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/thalha-dev/ennacmd/internal/shell"
	"golang.org/x/sys/unix"
)

func QueueInput(in *os.File, command string) error {
	text := sanitizeQueuedCommand(command)
	if strings.TrimSpace(text) == "" {
		return nil
	}

	if err := injectQueuedCommand(in, linuxTTYPath(in), text); err != nil {
		if errors.Is(err, unix.EPERM) || errors.Is(err, unix.EACCES) || errors.Is(err, unix.EIO) || errors.Is(err, unix.ENOTTY) {
			return fmt.Errorf("inject queued command into tty: %w (%s)", err, linuxShellInstallHint())
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

func linuxShellInstallHint() string {
	kind := shell.Detect("")
	command := "ennacmd shell-install"

	if executable, err := os.Executable(); err == nil && executable != "" {
		if relative, relErr := filepath.Rel(mustGetwd(), executable); relErr == nil && relative != "" && !strings.HasPrefix(relative, "..") {
			if !strings.HasPrefix(relative, ".") {
				relative = "." + string(filepath.Separator) + relative
			}
			command = relative + " shell-install"
		} else {
			command = executable + " shell-install"
		}
	}

	if kind != shell.Auto {
		command += " " + string(kind)
		return fmt.Sprintf("run '%s' once, then restart %s", command, kind.DisplayName())
	}

	return fmt.Sprintf("run '%s' once, then restart your shell", command)
}

func mustGetwd() string {
	workingDir, err := os.Getwd()
	if err != nil || workingDir == "" {
		return "."
	}
	return workingDir
}

func sanitizeQueuedCommand(command string) string {
	text := strings.ReplaceAll(command, "\r\n", " ")
	text = strings.ReplaceAll(text, "\n", " ")
	text = strings.ReplaceAll(text, "\r", " ")
	return strings.TrimRight(text, " ")
}
