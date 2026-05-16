package ui

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/thalha-dev/ennacmd/internal/ai"
	"github.com/thalha-dev/ennacmd/internal/clipboard"
	"github.com/thalha-dev/ennacmd/internal/config"
	"github.com/thalha-dev/ennacmd/internal/prompt"
	"github.com/thalha-dev/ennacmd/internal/shell"
)

type mode int

const (
	inputMode mode = iota
	commandMode
)

type requestKind int

const (
	requestCommand requestKind = iota
	requestExplain
)

type streamStartedMsg struct {
	requestID int
	ch        <-chan ai.StreamEvent
}

type streamMsg struct {
	requestID int
	event     ai.StreamEvent
}

type requestErrMsg struct {
	requestID int
	err       error
}

type copyMsg struct {
	err error
}

type model struct {
	ctx           context.Context
	config        config.Config
	shell         shell.Kind
	provider      ai.Provider
	currentDir    string
	input         textinput.Model
	mode          mode
	width         int
	height        int
	status        string
	statusIsError bool
	statusTone    lipgloss.Style
	quitErr       error
	resultAction  Action

	conversation []prompt.ContextTurn

	displayPrompt string
	command       string
	explanation   string
	showExplain   bool

	pendingPrompt      string
	pendingKind        requestKind
	pendingReplaceLast bool
	requestID          int
	generating         bool
	streamCh           <-chan ai.StreamEvent
	requestCancel      context.CancelFunc

	styles styles
}

type styles struct {
	panel       lipgloss.Style
	panelFill   lipgloss.Style
	requestFill lipgloss.Style
	outputFill  lipgloss.Style
	explainFill lipgloss.Style
	inputFill   lipgloss.Style
	headerTitle lipgloss.Style
	meta        lipgloss.Style
	label       lipgloss.Style
	prompt      lipgloss.Style
	command     lipgloss.Style
	explain     lipgloss.Style
	input       lipgloss.Style
	inputShell  lipgloss.Style
	inputBox    lipgloss.Style
	requestBox  lipgloss.Style
	outputBox   lipgloss.Style
	explainBox  lipgloss.Style
	badgeShell  lipgloss.Style
	badgeModel  lipgloss.Style
	badgeInfo   lipgloss.Style
	badgeInput  lipgloss.Style
	badgeCmd    lipgloss.Style
	badgeBusy   lipgloss.Style
	hint        lipgloss.Style
	hintKey     lipgloss.Style
	hintText    lipgloss.Style
	statusOK    lipgloss.Style
	statusErr   lipgloss.Style
	placeholder lipgloss.Style
}

func newModel(ctx context.Context, options Options, currentDir string) *model {
	input := textinput.New()
	input.Prompt = "› "
	input.Placeholder = "describe the command you want"
	input.Focus()
	input.CharLimit = 0

	theme := styles{
		panel: lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#0f172a", Dark: "#e2e8f0"}).
			Padding(1, 2).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.AdaptiveColor{Light: "#b8c5db", Dark: "#20304a"}),
		panelFill:   lipgloss.NewStyle(),
		requestFill: lipgloss.NewStyle(),
		outputFill:  lipgloss.NewStyle(),
		explainFill: lipgloss.NewStyle(),
		inputFill:   lipgloss.NewStyle(),
		headerTitle: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "#0f172a", Dark: "#f8fafc"}),
		meta:        lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#66758b", Dark: "#8ea0b8"}),
		label: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.AdaptiveColor{Light: "#56a6b8", Dark: "#74d5e3"}),
		prompt:     lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#1e293b", Dark: "#d7e3f4"}),
		command:    lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#0f172a", Dark: "#f8fafc"}),
		explain:    lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#334155", Dark: "#d7e3f4"}),
		input:      lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#0f172a", Dark: "#f8fafc"}),
		inputShell: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "#56a6b8", Dark: "#7dd3fc"}),
		inputBox: lipgloss.NewStyle().
			Padding(0, 1).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.AdaptiveColor{Light: "#b9c7dc", Dark: "#2b5c88"}),
		requestBox: lipgloss.NewStyle().
			Padding(1, 2).
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.AdaptiveColor{Light: "#cbd5e1", Dark: "#1b2b42"}),
		outputBox: lipgloss.NewStyle().
			Padding(1, 2).
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.AdaptiveColor{Light: "#9db5d0", Dark: "#295e8b"}),
		explainBox: lipgloss.NewStyle().
			Padding(1, 2).
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.AdaptiveColor{Light: "#c0cad8", Dark: "#314055"}),
		badgeShell:  badgeStyle("#dceefe", "#123b61", "#9fd2ff", "#0d2d49"),
		badgeModel:  badgeStyle("#e7fbf7", "#0f4f49", "#99f6e4", "#083b39"),
		badgeInfo:   badgeStyle("#eef2ff", "#3b4d7a", "#c7d2fe", "#27314f"),
		badgeInput:  badgeStyle("#ecfccb", "#365314", "#bef264", "#254117"),
		badgeCmd:    badgeStyle("#fef3c7", "#7c2d12", "#fcd34d", "#5f250e"),
		badgeBusy:   badgeStyle("#ffe7c2", "#8a4b00", "#fdba74", "#6a3d05"),
		hint:        lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#64748b", Dark: "#91a4bc"}),
		hintKey:     badgeStyle("#f8fafc", "#334155", "#dbeafe", "#172235"),
		hintText:    lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#64748b", Dark: "#8ea0b8"}),
		statusOK:    lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "#0f766e", Dark: "#7ee7c7"}),
		statusErr:   lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "#b91c1c", Dark: "#fda4af"}),
		placeholder: lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#8ea0b8", Dark: "#64748b"}),
	}

	input.PromptStyle = styleOn(theme.inputShell, theme.inputFill)
	input.TextStyle = styleOn(theme.input, theme.inputFill)
	input.PlaceholderStyle = styleOn(theme.placeholder, theme.inputFill)
	input.Cursor.Style = styleOn(lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#38bdf8", Dark: "#7dd3fc"}), theme.inputFill)

	m := &model{
		ctx:        ctx,
		config:     options.Config,
		shell:      options.Shell,
		provider:   options.Provider,
		currentDir: currentDir,
		input:      input,
		mode:       inputMode,
		width:      80,
		height:     24,
		styles:     theme,
	}
	m.statusTone = m.styles.statusOK
	m.resizeInput()
	m.syncInputPlaceholder()
	m.setStatus("Enter submits. Generated commands never auto-run.", false)
	return m
}

func (m *model) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, tea.WindowSize())
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch typed := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = typed.Width
		m.height = typed.Height
		m.resizeInput()
		return m, nil
	case streamStartedMsg:
		if typed.requestID != m.requestID {
			return m, nil
		}
		m.streamCh = typed.ch
		return m, waitForStream(typed.requestID, typed.ch)
	case streamMsg:
		if typed.requestID != m.requestID {
			return m, nil
		}
		if typed.event.Err != nil {
			m.finishRequestError(typed.event.Err)
			return m, nil
		}
		if typed.event.Delta != "" {
			m.appendDelta(typed.event.Delta)
		}
		if typed.event.Done {
			m.finishRequestSuccess()
			return m, textinput.Blink
		}
		return m, waitForStream(typed.requestID, m.streamCh)
	case requestErrMsg:
		if typed.requestID == m.requestID {
			m.finishRequestError(typed.err)
		}
		return m, nil
	case copyMsg:
		if typed.err != nil {
			m.setStatus(typed.err.Error(), true)
			return m, nil
		}
		m.resultAction = ActionCopy
		return m, tea.Quit
	}

	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		if m.mode == inputMode {
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			return m, cmd
		}
		return m, nil
	}

	if m.generating {
		if keyMsg.Type == tea.KeyEsc || strings.EqualFold(keyMsg.String(), "esc") {
			m.cancelActiveRequest()
			m.quitErr = ErrCancelled
			m.resultAction = ActionCancel
			return m, tea.Quit
		}
		return m, nil
	}

	if keyMsg.Type == tea.KeyEsc || strings.EqualFold(keyMsg.String(), "esc") {
		m.quitErr = ErrCancelled
		m.resultAction = ActionCancel
		return m, tea.Quit
	}

	if m.mode == commandMode {
		return m.updateCommandMode(keyMsg)
	}
	return m.updateInputMode(keyMsg)
}

func (m *model) View() string {
	panelWidth := clampInt(m.width-14, 68, 108)
	panelHeight := clampInt(m.height-6, 18, 30)
	card := m.renderCard(panelWidth, panelHeight)
	if m.width <= 0 || m.height <= 0 {
		return card
	}
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, card)
}

func (m *model) updateInputMode(keyMsg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if keyMsg.Type == tea.KeyEnter {
		request := strings.TrimSpace(m.input.Value())
		if request == "" {
			return m, nil
		}
		m.input.SetValue("")
		return m.beginCommandRequest(request, false)
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(keyMsg)
	return m, cmd
}

func (m *model) updateCommandMode(keyMsg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case keyMsg.Type == tea.KeyEnter:
		if strings.TrimSpace(m.command) == "" {
			return m, nil
		}
		m.resultAction = ActionAccept
		return m, tea.Quit
	case isCopyShortcut(keyMsg):
		if strings.TrimSpace(m.command) == "" {
			return m, nil
		}
		return m, copyCommand(m.command)
	case isExplainShortcut(keyMsg):
		if strings.TrimSpace(m.command) == "" {
			return m, nil
		}
		if m.showExplain {
			m.showExplain = false
			m.setStatus("Command view", false)
			return m, nil
		}
		return m.beginExplainRequest()
	case isRefineRuneKey(keyMsg) && !m.showExplain:
		m.mode = inputMode
		m.input.Focus()
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(keyMsg)
		return m, cmd
	default:
		return m, nil
	}
}

func isCopyShortcut(keyMsg tea.KeyMsg) bool {
	return keyMsg.Type == tea.KeyCtrlC || strings.EqualFold(keyMsg.String(), "ctrl+c") || isControlRuneKey(keyMsg, 3)
}

func isExplainShortcut(keyMsg tea.KeyMsg) bool {
	return keyMsg.Type == tea.KeyCtrlE || strings.EqualFold(keyMsg.String(), "ctrl+e") || isControlRuneKey(keyMsg, 5)
}

func isRefineRuneKey(keyMsg tea.KeyMsg) bool {
	if keyMsg.Type != tea.KeyRunes || len(keyMsg.Runes) != 1 {
		return false
	}
	return unicode.IsPrint(keyMsg.Runes[0])
}

func isControlRuneKey(keyMsg tea.KeyMsg, value rune) bool {
	return keyMsg.Type == tea.KeyRunes && len(keyMsg.Runes) == 1 && keyMsg.Runes[0] == value
}

func (m *model) beginCommandRequest(request string, replaceLast bool) (tea.Model, tea.Cmd) {
	m.cancelActiveRequest()
	requestContext, cancel := context.WithCancel(m.ctx)
	m.requestCancel = cancel
	m.generating = true
	m.mode = commandMode
	m.input.Blur()
	m.pendingPrompt = request
	m.pendingKind = requestCommand
	m.pendingReplaceLast = replaceLast
	m.requestID++
	m.displayPrompt = request
	m.input.SetValue("")
	m.command = ""
	m.explanation = ""
	m.showExplain = false
	m.streamCh = nil
	m.syncInputPlaceholder()
	m.setStatus("Generating command...", false)

	builderPrompt := prompt.CommandRequest(m.shell, m.currentDir, request, m.contextForRequest(replaceLast), m.config.Model, m.config.Temperature, m.config.Streaming)
	return m, startProviderRequest(m.provider, requestContext, builderPrompt, m.requestID, m.config.Streaming)
}

func (m *model) beginExplainRequest() (tea.Model, tea.Cmd) {
	m.cancelActiveRequest()
	requestContext, cancel := context.WithCancel(m.ctx)
	m.requestCancel = cancel
	m.generating = true
	m.mode = commandMode
	m.pendingPrompt = fmt.Sprintf("Explain: %s", m.command)
	m.pendingKind = requestExplain
	m.pendingReplaceLast = false
	m.requestID++
	m.explanation = ""
	m.showExplain = true
	m.streamCh = nil
	m.setStatus("Explaining command...", false)

	explainPrompt := prompt.ExplainRequest(m.shell, m.command, m.config.Model, m.config.Temperature, m.config.Streaming)
	return m, startProviderRequest(m.provider, requestContext, explainPrompt, m.requestID, m.config.Streaming)
}

func (m *model) appendDelta(delta string) {
	if m.pendingKind == requestExplain {
		m.explanation += delta
		return
	}
	m.command += delta
}

func (m *model) finishRequestSuccess() {
	m.generating = false
	m.streamCh = nil
	m.cancelActiveRequest()
	m.mode = commandMode
	m.input.Blur()
	trimmedCommand := strings.TrimSpace(m.command)
	trimmedExplanation := strings.TrimSpace(m.explanation)

	switch m.pendingKind {
	case requestExplain:
		m.explanation = trimmedExplanation
		m.showExplain = true
		m.setStatus("Explanation ready. Ctrl+E returns to the command.", false)
	default:
		entry := prompt.ContextTurn{Prompt: m.pendingPrompt, Response: trimmedCommand}
		if m.pendingReplaceLast && len(m.conversation) > 0 {
			m.conversation[len(m.conversation)-1] = entry
		} else {
			m.conversation = append(m.conversation, entry)
		}
		m.displayPrompt = entry.Prompt
		m.command = entry.Response
		m.showExplain = false
		m.syncInputPlaceholder()
		m.setStatus("Enter inserts. Type to refine.", false)
	}
	m.resizeInput()
}

func (m *model) finishRequestError(err error) {
	m.generating = false
	m.streamCh = nil
	m.cancelActiveRequest()
	m.mode = inputMode
	m.input.Focus()
	m.syncInputPlaceholder()
	m.setStatus(err.Error(), true)
	if m.pendingKind == requestExplain {
		m.showExplain = false
	}
}

func (m *model) cancelActiveRequest() {
	if m.requestCancel != nil {
		m.requestCancel()
		m.requestCancel = nil
	}
}

func (m *model) resizeInput() {
	panelWidth := clampInt(m.width-14, 68, 108)
	bodyWidth := innerWidth(panelWidth, m.styles.panel, 28)
	inputWidth := innerWidth(bodyWidth, m.styles.inputBox, 16)
	if inputWidth < 16 {
		inputWidth = 16
	}
	m.input.Width = inputWidth
}

func (m *model) renderCard(panelWidth int, panelHeight int) string {
	bodyWidth := innerWidth(panelWidth, m.styles.panel, 28)
	blank := blankLineWithStyle(bodyWidth, m.styles.panelFill)
	card := lipgloss.JoinVertical(
		lipgloss.Left,
		m.renderHeader(bodyWidth),
		blank,
		renderPaddedTextLineOn("cwd  "+compactPath(m.currentDir, bodyWidth), bodyWidth, m.styles.meta, m.styles.panelFill),
		blank,
		m.renderRequestSection(bodyWidth),
		blank,
		m.renderOutputSection(bodyWidth),
		blank,
		m.renderInputSection(bodyWidth),
		blank,
		m.renderFooter(bodyWidth),
	)

	_ = panelHeight
	return m.styles.panel.
		Width(panelWidth).
		Render(card)
}

func (m *model) renderHeader(width int) string {
	title := styleOn(m.styles.headerTitle, m.styles.panelFill).Render("ennacmd")
	right := m.renderModeBadge()
	gap := width - lipgloss.Width(title) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	line1 := title + m.styles.panelFill.Render(strings.Repeat(" ", gap)) + right
	badges := []string{
		m.styles.badgeShell.Render(strings.ToLower(m.shell.DisplayName())),
		m.styles.badgeInfo.Render(strings.ToLower(m.config.Provider)),
		m.styles.badgeModel.Render(truncateRight(m.config.Model, 18)),
	}
	if m.generating {
		badges = append(badges, m.styles.badgeBusy.Render("streaming"))
	}
	line2 := wrapTokensOn(width, badges, m.styles.panelFill)
	return lipgloss.JoinVertical(lipgloss.Left, padRenderedLineWithStyle(line1, width, m.styles.panelFill), blankLineWithStyle(width, m.styles.panelFill), line2)
}

func (m *model) renderRequestSection(width int) string {
	label := renderPaddedTextLineOn("REQUEST", width, m.styles.label, m.styles.panelFill)
	value := strings.TrimSpace(m.displayPrompt)
	bodyWidth := innerWidth(width, m.styles.requestBox, 20)
	if value == "" {
		value = "Ask for one shell command in plain English."
		body := strings.Join(renderStyledLinesOn(value, bodyWidth, m.styles.placeholder, m.styles.requestFill), "\n")
		return lipgloss.JoinVertical(lipgloss.Left, label, m.styles.requestBox.Width(width).Render(body))
	}
	body := strings.Join(renderStyledLinesOn(value, bodyWidth, m.styles.prompt, m.styles.requestFill), "\n")
	return lipgloss.JoinVertical(lipgloss.Left, label, m.styles.requestBox.Width(width).Render(body))
}

func (m *model) renderOutputSection(width int) string {
	label := renderPaddedTextLineOn(strings.ToUpper(m.outputLabel()), width, m.styles.label, m.styles.panelFill)
	text := strings.TrimSpace(m.command)
	textStyle := m.styles.command
	boxStyle := m.styles.outputBox
	fillStyle := m.styles.outputFill
	if m.showExplain {
		text = strings.TrimSpace(m.explanation)
		textStyle = m.styles.explain
		boxStyle = m.styles.explainBox
		fillStyle = m.styles.explainFill
	}
	if m.generating {
		text = "streaming response..."
		if m.pendingKind == requestExplain {
			text = strings.TrimSpace(m.explanation)
			if text == "" {
				text = "explaining command..."
			}
			textStyle = m.styles.explain
			boxStyle = m.styles.explainBox
			fillStyle = m.styles.explainFill
		} else if partial := strings.TrimSpace(m.command); partial != "" {
			text = partial
		}
	}
	if text == "" {
		text = "Generated shell commands appear here. They are inserted only after you accept them."
		textStyle = m.styles.placeholder
	}
	bodyWidth := innerWidth(width, boxStyle, 20)
	body := strings.Join(renderStyledLinesOn(text, bodyWidth, textStyle, fillStyle), "\n")
	return lipgloss.JoinVertical(lipgloss.Left, label, boxStyle.Width(width).Render(body))
}

func (m *model) renderFooter(width int) string {
	hintLine := wrapTokensOn(width, m.renderHintTokens(), m.styles.panelFill)
	status := strings.TrimSpace(m.status)
	footerLines := []string{hintLine}
	if status != "" && m.statusIsError {
		footerLines = append(footerLines, blankLineWithStyle(width, m.styles.panelFill), renderPaddedTextLineOn(status, width, m.statusTone, m.styles.panelFill))
	}
	return lipgloss.JoinVertical(lipgloss.Left, footerLines...)
}

func (m *model) renderInputSection(width int) string {
	label := renderPaddedTextLineOn("PROMPT", width, m.styles.label, m.styles.panelFill)
	inputBodyWidth := innerWidth(width, m.styles.inputBox, 16)
	inputBody := padRenderedLineWithStyle(m.input.View(), inputBodyWidth, m.styles.inputFill)
	caption := renderPaddedTextLineOn("Write a new request or refine the current command. Acceptance never auto-runs anything.", width, m.styles.meta, m.styles.panelFill)
	box := m.styles.inputBox.Width(width).Render(inputBody)
	return lipgloss.JoinVertical(lipgloss.Left, label, box, caption)
}

func (m *model) renderPopupLines() []string {
	maxBodyWidth := clampInt(m.width-16, 34, 72)

	header := lipgloss.JoinHorizontal(lipgloss.Left,
		lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "#0f172a", Dark: "#f8fafc"}).Render("ennacmd"),
		m.styles.meta.Render("  "+strings.ToLower(m.shell.DisplayName())),
	)

	content := []string{header}
	if m.displayPrompt != "" {
		content = append(content, "", m.styles.label.Render("request"))
		content = append(content, m.wrapStyled(m.displayPrompt, maxBodyWidth, m.styles.prompt)...)
	}

	content = append(content, "", m.styles.label.Render(m.outputLabel()))
	content = append(content, m.renderOutput(maxBodyWidth)...)
	content = append(content, "", m.styles.input.Render(m.input.View()))
	content = append(content, m.styles.hint.Render(m.hints()))
	if status := strings.TrimSpace(m.status); status != "" && m.statusIsError {
		content = append(content, m.statusTone.Render(status))
	}

	contentWidth := maxInt(36, maxLineWidth(content))
	panelWidth := contentWidth + 4
	blank := m.styles.panel.Render(strings.Repeat(" ", panelWidth))
	lines := []string{blank}
	for _, line := range content {
		inner := "  " + padStyledLine(line, contentWidth) + "  "
		lines = append(lines, m.styles.panel.Render(inner))
	}
	lines = append(lines, blank)
	return lines
}

func (m *model) renderOutput(width int) []string {
	text := strings.TrimSpace(m.command)
	style := m.styles.command
	if m.showExplain {
		text = strings.TrimSpace(m.explanation)
		style = m.styles.explain
	}
	if m.generating {
		text = "streaming..."
		if m.pendingKind == requestExplain {
			text = strings.TrimSpace(m.explanation)
			if text == "" {
				text = "explaining..."
			}
			style = m.styles.explain
		} else if partial := strings.TrimSpace(m.command); partial != "" {
			text = partial
		}
	}
	if text == "" {
		text = "Type a request below and ennacmd will stream back a shell command only."
		style = m.styles.placeholder
	}

	plainLines := wrapPlain(text, maxInt(16, width-2))
	styled := make([]string, 0, len(plainLines))
	for _, line := range plainLines {
		styled = append(styled, "  "+style.Render(line))
	}
	return styled
}

func (m *model) renderModeBadge() string {
	if m.generating {
		return m.styles.badgeBusy.Render("generating")
	}
	if m.mode == commandMode {
		return m.styles.badgeCmd.Render("command mode")
	}
	return m.styles.badgeInput.Render("input mode")
}

func (m *model) renderHintTokens() []string {
	if m.generating {
		return []string{m.renderHintToken("esc", "cancel")}
	}
	if m.mode == inputMode {
		return []string{m.renderHintToken("enter", "send"), m.renderHintToken("esc", "close")}
	}
	if m.showExplain {
		return []string{
			m.renderHintToken("enter", "insert"),
			m.renderHintToken("ctrl+c", "copy"),
			m.renderHintToken("ctrl+e", "command"),
			m.renderHintToken("esc", "close"),
		}
	}
	return []string{
		m.renderHintToken("enter", "insert"),
		m.renderHintToken("type", "refine"),
		m.renderHintToken("ctrl+c", "copy"),
		m.renderHintToken("ctrl+e", "explain"),
		m.renderHintToken("esc", "close"),
	}
}

func (m *model) renderHintToken(key string, text string) string {
	return m.styles.hintKey.Render(strings.ToUpper(key)) + m.styles.panelFill.Render(" ") + styleOn(m.styles.hintText, m.styles.panelFill).Render(text)
}

func (m *model) syncInputPlaceholder() {
	if m.mode == commandMode || strings.TrimSpace(m.command) != "" {
		m.input.Placeholder = "refine the command or ask a follow-up"
		return
	}
	m.input.Placeholder = "describe the command you want"
}

func (m *model) wrapStyled(text string, width int, style lipgloss.Style) []string {
	wrapped := wrapPlain(text, width)
	styled := make([]string, 0, len(wrapped))
	for _, line := range wrapped {
		styled = append(styled, "  "+style.Render(padStyledLine(line, width)))
	}
	return styled
}

func (m *model) outputLabel() string {
	if m.showExplain {
		return "explanation"
	}
	return "command"
}

func (m *model) hints() string {
	if m.generating {
		return "esc cancel"
	}
	if m.mode == inputMode {
		return "enter send  esc close"
	}
	if m.showExplain {
		return "enter insert  ctrl+c copy  ctrl+e command  esc close"
	}
	return "enter insert  type refine  ctrl+c copy  ctrl+e explain  esc close"
}

func (m *model) contextForRequest(replaceLast bool) []prompt.ContextTurn {
	contextEntries := append([]prompt.ContextTurn(nil), m.conversation...)
	if replaceLast && len(contextEntries) > 0 {
		return contextEntries[:len(contextEntries)-1]
	}
	return contextEntries
}

func (m *model) setStatus(message string, isError bool) {
	m.status = strings.TrimSpace(message)
	m.statusIsError = isError
	if isError {
		m.statusTone = m.styles.statusErr
		return
	}
	m.statusTone = m.styles.statusOK
}

func (m *model) result() Result {
	return Result{
		Action:  m.resultAction,
		Command: strings.TrimSpace(m.command),
	}
}

func startProviderRequest(provider ai.Provider, ctx context.Context, prompt ai.Prompt, requestID int, streaming bool) tea.Cmd {
	return func() tea.Msg {
		if streaming {
			ch, err := provider.Stream(ctx, prompt)
			if err != nil {
				return requestErrMsg{requestID: requestID, err: err}
			}
			return streamStartedMsg{requestID: requestID, ch: ch}
		}

		response, err := provider.Complete(ctx, prompt)
		if err != nil {
			return requestErrMsg{requestID: requestID, err: err}
		}
		buffer := make(chan ai.StreamEvent, 2)
		buffer <- ai.StreamEvent{Delta: response.Content}
		buffer <- ai.StreamEvent{Done: true}
		close(buffer)
		return streamStartedMsg{requestID: requestID, ch: buffer}
	}
}

func waitForStream(requestID int, ch <-chan ai.StreamEvent) tea.Cmd {
	return func() tea.Msg {
		event, ok := <-ch
		if !ok {
			return streamMsg{requestID: requestID, event: ai.StreamEvent{Done: true}}
		}
		return streamMsg{requestID: requestID, event: event}
	}
}

func copyCommand(command string) tea.Cmd {
	return func() tea.Msg {
		return copyMsg{err: clipboard.Copy(command)}
	}
}

func badgeStyle(lightBG string, lightFG string, darkFG string, darkBG string) lipgloss.Style {
	return lipgloss.NewStyle().
		Padding(0, 1).
		Bold(true).
		Foreground(lipgloss.AdaptiveColor{Light: lightFG, Dark: darkFG})
}

func renderStyledLines(text string, width int, style lipgloss.Style) []string {
	lines := wrapPlain(text, width)
	styled := make([]string, 0, len(lines))
	for _, line := range lines {
		styled = append(styled, style.Render(padStyledLine(line, width)))
	}
	return styled
}

func renderStyledLinesOn(text string, width int, style lipgloss.Style, fillStyle lipgloss.Style) []string {
	lines := wrapPlain(text, width)
	styled := make([]string, 0, len(lines))
	styledStyle := styleOn(style, fillStyle)
	for _, line := range lines {
		styled = append(styled, styledStyle.Render(padStyledLine(line, width)))
	}
	return styled
}

func renderPaddedTextLineOn(text string, width int, style lipgloss.Style, fillStyle lipgloss.Style) string {
	return styleOn(style, fillStyle).Render(padStyledLine(text, width))
}

func styleOn(style lipgloss.Style, fillStyle lipgloss.Style) lipgloss.Style {
	return style.Copy().Background(fillStyle.GetBackground())
}

func padRenderedBlock(block string, width int) string {
	if width <= 0 {
		return block
	}
	lines := strings.Split(strings.ReplaceAll(block, "\r\n", "\n"), "\n")
	padded := make([]string, 0, len(lines))
	for _, line := range lines {
		padded = append(padded, padStyledLine(line, width))
	}
	return strings.Join(padded, "\n")
}

func blankLine(width int) string {
	if width <= 0 {
		return ""
	}
	return strings.Repeat(" ", width)
}

func blankLineWithStyle(width int, fillStyle lipgloss.Style) string {
	if width <= 0 {
		return ""
	}
	return fillStyle.Render(strings.Repeat(" ", width))
}

func padRenderedLineWithStyle(line string, width int, fillStyle lipgloss.Style) string {
	current := lipgloss.Width(line)
	if current >= width {
		return line
	}
	return line + fillStyle.Render(strings.Repeat(" ", width-current))
}

func wrapTokensOn(width int, tokens []string, fillStyle lipgloss.Style) string {
	if len(tokens) == 0 {
		return blankLineWithStyle(width, fillStyle)
	}
	separator := fillStyle.Render("  ")
	lines := make([]string, 0, 2)
	current := tokens[0]
	for _, token := range tokens[1:] {
		candidate := current + separator + token
		if lipgloss.Width(candidate) > width {
			lines = append(lines, padRenderedLineWithStyle(current, width, fillStyle))
			current = token
			continue
		}
		current = candidate
	}
	lines = append(lines, padRenderedLineWithStyle(current, width, fillStyle))
	return strings.Join(lines, "\n")
}

func innerWidth(totalWidth int, style lipgloss.Style, minimum int) int {
	inner := totalWidth - style.GetHorizontalFrameSize()
	if inner < minimum {
		return minimum
	}
	return inner
}

func wrapTokens(width int, tokens []string) string {
	if len(tokens) == 0 {
		return ""
	}
	lines := make([]string, 0, 2)
	current := ""
	for _, token := range tokens {
		if current == "" {
			current = token
			continue
		}
		candidate := current + "  " + token
		if lipgloss.Width(candidate) > width {
			lines = append(lines, current)
			current = token
			continue
		}
		current = candidate
	}
	if current != "" {
		lines = append(lines, current)
	}
	return strings.Join(lines, "\n")
}

func compactPath(path string, width int) string {
	clean := filepath.Clean(path)
	if lipgloss.Width(clean) <= width {
		return clean
	}
	base := filepath.Base(clean)
	parent := filepath.Base(filepath.Dir(clean))
	short := filepath.Join("...", parent, base)
	if lipgloss.Width(short) <= width {
		return short
	}
	return truncateLeft(clean, width)
}

func truncateLeft(text string, width int) string {
	runes := []rune(text)
	if len(runes) <= width {
		return text
	}
	if width <= 3 {
		return string(runes[len(runes)-width:])
	}
	return "..." + string(runes[len(runes)-(width-3):])
}

func truncateRight(text string, width int) string {
	runes := []rune(text)
	if len(runes) <= width {
		return text
	}
	if width <= 3 {
		return string(runes[:width])
	}
	return string(runes[:width-3]) + "..."
}

func wrapPlain(text string, width int) []string {
	if width <= 0 {
		return []string{text}
	}
	segments := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	wrapped := make([]string, 0, len(segments))
	for _, segment := range segments {
		if segment == "" {
			wrapped = append(wrapped, "")
			continue
		}
		for lipgloss.Width(segment) > width {
			cut := width
			for cut > 1 && !utf8.ValidString(segment[:cut]) {
				cut--
			}
			if space := strings.LastIndex(segment[:cut], " "); space > width/3 {
				cut = space
			}
			wrapped = append(wrapped, strings.TrimSpace(segment[:cut]))
			segment = strings.TrimSpace(segment[cut:])
		}
		wrapped = append(wrapped, segment)
	}
	if len(wrapped) == 0 {
		return []string{""}
	}
	return wrapped
}

func maxLineWidth(lines []string) int {
	maxWidth := 0
	for _, line := range lines {
		if width := lipgloss.Width(line); width > maxWidth {
			maxWidth = width
		}
	}
	return maxWidth
}

func padStyledLine(line string, width int) string {
	current := lipgloss.Width(line)
	if current >= width {
		return line
	}
	return line + strings.Repeat(" ", width-current)
}

func clampInt(value int, lower int, upper int) int {
	if value < lower {
		return lower
	}
	if value > upper {
		return upper
	}
	return value
}

func maxInt(first int, second int) int {
	if first > second {
		return first
	}
	return second
}
