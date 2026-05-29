package shell

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	integrationStartMarker = "# >>> ennacmd shell integration >>>"
	integrationEndMarker   = "# <<< ennacmd shell integration <<<"
)

func IntegrationScript(kind Kind) (string, error) {
	return integrationScript(kind, "")
}

func integrationScript(kind Kind, fallbackBinaryPath string) (string, error) {
	bashBinary := posixSingleQuote(fallbackBinaryPath)
	powerShellBinary := powerShellSingleQuote(fallbackBinaryPath)

	switch kind {
	case Zsh:
		return fmt.Sprintf(strings.TrimSpace(`function ennacmd() {
	local _ennacmd_binary
	_ennacmd_binary="$(__ennacmd_resolve_binary)" || {
		print -u2 -- "ennacmd: binary not found; install ennacmd on PATH or set ENNACMD_BIN"
		return 127
	}
  if (( $# > 0 )); then
		"$_ennacmd_binary" "$@"
    return $?
  fi

  local _ennacmd_command
	_ennacmd_command="$("$_ennacmd_binary" --capture)"
  local _ennacmd_status=$?
  if (( _ennacmd_status != 0 )); then
    return $_ennacmd_status
  fi

  if [[ -n "$_ennacmd_command" ]]; then
    print -z -- "$_ennacmd_command"
  fi
}

function __ennacmd_resolve_binary() {
	if [[ -n "${ENNACMD_BIN:-}" && -x "${ENNACMD_BIN}" ]]; then
		print -r -- "${ENNACMD_BIN}"
		return 0
	fi

	local from_path
	from_path="$(whence -p ennacmd 2>/dev/null)"
	if [[ -n "$from_path" && -x "$from_path" ]]; then
		print -r -- "$from_path"
		return 0
	fi

	local fallback=%s
	if [[ -n "$fallback" && -x "$fallback" ]]; then
		print -r -- "$fallback"
		return 0
	fi

	return 1
}`), bashBinary) + "\n", nil
	case Bash:
		return fmt.Sprintf(strings.TrimSpace(`__ennacmd_widget() {
	local binary
	binary="$(__ennacmd_resolve_binary)" || {
		printf '%s\n' 'ennacmd: binary not found; install ennacmd on PATH or set ENNACMD_BIN' >&2
		return 127
	}
  local command
	command="$("$binary" --capture)"
  local status=$?
  if [[ $status -ne 0 ]]; then
    return $status
  fi

  if [[ -n "$command" ]]; then
    READLINE_LINE="$command"
    READLINE_POINT=${#READLINE_LINE}
  fi
}

__ennacmd_resolve_binary() {
	if [[ -n "${ENNACMD_BIN:-}" && -x "${ENNACMD_BIN}" ]]; then
		printf '%s\n' "${ENNACMD_BIN}"
		return 0
	fi

	local from_path
	from_path="$(type -P ennacmd 2>/dev/null)"
	if [[ -n "$from_path" && -x "$from_path" ]]; then
		printf '%s\n' "$from_path"
		return 0
	fi

	local fallback=%s
	if [[ -n "$fallback" && -x "$fallback" ]]; then
		printf '%s\n' "$fallback"
		return 0
	fi

	return 1
}

bind -x '"\C-g":__ennacmd_widget'`), bashBinary) + "\n", nil
	case Fish:
		return fmt.Sprintf(strings.TrimSpace(`function __ennacmd_widget
	    set -l binary (__ennacmd_resolve_binary)
	    set -l status $status
	    if test $status -ne 0
	        printf '%s\n' 'ennacmd: binary not found; install ennacmd on PATH or set ENNACMD_BIN' >&2
	        return 127
	    end

	    set -l command ($binary --capture)
    set -l status $status
    if test $status -ne 0
        return $status
    end

    if test -n "$command"
        commandline --replace -- $command
        commandline -f repaint
    end
end

function __ennacmd_resolve_binary
	    if test -n "$ENNACMD_BIN"; and test -x "$ENNACMD_BIN"
	        printf '%s\n' "$ENNACMD_BIN"
	        return 0
	    end

	    set -l from_path (type -p ennacmd 2>/dev/null)
	    if test -n "$from_path"; and test -x "$from_path"
	        printf '%s\n' "$from_path"
	        return 0
	    end

	    set -l fallback %s
	    if test -n "$fallback"; and test -x "$fallback"
	        printf '%s\n' "$fallback"
	        return 0
	    end

	    return 1
end

bind \cg __ennacmd_widget`), bashBinary) + "\n", nil
	case PowerShell:
		return fmt.Sprintf(strings.TrimSpace(`function ennacmd {
    param(
        [Parameter(ValueFromRemainingArguments = $true)]
        [string[]] $Arguments
    )

		$binary = Get-EnnacmdBinary
    if ($Arguments.Count -gt 0) {
        & $binary @Arguments
        return
    }

    $command = & $binary --capture
    $status = $LASTEXITCODE
    if ($status -ne 0) {
        $global:LASTEXITCODE = $status
        return
    }

    if (-not [string]::IsNullOrWhiteSpace($command)) {
        [Microsoft.PowerShell.PSConsoleReadLine]::Insert($command)
        [Microsoft.PowerShell.PSConsoleReadLine]::InvokePrompt()
    }
}

function Get-EnnacmdBinary {
	if (-not [string]::IsNullOrWhiteSpace($env:ENNACMD_BIN) -and (Test-Path $env:ENNACMD_BIN -PathType Leaf)) {
		return $env:ENNACMD_BIN
	}

	$command = Get-Command ennacmd.exe -CommandType Application -ErrorAction SilentlyContinue
	if ($command) {
		return $command.Source
	}

	$fallback = %s
	if (-not [string]::IsNullOrWhiteSpace($fallback) -and (Test-Path $fallback -PathType Leaf)) {
		return $fallback
	}

	throw 'ennacmd: binary not found; install ennacmd on PATH or set ENNACMD_BIN'
}`), powerShellBinary) + "\n", nil
	default:
		return "", fmt.Errorf("shell integration is not available for %q", kind)
	}
}

func InstallIntegration(kind Kind) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve user home: %w", err)
	}

	binaryPath, err := integrationBinaryPath()
	if err != nil {
		return "", err
	}

	script, err := integrationScript(kind, binaryPath)
	if err != nil {
		return "", err
	}

	path, managedFile, err := integrationPath(kind, home)
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", fmt.Errorf("create shell integration directory: %w", err)
	}

	existing := []byte(nil)
	if data, readErr := os.ReadFile(path); readErr == nil {
		existing = data
	} else if !os.IsNotExist(readErr) {
		return "", fmt.Errorf("read shell integration file: %w", readErr)
	}

	updated := mergeManagedBlock(string(existing), script)
	if managedFile {
		updated = managedScriptBlock(script)
	}

	if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
		return "", fmt.Errorf("write shell integration file: %w", err)
	}

	return path, nil
}

func integrationPath(kind Kind, home string) (string, bool, error) {
	switch kind {
	case Zsh:
		return filepath.Join(home, ".zshrc"), false, nil
	case Bash:
		return filepath.Join(home, ".bashrc"), false, nil
	case Fish:
		return filepath.Join(home, ".config", "fish", "conf.d", "ennacmd.fish"), true, nil
	case PowerShell:
		path, err := powerShellProfilePath(home)
		return path, false, err
	default:
		return "", false, fmt.Errorf("automatic shell installation is not available for %q", kind)
	}
}

func powerShellProfilePath(home string) (string, error) {
	candidates := []string{"pwsh", "powershell"}
	for _, candidate := range candidates {
		binary, err := exec.LookPath(candidate)
		if err != nil {
			continue
		}

		output, err := exec.Command(binary, "-NoProfile", "-Command", "$PROFILE.CurrentUserCurrentHost").Output()
		if err != nil {
			continue
		}

		profilePath := strings.TrimSpace(string(output))
		if profilePath != "" {
			return profilePath, nil
		}
	}

	return filepath.Join(home, "Documents", "PowerShell", "Microsoft.PowerShell_profile.ps1"), nil
}

func integrationBinaryPath() (string, error) {
	if len(os.Args) > 0 {
		if candidate, err := exec.LookPath(os.Args[0]); err == nil {
			if absolute, absErr := filepath.Abs(candidate); absErr == nil {
				return absolute, nil
			}
			return candidate, nil
		}

		if filepath.IsAbs(os.Args[0]) {
			return os.Args[0], nil
		}

		if strings.ContainsRune(os.Args[0], filepath.Separator) {
			if absolute, err := filepath.Abs(os.Args[0]); err == nil {
				return absolute, nil
			}
		}
	}

	path, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve executable path: %w", err)
	}
	return path, nil
}

func posixSingleQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func powerShellSingleQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

func managedScriptBlock(body string) string {
	trimmed := strings.TrimSpace(body)
	if trimmed == "" {
		return integrationStartMarker + "\n" + integrationEndMarker + "\n"
	}
	return integrationStartMarker + "\n" + trimmed + "\n" + integrationEndMarker + "\n"
}

func mergeManagedBlock(existing string, body string) string {
	block := managedScriptBlock(body)
	start := strings.Index(existing, integrationStartMarker)
	end := strings.Index(existing, integrationEndMarker)
	if start >= 0 && end > start {
		end += len(integrationEndMarker)
		remainder := strings.TrimLeft(existing[end:], "\r\n")
		prefix := strings.TrimRight(existing[:start], "\r\n")
		if prefix == "" {
			if remainder == "" {
				return block
			}
			return block + remainder
		}
		if remainder == "" {
			return prefix + "\n\n" + block
		}
		return prefix + "\n\n" + block + "\n" + remainder
	}

	trimmed := strings.TrimRight(existing, "\r\n")
	if trimmed == "" {
		return block
	}

	return trimmed + "\n\n" + block
}
