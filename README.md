# ennacmd

`ennacmd` is a terminal-first assistant for generating, refining, and explaining shell commands.

It opens a focused TUI, converts plain-English intent into executable shell commands, and lets you review, refine, copy, or accept the result. It does not run commands for you.

## Highlights

- Terminal UI built with Bubble Tea and Lip Gloss
- Shell-aware command generation for PowerShell, bash, zsh, and fish
- Provider support for OpenAI-compatible APIs, OpenRouter, and Ollama
- First-run setup flow with live provider validation
- Command refinement, explanation, copy, and accept flows
- Windows prompt prefilling when a command is accepted

## Installation

### Install with Go

Because the repository root is now the executable package, the install command is the clean public form:

```bash
go install github.com/thalha-dev/ennacmd@latest
```

To install a specific tagged release:

```bash
go install github.com/thalha-dev/ennacmd@v0.1.0
```

### Build from source

```powershell
go build ./...
New-Item -ItemType Directory -Force .\bin | Out-Null
go build -o .\bin\ennacmd.exe .
```

## Quick Start

Run:

```text
ennacmd
```

On first launch, `ennacmd` opens interactive setup when the config is missing or incomplete. The setup flow:

- lets you choose a provider
- asks for the model name
- collects provider-specific settings such as API keys and base URLs
- validates the provider configuration before saving it
- opens the main UI immediately after successful setup

To rerun setup later:

```text
ennacmd setup
```

## Commands

```text
ennacmd
ennacmd setup
ennacmd version
```

## Keyboard Controls

- `Enter` in input mode sends your request
- `Enter` in command mode accepts the current command
- `Type` in command mode starts a refinement
- `Ctrl+C` copies the current command and closes
- `Ctrl+E` shows an explanation for the current command
- `Esc` closes the UI

## Configuration

`ennacmd` stores user data in:

- `~/.ennacmd/config.yaml`
- `~/.ennacmd/cache/`

Default configuration:

```yaml
provider: openai
base_url: https://api.openai.com/v1
api_key: ""
model: gpt-4o-mini
temperature: 0.2
shell: auto
streaming: true
```

Provider notes:

- `openai` requires `api_key`
- `openrouter` uses `provider: openrouter` and `base_url: https://openrouter.ai/api/v1`
- `ollama` uses `provider: ollama` and typically `base_url: http://localhost:11434`

You can edit `config.yaml` manually after setup if needed.

## Behavior

Command generation is constrained intentionally:

- output is shell command text, not markdown
- explanations are only returned when requested
- syntax follows the detected shell
- commands are adapted to the current operating system

On Windows PowerShell, accepted commands can be prefixed back into the active prompt instead of being executed automatically.

## Development

Project layout:

```text
main.go
internal/
  ai/
  app/
  clipboard/
  config/
  prompt/
  provider/
  shell/
  terminal/
  ui/
```

Useful local commands:

```powershell
go build ./...
go test ./...
go build -o .\bin\ennacmd.exe .
```

## Releases

This repository is set up for automatic tagged releases with GitHub Actions and GoReleaser.

Push a semantic version tag like `v0.1.0`, and the workflow will:

- run tests
- build release binaries for Windows, Linux, and macOS
- create or update the GitHub Release
- upload release archives and checksums automatically

The public install command is:

```bash
go install github.com/thalha-dev/ennacmd@latest
```
