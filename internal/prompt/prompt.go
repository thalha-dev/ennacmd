package prompt

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/thalha-dev/ennacmd/internal/ai"
	"github.com/thalha-dev/ennacmd/internal/shell"
)

const sessionContextLimit = 6

type ContextTurn struct {
	Prompt   string
	Response string
}

func CommandRequest(activeShell shell.Kind, currentDir string, currentInput string, prior []ContextTurn, model string, temperature float64, streaming bool) ai.Prompt {
	targetOS := detectedOS()
	messages := []ai.Message{{
		Role: ai.RoleSystem,
		Content: strings.TrimSpace(fmt.Sprintf(`You are ennacmd, an AI shell command generator.
Return only executable shell commands.
Do not include markdown.
Do not include explanations unless requested.
Use syntax appropriate for the detected shell.
Prefer commands, flags, and path conventions appropriate for the detected operating system.
Do not assume Windows unless the detected operating system is Windows.
When the shell is PowerShell, prefer PowerShell-native commands.
When the shell is bash, zsh, or fish, prefer native Unix-style commands available on the detected operating system.
Prefer concise commands.
Prioritize correctness over creativity.
Detected operating system: %s.
Active shell: %s.`, targetOS, activeShell.DisplayName())),
	}}

	for _, entry := range trimContext(prior) {
		messages = append(messages,
			ai.Message{Role: ai.RoleUser, Content: entry.Prompt},
			ai.Message{Role: ai.RoleAssistant, Content: entry.Response},
		)
	}

	userMessage := strings.TrimSpace(fmt.Sprintf("Current working directory: %s\nRequest: %s", currentDir, currentInput))
	messages = append(messages, ai.Message{Role: ai.RoleUser, Content: userMessage})

	return ai.Prompt{
		Messages:    messages,
		Temperature: temperature,
		Model:       model,
		Stream:      streaming,
	}
}

func ExplainRequest(activeShell shell.Kind, command string, model string, temperature float64, streaming bool) ai.Prompt {
	targetOS := detectedOS()
	return ai.Prompt{
		Messages: []ai.Message{
			{
				Role: ai.RoleSystem,
				Content: strings.TrimSpace(fmt.Sprintf(`Explain shell commands for %s.
Operating system: %s.
Be concise.
Do not use markdown fences.
Do not add filler.`, activeShell.DisplayName(), targetOS)),
			},
			{
				Role:    ai.RoleUser,
				Content: fmt.Sprintf("Explain this command: %s", command),
			},
		},
		Temperature: temperature,
		Model:       model,
		Stream:      streaming,
	}
}

func detectedOS() string {
	switch runtime.GOOS {
	case "windows":
		return "Windows"
	case "darwin":
		return "macOS"
	case "linux":
		return "Linux"
	default:
		return runtime.GOOS
	}
}

func trimContext(entries []ContextTurn) []ContextTurn {
	if len(entries) <= sessionContextLimit {
		return entries
	}
	return entries[len(entries)-sessionContextLimit:]
}
