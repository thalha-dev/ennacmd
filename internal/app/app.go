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

func Run(args []string) error {
	if len(args) > 0 {
		switch args[0] {
		case "__insert-helper":
			return terminal.RunInsertHelper(args[1:])
		case "setup":
			loaded, err := config.Load()
			if err != nil {
				return err
			}
			return runSetup(loaded)
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
		return runSetup(loaded)
	}

	return runInteractive(loaded)
}

func runInteractive(loaded config.Config) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	uiOptions := ui.Options{Config: loaded}

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
		if err := terminal.QueueInput(os.Stdin, result.Command); err != nil {
			return err
		}
	}

	return err
}

func runSetup(loaded config.Config) error {
	activeShell := shell.Detect(loaded.Shell)
	loaded.Shell = string(activeShell)
	initOptions := ui.InitOptions{Config: loaded}

	configured, err := ui.RunInit(context.Background(), initOptions)
	cancelled := errors.Is(err, ui.ErrCancelled)
	if err != nil && !cancelled {
		return err
	}
	if cancelled {
		return nil
	}

	return runInteractive(configured)
}

func printUsage(out *os.File) {
	_, _ = fmt.Fprintln(out, "ennacmd")
	_, _ = fmt.Fprintln(out, "")
	_, _ = fmt.Fprintln(out, "Usage:")
	_, _ = fmt.Fprintln(out, "  ennacmd")
	_, _ = fmt.Fprintln(out, "    opens setup automatically when config is incomplete, otherwise opens the command UI")
	_, _ = fmt.Fprintln(out, "  ennacmd setup")
	_, _ = fmt.Fprintln(out, "    reruns setup so you can update provider settings")
	_, _ = fmt.Fprintln(out, "  ennacmd version")
}
