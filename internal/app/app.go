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
	_, _ = fmt.Fprintln(out, "  ennacmd setup")
	_, _ = fmt.Fprintln(out, "    reruns setup so you can update provider settings")
	_, _ = fmt.Fprintln(out, "  ennacmd version")
}
