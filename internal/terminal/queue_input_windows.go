//go:build windows

package terminal

import (
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	consoleKeyEvent = 0x0001
	createNoWindow  = 0x08000000
)

var writeConsoleInputProc = windows.NewLazySystemDLL("kernel32.dll").NewProc("WriteConsoleInputW")

type inputRecord struct {
	EventType uint16
	_         uint16
	KeyEvent  keyEventRecord
}

type keyEventRecord struct {
	KeyDown         int32
	RepeatCount     uint16
	VirtualKeyCode  uint16
	VirtualScanCode uint16
	UnicodeChar     uint16
	ControlKeyState uint32
}

func QueueInput(in *os.File, command string) error {
	_ = in

	text := sanitizeQueuedCommand(command)
	if strings.TrimSpace(text) == "" {
		return nil
	}

	if err := writeConsoleInput(text); err == nil {
		return nil
	}

	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable path: %w", err)
	}

	payload := base64.RawURLEncoding.EncodeToString([]byte(text))
	cmd := exec.Command(executable, "__insert-helper", strconv.Itoa(os.Getpid()), payload)
	cmd.Stdin = nil
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start deferred insert helper: %w", err)
	}

	return nil
}

func RunInsertHelper(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing insert payload")
	}

	payloadIndex := 0
	if len(args) > 1 {
		parentPID, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("parse parent pid: %w", err)
		}
		waitForProcessExit(uint32(parentPID), 3*time.Second)
		payloadIndex = 1
	}
	if payloadIndex >= len(args) {
		return fmt.Errorf("missing insert payload")
	}

	payload, err := base64.RawURLEncoding.DecodeString(args[payloadIndex])
	if err != nil {
		return fmt.Errorf("decode insert payload: %w", err)
	}

	text := string(payload)
	if strings.TrimSpace(text) == "" {
		return nil
	}

	time.Sleep(120 * time.Millisecond)
	return writeConsoleInput(text)
}

func waitForProcessExit(pid uint32, timeout time.Duration) {
	handle, err := windows.OpenProcess(windows.SYNCHRONIZE, false, pid)
	if err != nil {
		return
	}
	defer windows.CloseHandle(handle)

	milliseconds := uint32(timeout / time.Millisecond)
	if milliseconds == 0 {
		milliseconds = 1
	}
	_, _ = windows.WaitForSingleObject(handle, milliseconds)
}

func writeConsoleInput(text string) error {
	utf16Text := utf16.Encode([]rune(text))
	if len(utf16Text) == 0 {
		return nil
	}

	consoleIn, err := os.OpenFile("CONIN$", os.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("open console input: %w", err)
	}
	defer consoleIn.Close()

	records := make([]inputRecord, 0, len(utf16Text))
	for _, codeUnit := range utf16Text {
		records = append(records, inputRecord{
			EventType: consoleKeyEvent,
			KeyEvent: keyEventRecord{
				KeyDown:     1,
				RepeatCount: 1,
				UnicodeChar: codeUnit,
			},
		})
	}

	written := uint32(0)

	r1, _, callErr := writeConsoleInputProc.Call(
		consoleIn.Fd(),
		uintptr(unsafe.Pointer(&records[0])),
		uintptr(len(records)),
		uintptr(unsafe.Pointer(&written)),
	)
	if r1 == 0 {
		if callErr != nil && callErr != windows.ERROR_SUCCESS {
			return fmt.Errorf("write console input: %w", callErr)
		}
		return fmt.Errorf("write console input failed")
	}

	if written != uint32(len(records)) {
		return fmt.Errorf("write console input wrote %d of %d events", written, len(records))
	}

	return nil
}

func sanitizeQueuedCommand(command string) string {
	text := strings.ReplaceAll(command, "\r\n", " ")
	text = strings.ReplaceAll(text, "\n", " ")
	text = strings.ReplaceAll(text, "\r", " ")
	return strings.TrimRight(text, " ")
}
