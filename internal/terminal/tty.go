package terminal

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"

	"golang.org/x/term"
)

type TTY struct {
	In  *os.File
	Out *os.File
}

func OpenTTY() (*TTY, error) {
	if runtime.GOOS == "windows" {
		in, err := os.OpenFile("CONIN$", os.O_RDWR, 0)
		if err != nil {
			return nil, fmt.Errorf("open console input: %w", err)
		}
		out, err := os.OpenFile("CONOUT$", os.O_RDWR, 0)
		if err != nil {
			_ = in.Close()
			return nil, fmt.Errorf("open console output: %w", err)
		}
		return &TTY{In: in, Out: out}, nil
	}

	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return nil, fmt.Errorf("open controlling tty: %w", err)
	}
	return &TTY{In: tty, Out: tty}, nil
}

func (t *TTY) Close() {
	if t == nil {
		return
	}
	if t.In != nil {
		_ = t.In.Close()
	}
	if t.Out != nil && t.Out != t.In {
		_ = t.Out.Close()
	}
}

func Size(out *os.File) (int, int) {
	if out == nil {
		return 80, 24
	}
	width, height, err := term.GetSize(int(out.Fd()))
	if err == nil && width > 0 && height > 0 {
		return width, height
	}
	if fallbackWidth, fallbackHeight, ok := fallbackSize(out); ok {
		return fallbackWidth, fallbackHeight
	}
	if err != nil || width <= 0 || height <= 0 {
		return 80, 24
	}
	return width, height
}

func SaveCursor(w io.Writer) {
	if w == nil {
		return
	}
	_, _ = fmt.Fprint(w, "\x1b[s")
}

func RestoreCursor(w io.Writer) {
	if w == nil {
		return
	}
	_, _ = fmt.Fprint(w, "\x1b[u")
}

func ClearOverlay(w io.Writer, row int, col int, width int, height int) {
	if w == nil || row <= 0 || col <= 0 || width <= 0 || height <= 0 {
		return
	}
	blank := strings.Repeat(" ", width)
	for index := 0; index < height; index++ {
		_, _ = fmt.Fprintf(w, "\x1b[%d;%dH%s", row+index, col, blank)
	}
}
