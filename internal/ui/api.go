package ui

import (
	"context"
	"errors"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/thalha-dev/ennacmd/internal/ai"
	"github.com/thalha-dev/ennacmd/internal/config"
	"github.com/thalha-dev/ennacmd/internal/shell"
)

var ErrCancelled = errors.New("interaction cancelled")

type Action string

const (
	ActionCancel Action = "cancel"
	ActionAccept Action = "accept"
	ActionCopy   Action = "copy"
)

type Result struct {
	Action      Action
	Command     string
	AnchorRow   int
	AnchorCol   int
	PopupWidth  int
	PopupHeight int
}

type Options struct {
	Config   config.Config
	Shell    shell.Kind
	Provider ai.Provider
	Input    *os.File
	Output   *os.File
}

type InitOptions struct {
	Config config.Config
	Input  *os.File
	Output *os.File
}

func Run(ctx context.Context, options Options) (Result, error) {
	currentDir, err := os.Getwd()
	if err != nil {
		currentDir = "."
	}

	input := options.Input
	if input == nil {
		input = os.Stdin
	}
	output := options.Output
	if output == nil {
		output = os.Stdout
	}

	uiModel := newModel(ctx, options, currentDir)
	program := tea.NewProgram(uiModel,
		tea.WithContext(ctx),
		tea.WithInput(input),
		tea.WithOutput(output),
		tea.WithAltScreen(),
	)

	finalModel, err := program.Run()
	if err != nil {
		return uiModel.result(), err
	}

	completed, ok := finalModel.(*model)
	if !ok {
		return Result{}, errors.New("unexpected final ui model")
	}

	return completed.result(), completed.quitErr
}

func RunInit(ctx context.Context, options InitOptions) (config.Config, error) {
	input := options.Input
	if input == nil {
		input = os.Stdin
	}
	output := options.Output
	if output == nil {
		output = os.Stdout
	}

	setupModel := newInitModel(ctx, options)
	program := tea.NewProgram(setupModel,
		tea.WithContext(ctx),
		tea.WithInput(input),
		tea.WithOutput(output),
		tea.WithAltScreen(),
	)

	finalModel, err := program.Run()
	if err != nil {
		return setupModel.result(), err
	}

	completed, ok := finalModel.(*initModel)
	if !ok {
		return config.Config{}, errors.New("unexpected final init ui model")
	}

	return completed.result(), completed.quitErr
}
