package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/thalha-dev/ennacmd/internal/config"
	"github.com/thalha-dev/ennacmd/internal/provider"
	"github.com/thalha-dev/ennacmd/internal/shell"
	"github.com/thalha-dev/ennacmd/internal/terminal"
	"github.com/thalha-dev/ennacmd/internal/ui"
)

const version = "0.1.0"

type runOptions struct {
	capture bool
}

func Run(args []string) error {
	options, remainingArgs := parseRunOptions(args)

	if len(remainingArgs) > 0 {
		switch remainingArgs[0] {
		case "__insert-helper":
			return terminal.RunInsertHelper(remainingArgs[1:])
		case "shell-init":
			return runShellInit(remainingArgs[1:])
		case "shell-install":
			return runShellInstall(remainingArgs[1:])
		case "setup":
			loaded, err := config.Load()
			if err != nil {
				return err
			}
			return runSetup(loaded, options)
		case "version", "--version", "-v":
			_, err := fmt.Fprintln(os.Stdout, version)
			return err
		case "help", "--help", "-h":
			printUsage(os.Stdout)
			return nil
		}
	}

	loaded, err := config.Load()
	if err != nil {
		return err
	}
	if err := loaded.Validate(); err != nil {
		return runSetup(loaded, options)
	}

	return runInteractive(loaded, options)
}

func parseRunOptions(args []string) (runOptions, []string) {
	options := runOptions{}
	remainingArgs := make([]string, 0, len(args))
	for _, arg := range args {
		switch arg {
		case "--capture":
			options.capture = true
		default:
			remainingArgs = append(remainingArgs, arg)
		}
	}
	return options, remainingArgs
}

func runShellInit(args []string) error {
	kind, err := resolveShellKind(args)
	if err != nil {
		return err
	}

	script, err := shell.IntegrationScript(kind)
	if err != nil {
		return err
	}

	_, err = fmt.Fprint(os.Stdout, script)
	return err
}

func runShellInstall(args []string) error {
	kind, err := resolveShellKind(args)
	if err != nil {
		return err
	}

	path, err := shell.InstallIntegration(kind)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(os.Stdout, "installed %s shell integration at %s\nrerun this command if you move the ennacmd binary\n", kind.DisplayName(), path)
	return err
}

func resolveShellKind(args []string) (shell.Kind, error) {
	if len(args) > 1 {
		return shell.Auto, fmt.Errorf("expected zero or one shell argument")
	}

	if len(args) == 1 {
		kind := shell.Normalize(args[0])
		if kind == shell.Auto {
			return shell.Auto, fmt.Errorf("unsupported shell %q", args[0])
		}
		return kind, nil
	}

	kind := shell.Detect("")
	if kind == shell.Auto {
		return shell.Auto, fmt.Errorf("could not detect shell")
	}
	return kind, nil
}

func runInteractive(loaded config.Config, options runOptions) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	uiOptions := ui.Options{Config: loaded}
	if options.capture {
		tty, err := terminal.OpenTTY()
		if err != nil {
			return err
		}
		defer tty.Close()
		uiOptions.Input = tty.In
		uiOptions.Output = tty.Out
	}

	activeShell := shell.Detect(loaded.Shell)
	loaded.Shell = string(activeShell)
	uiOptions.Config = loaded
	uiOptions.Shell = activeShell

	if err := loaded.Validate(); err != nil {
		return err
	}

	aiProvider, err := provider.New(loaded)
	if err != nil {
		return err
	}
	uiOptions.Provider = aiProvider

	result, err := ui.Run(ctx, uiOptions)
	cancelled := errors.Is(err, ui.ErrCancelled)
	if err != nil && !cancelled {
		return err
	}
	if cancelled {
		return nil
	}

	if result.Action == ui.ActionAccept && strings.TrimSpace(result.Command) != "" {
		if options.capture {
			_, err := fmt.Fprintln(os.Stdout, result.Command)
			return err
		}
		if err := terminal.QueueInput(os.Stdin, result.Command); err != nil {
			return err
		}
	}

	return err
}

func runSetup(loaded config.Config, options runOptions) error {
	activeShell := shell.Detect(loaded.Shell)
	loaded.Shell = string(activeShell)
	initOptions := ui.InitOptions{Config: loaded}
	if options.capture {
		tty, err := terminal.OpenTTY()
		if err != nil {
			return err
		}
		defer tty.Close()
		initOptions.Input = tty.In
		initOptions.Output = tty.Out
	}

	configured, err := ui.RunInit(context.Background(), initOptions)
	cancelled := errors.Is(err, ui.ErrCancelled)
	if err != nil && !cancelled {
		return err
	}
	if cancelled {
		return nil
	}

	return runInteractive(configured, options)
}

func printUsage(out *os.File) {
	_, _ = fmt.Fprintln(out, "ennacmd")
	_, _ = fmt.Fprintln(out, "")
	_, _ = fmt.Fprintln(out, "Usage:")
	_, _ = fmt.Fprintln(out, "  ennacmd")
	_, _ = fmt.Fprintln(out, "    opens setup automatically when config is incomplete, otherwise opens the command UI")
	_, _ = fmt.Fprintln(out, "  ennacmd --capture")
	_, _ = fmt.Fprintln(out, "    runs the UI on the controlling terminal and prints the accepted command to stdout")
	_, _ = fmt.Fprintln(out, "  ennacmd shell-init [shell]")
	_, _ = fmt.Fprintln(out, "    prints shell integration for zsh, bash, fish, or powershell")
	_, _ = fmt.Fprintln(out, "  ennacmd shell-install [shell]")
	_, _ = fmt.Fprintln(out, "    installs the supported shell integration into your shell profile")
	_, _ = fmt.Fprintln(out, "  ennacmd setup")
	_, _ = fmt.Fprintln(out, "    reruns setup so you can update provider settings")
	_, _ = fmt.Fprintln(out, "  ennacmd version")
}
