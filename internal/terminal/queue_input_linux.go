//go:build linux

package terminal

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

func QueueInput(in *os.File, command string) error {
	text := sanitizeQueuedCommand(command)
	if strings.TrimSpace(text) == "" {
		return nil
	}

	if err := injectQueuedCommand(in, linuxTTYPath(in), text); err == nil {
		return nil
	}

	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable path: %w", err)
	}

	ttyPath := linuxTTYPath(in)
	payload := base64.RawURLEncoding.EncodeToString([]byte(text))
	ttyPayload := base64.RawURLEncoding.EncodeToString([]byte(ttyPath))
	cmd := exec.Command(executable, "__insert-helper", strconv.Itoa(os.Getpid()), ttyPayload, payload)
	cmd.Stdin = nil
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start deferred insert helper: %w", err)
	}

	return nil
}

func RunInsertHelper(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("missing insert payload")
	}

	parentPID, err := strconv.Atoi(args[0])
	if err != nil {
		return fmt.Errorf("parse parent pid: %w", err)
	}

	ttyPayloadIndex := 1
	payloadIndex := 2
	if len(args) == 2 {
		ttyPayloadIndex = -1
		payloadIndex = 1
	}

	ttyPath := ""
	if ttyPayloadIndex >= 0 {
		decodedPath, err := base64.RawURLEncoding.DecodeString(args[ttyPayloadIndex])
		if err != nil {
			return fmt.Errorf("decode tty payload: %w", err)
		}
		ttyPath = string(decodedPath)
	}

	payload, err := base64.RawURLEncoding.DecodeString(args[payloadIndex])
	if err != nil {
		return fmt.Errorf("decode insert payload: %w", err)
	}

	text := string(payload)
	if strings.TrimSpace(text) == "" {
		return nil
	}

	waitForLinuxProcessExit(parentPID, 3*time.Second)
	time.Sleep(120 * time.Millisecond)

	if err := injectQueuedCommand(nil, ttyPath, text); err != nil {
		return fmt.Errorf("inject queued command into tty: %w", err)
	}

	return nil
}

func waitForLinuxProcessExit(pid int, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		err := unix.Kill(pid, 0)
		if err == nil {
			time.Sleep(20 * time.Millisecond)
			continue
		}
		if errors.Is(err, unix.ESRCH) {
			return
		}
		return
	}
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
	if errors.Is(lastErr, unix.EPERM) || errors.Is(lastErr, unix.EACCES) || errors.Is(lastErr, unix.EIO) || errors.Is(lastErr, unix.ENOTTY) {
		return fmt.Errorf("%w", lastErr)
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
