package shell

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

type Kind string

const (
	Auto       Kind = "auto"
	PowerShell Kind = "powershell"
	Bash       Kind = "bash"
	Zsh        Kind = "zsh"
	Fish       Kind = "fish"
)

func Detect(value string) Kind {
	if normalized := Normalize(value); normalized != Auto {
		return normalized
	}

	if runtime.GOOS == "windows" {
		if strings.Contains(strings.ToLower(os.Getenv("PSMODULEPATH")), "powershell") {
			return PowerShell
		}
		comspec := strings.ToLower(filepath.Base(os.Getenv("COMSPEC")))
		if strings.Contains(comspec, "pwsh") || strings.Contains(comspec, "powershell") {
			return PowerShell
		}
		return PowerShell
	}

	shellPath := strings.ToLower(filepath.Base(os.Getenv("SHELL")))
	switch {
	case strings.Contains(shellPath, "fish"):
		return Fish
	case strings.Contains(shellPath, "zsh"):
		return Zsh
	case strings.Contains(shellPath, "bash"):
		return Bash
	default:
		return Bash
	}
}

func Normalize(value string) Kind {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "powershell", "pwsh", "windows-powershell", "windows powershell":
		return PowerShell
	case "bash":
		return Bash
	case "zsh":
		return Zsh
	case "fish":
		return Fish
	default:
		return Auto
	}
}

func (k Kind) DisplayName() string {
	switch k {
	case PowerShell:
		return "Windows PowerShell"
	case Zsh:
		return "zsh"
	case Fish:
		return "fish"
	case Bash:
		return "bash"
	default:
		return string(k)
	}
}
