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
	binaryPath, err := integrationBinaryPath()
	if err != nil {
		return "", err
	}

	bashBinary := posixSingleQuote(binaryPath)
	powerShellBinary := powerShellSingleQuote(binaryPath)

	switch kind {
	case Zsh:
		return fmt.Sprintf(strings.TrimSpace(`function ennacmd() {
	local _ennacmd_binary=%s
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
}`), bashBinary) + "\n", nil
	case Bash:
		return fmt.Sprintf(strings.TrimSpace(`__ennacmd_widget() {
	local binary=%s
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

bind -x '"\C-g":__ennacmd_widget'`), bashBinary) + "\n", nil
	case Fish:
		return fmt.Sprintf(strings.TrimSpace(`function __ennacmd_widget
	    set -l binary %s
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

bind \cg __ennacmd_widget`), bashBinary) + "\n", nil
	case PowerShell:
		return fmt.Sprintf(strings.TrimSpace(`function ennacmd {
    param(
        [Parameter(ValueFromRemainingArguments = $true)]
        [string[]] $Arguments
    )

		$binary = %s
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

	script, err := IntegrationScript(kind)
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
