package ui

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/thalha-dev/ennacmd/internal/ai"
	"github.com/thalha-dev/ennacmd/internal/config"
	"github.com/thalha-dev/ennacmd/internal/provider"
)

type initStep int

const (
	initStepProvider initStep = iota
	initStepModel
	initStepFields
)

type initProviderChoice struct {
	ID           string
	Title        string
	Description  string
	DefaultURL   string
	DefaultModel string
}

type initFieldSpec struct {
	Key         string
	Label       string
	Placeholder string
	Help        string
	Required    bool
	Sensitive   bool
}

type initValidationMsg struct {
	config config.Config
	err    error
}

type initStyles struct {
	panel              lipgloss.Style
	panelFill          lipgloss.Style
	providerFill       lipgloss.Style
	providerFillActive lipgloss.Style
	fieldFill          lipgloss.Style
	fieldFillActive    lipgloss.Style
	inputFill          lipgloss.Style
	headerTitle        lipgloss.Style
	meta               lipgloss.Style
	label              lipgloss.Style
	description        lipgloss.Style
	providerBox        lipgloss.Style
	providerBoxActive  lipgloss.Style
	fieldBox           lipgloss.Style
	fieldBoxActive     lipgloss.Style
	input              lipgloss.Style
	inputShell         lipgloss.Style
	placeholder        lipgloss.Style
	hint               lipgloss.Style
	hintKey            lipgloss.Style
	hintText           lipgloss.Style
	statusOK           lipgloss.Style
	statusErr          lipgloss.Style
	badgeInfo          lipgloss.Style
	badgeModel         lipgloss.Style
	badgeBusy          lipgloss.Style
	selectionMarker    lipgloss.Style
	selectionInactive  lipgloss.Style
}

type initModel struct {
	ctx           context.Context
	config        config.Config
	resultConfig  config.Config
	providers     []initProviderChoice
	providerIndex int
	modelInput    textinput.Model
	fieldSpecs    []initFieldSpec
	fieldInputs   []textinput.Model
	activeField   int
	step          initStep
	width         int
	height        int
	status        string
	statusIsError bool
	statusTone    lipgloss.Style
	validating    bool
	quitErr       error
	styles        initStyles
}

func newInitModel(ctx context.Context, options InitOptions) *initModel {
	styles := newInitStyles()
	providers := initProviderChoices()
	draft := normalizeInitConfig(options.Config)
	providerIndex := providerChoiceIndex(providers, draft.Provider)
	if providerIndex < 0 {
		providerIndex = 0
		draft = normalizeInitConfig(config.DefaultForProvider(providers[0].ID))
		draft.Paths = options.Config.Paths
	}

	modelInput := newInitTextInput(styles, "enter the model name exposed by the provider")
	modelInput.SetValue(strings.TrimSpace(draft.Model))
	modelInput.Blur()

	m := &initModel{
		ctx:           ctx,
		config:        draft,
		providers:     providers,
		providerIndex: providerIndex,
		modelInput:    modelInput,
		step:          initStepProvider,
		width:         80,
		height:        24,
		styles:        styles,
	}
	m.statusTone = m.styles.statusOK
	m.rebuildFieldInputs()
	m.setStatus("Select a provider, then press Enter.", false)
	return m
}

func (m *initModel) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, tea.WindowSize())
}

func (m *initModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch typed := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = typed.Width
		m.height = typed.Height
		m.resizeInputs()
		return m, nil
	case initValidationMsg:
		m.validating = false
		if typed.err != nil {
			m.setStatus(typed.err.Error(), true)
			m.focusActiveField()
			return m, textinput.Blink
		}
		m.resultConfig = typed.config
		m.setStatus("Setup verified. Opening ennacmd...", false)
		return m, tea.Quit
	}

	if m.validating {
		return m, nil
	}

	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		switch m.step {
		case initStepModel:
			var cmd tea.Cmd
			m.modelInput, cmd = m.modelInput.Update(msg)
			return m, cmd
		case initStepFields:
			if len(m.fieldInputs) == 0 {
				return m, nil
			}
			var cmd tea.Cmd
			m.fieldInputs[m.activeField], cmd = m.fieldInputs[m.activeField].Update(msg)
			return m, cmd
		default:
			return m, nil
		}
	}

	if keyMsg.Type == tea.KeyEsc || strings.EqualFold(keyMsg.String(), "esc") {
		m.quitErr = ErrCancelled
		return m, tea.Quit
	}

	switch m.step {
	case initStepProvider:
		return m.updateProviderStep(keyMsg)
	case initStepModel:
		return m.updateModelStep(keyMsg)
	case initStepFields:
		return m.updateFieldsStep(keyMsg)
	default:
		return m, nil
	}
}

func (m *initModel) View() string {
	panelWidth := clampInt(m.width-14, 72, 110)
	bodyWidth := innerWidth(panelWidth, m.styles.panel, 28)
	blank := blankLineWithStyle(bodyWidth, m.styles.panelFill)
	body := lipgloss.JoinVertical(
		lipgloss.Left,
		m.renderHeader(bodyWidth),
		blank,
		renderPaddedTextLineOn(m.stepSummary(), bodyWidth, m.styles.meta, m.styles.panelFill),
		blank,
		m.renderBody(bodyWidth),
		blank,
		m.renderFooter(bodyWidth),
	)
	card := m.styles.panel.Width(panelWidth).Render(body)
	if m.width <= 0 || m.height <= 0 {
		return card
	}
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, card)
}

func (m *initModel) result() config.Config {
	if m.resultConfig.Provider != "" {
		return m.resultConfig
	}
	return m.config
}

func (m *initModel) updateProviderStep(keyMsg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case keyMsg.Type == tea.KeyEnter:
		m.applySelectedProvider()
		m.step = initStepModel
		m.modelInput.Focus()
		m.resizeInputs()
		m.setStatus("Enter the exact model name for the selected provider.", false)
		return m, textinput.Blink
	case keyMsg.Type == tea.KeyUp || keyMsg.Type == tea.KeyLeft || strings.EqualFold(keyMsg.String(), "k"):
		if m.providerIndex > 0 {
			m.providerIndex--
		}
		m.setStatus("Select a provider, then press Enter.", false)
		return m, nil
	case keyMsg.Type == tea.KeyDown || keyMsg.Type == tea.KeyRight || strings.EqualFold(keyMsg.String(), "j"):
		if m.providerIndex < len(m.providers)-1 {
			m.providerIndex++
		}
		m.setStatus("Select a provider, then press Enter.", false)
		return m, nil
	case keyMsg.Type == tea.KeyRunes:
		if idx, ok := providerIndexFromKey(keyMsg.String(), len(m.providers)); ok {
			m.providerIndex = idx
			m.setStatus("Select a provider, then press Enter.", false)
			return m, nil
		}
	}

	return m, nil
}

func (m *initModel) updateModelStep(keyMsg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if keyMsg.Type == tea.KeyCtrlB || strings.EqualFold(keyMsg.String(), "ctrl+b") {
		m.step = initStepProvider
		m.modelInput.Blur()
		m.setStatus("Choose a provider, then press Enter.", false)
		return m, nil
	}
	if keyMsg.Type == tea.KeyEnter {
		m.syncModelValue()
		if strings.TrimSpace(m.config.Model) == "" {
			m.setStatus("config model must not be empty", true)
			return m, nil
		}
		m.step = initStepFields
		m.rebuildFieldInputs()
		m.focusActiveField()
		m.setStatus("Enter the provider settings, then press Enter to test.", false)
		return m, textinput.Blink
	}

	var cmd tea.Cmd
	m.modelInput, cmd = m.modelInput.Update(keyMsg)
	return m, cmd
}

func (m *initModel) updateFieldsStep(keyMsg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if keyMsg.Type == tea.KeyCtrlB || strings.EqualFold(keyMsg.String(), "ctrl+b") {
		m.step = initStepModel
		m.modelInput.Focus()
		for index := range m.fieldInputs {
			m.fieldInputs[index].Blur()
		}
		m.setStatus("Update the model name or go back again to change the provider.", false)
		return m, textinput.Blink
	}

	switch {
	case keyMsg.Type == tea.KeyUp:
		if m.activeField > 0 {
			m.activeField--
			m.focusActiveField()
		}
		return m, textinput.Blink
	case keyMsg.Type == tea.KeyDown || keyMsg.Type == tea.KeyTab:
		if m.activeField < len(m.fieldInputs)-1 {
			m.activeField++
			m.focusActiveField()
			return m, textinput.Blink
		}
		return m.beginValidation()
	case keyMsg.Type == tea.KeyEnter:
		if m.activeField < len(m.fieldInputs)-1 {
			m.activeField++
			m.focusActiveField()
			return m, textinput.Blink
		}
		return m.beginValidation()
	}

	if len(m.fieldInputs) == 0 {
		return m.beginValidation()
	}

	var cmd tea.Cmd
	m.fieldInputs[m.activeField], cmd = m.fieldInputs[m.activeField].Update(keyMsg)
	return m, cmd
}

func (m *initModel) beginValidation() (tea.Model, tea.Cmd) {
	m.syncFieldValues()
	m.config = normalizeInitConfig(m.config)
	if err := m.config.Validate(); err != nil {
		m.setStatus(err.Error(), true)
		return m, nil
	}

	m.validating = true
	m.modelInput.Blur()
	for index := range m.fieldInputs {
		m.fieldInputs[index].Blur()
	}
	m.setStatus("Testing provider settings and saving config...", false)
	return m, validateAndSaveConfig(m.ctx, m.config)
}

func (m *initModel) applySelectedProvider() {
	choice := m.providers[m.providerIndex]
	providerChanged := !strings.EqualFold(m.config.Provider, choice.ID)
	defaults := config.DefaultForProvider(choice.ID)

	m.config.Provider = choice.ID
	if providerChanged {
		m.config.Model = defaults.Model
		m.config.BaseURL = defaults.BaseURL
		m.config.APIKey = ""
	}
	m.config = normalizeInitConfig(m.config)
	m.modelInput.SetValue(m.config.Model)
	m.rebuildFieldInputs()
	if m.activeField >= len(m.fieldInputs) {
		m.activeField = maxInt(0, len(m.fieldInputs)-1)
	}
	for index := range m.fieldInputs {
		m.fieldInputs[index].Blur()
	}
	if len(m.fieldInputs) > 0 {
		m.focusActiveField()
	}
}

func (m *initModel) syncModelValue() {
	m.config.Model = strings.TrimSpace(m.modelInput.Value())
	if m.config.Model == "" {
		m.config.Model = config.DefaultModel(m.config.Provider)
		m.modelInput.SetValue(m.config.Model)
	}
}

func (m *initModel) syncFieldValues() {
	m.syncModelValue()
	for index, spec := range m.fieldSpecs {
		value := strings.TrimSpace(m.fieldInputs[index].Value())
		switch spec.Key {
		case "api_key":
			m.config.APIKey = value
		case "base_url":
			m.config.BaseURL = value
		}
	}
	if strings.TrimSpace(m.config.BaseURL) == "" {
		m.config.BaseURL = config.DefaultBaseURL(m.config.Provider)
	}
	if !providerRequiresAPIKey(m.config.Provider) {
		m.config.APIKey = ""
	}
}

func (m *initModel) rebuildFieldInputs() {
	m.fieldSpecs = initFieldSpecs(m.config.Provider)
	m.fieldInputs = make([]textinput.Model, 0, len(m.fieldSpecs))
	for _, spec := range m.fieldSpecs {
		input := newInitTextInput(m.styles, spec.Placeholder)
		switch spec.Key {
		case "api_key":
			input.SetValue(strings.TrimSpace(m.config.APIKey))
			if spec.Sensitive {
				input.EchoMode = textinput.EchoPassword
				input.EchoCharacter = '*'
			}
		case "base_url":
			input.SetValue(strings.TrimSpace(m.config.BaseURL))
		}
		m.fieldInputs = append(m.fieldInputs, input)
	}
	if len(m.fieldInputs) == 0 {
		m.activeField = 0
		return
	}
	if m.activeField < 0 {
		m.activeField = 0
	}
	if m.activeField >= len(m.fieldInputs) {
		m.activeField = len(m.fieldInputs) - 1
	}
	m.focusActiveField()
	m.resizeInputs()
}

func (m *initModel) focusActiveField() {
	for index := range m.fieldInputs {
		if index == m.activeField {
			m.fieldInputs[index].Focus()
			continue
		}
		m.fieldInputs[index].Blur()
	}
	if m.step == initStepModel {
		m.modelInput.Focus()
	} else {
		m.modelInput.Blur()
	}
}

func (m *initModel) resizeInputs() {
	panelWidth := clampInt(m.width-14, 72, 110)
	bodyWidth := innerWidth(panelWidth, m.styles.panel, 28)
	inputWidth := innerWidth(bodyWidth, m.styles.fieldBoxActive, 16)
	if inputWidth < 16 {
		inputWidth = 16
	}
	m.modelInput.Width = inputWidth
	for index := range m.fieldInputs {
		m.fieldInputs[index].Width = inputWidth
	}
}

func (m *initModel) renderHeader(width int) string {
	title := styleOn(m.styles.headerTitle, m.styles.panelFill).Render("ennacmd setup")
	right := m.styles.badgeInfo.Render(m.stepBadge())
	gap := width - lipgloss.Width(title) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	line1 := title + m.styles.panelFill.Render(strings.Repeat(" ", gap)) + right

	choice := m.providers[m.providerIndex]
	badges := []string{m.styles.badgeInfo.Render(choice.Title)}
	if model := strings.TrimSpace(m.currentModel()); model != "" {
		badges = append(badges, m.styles.badgeModel.Render(truncateRight(model, 24)))
	}
	if m.validating {
		badges = append(badges, m.styles.badgeBusy.Render("testing"))
	}

	return lipgloss.JoinVertical(lipgloss.Left, padRenderedLineWithStyle(line1, width, m.styles.panelFill), blankLineWithStyle(width, m.styles.panelFill), wrapTokensOn(width, badges, m.styles.panelFill))
}

func (m *initModel) renderBody(width int) string {
	switch m.step {
	case initStepProvider:
		return m.renderProviderStep(width)
	case initStepModel:
		return m.renderModelStep(width)
	case initStepFields:
		return m.renderFieldsStep(width)
	default:
		return ""
	}
}

func (m *initModel) renderProviderStep(width int) string {
	sections := []string{renderPaddedTextLineOn("PROVIDER", width, m.styles.label, m.styles.panelFill)}
	for index, choice := range m.providers {
		boxStyle := m.styles.providerBox
		fillStyle := m.styles.providerFill
		markerStyle := m.styles.selectionInactive
		marker := " "
		if index == m.providerIndex {
			boxStyle = m.styles.providerBoxActive
			fillStyle = m.styles.providerFillActive
			markerStyle = m.styles.selectionMarker
			marker = ">"
		}
		boxBodyWidth := innerWidth(width, boxStyle, 20)

		header := styleOn(markerStyle, fillStyle).Render(marker) + fillStyle.Render(" ") + styleOn(lipgloss.NewStyle().Bold(true), fillStyle).Render(choice.Title)
		content := []string{padRenderedLineWithStyle(header, boxBodyWidth, fillStyle), blankLineWithStyle(boxBodyWidth, fillStyle)}
		content = append(content, renderStyledLinesOn(choice.Description, boxBodyWidth, m.styles.description, fillStyle)...)
		content = append(content, renderStyledLinesOn("default model  "+choice.DefaultModel, boxBodyWidth, m.styles.meta, fillStyle)...)
		content = append(content, renderStyledLinesOn("base URL       "+choice.DefaultURL, boxBodyWidth, m.styles.meta, fillStyle)...)
		sections = append(sections, boxStyle.Width(width).Render(strings.Join(content, "\n")))
	}
	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

func (m *initModel) renderModelStep(width int) string {
	boxBodyWidth := innerWidth(width, m.styles.fieldBoxActive, 20)
	content := []string{
		renderPaddedTextLineOn("MODEL", boxBodyWidth, m.styles.label, m.styles.fieldFillActive),
		padRenderedLineWithStyle(m.modelInput.View(), boxBodyWidth, m.styles.inputFill),
	}
	content = append(content, renderStyledLinesOn("Use the exact model identifier exposed by the selected provider.", boxBodyWidth, m.styles.meta, m.styles.fieldFillActive)...)
	box := m.styles.fieldBoxActive.Width(width).Render(strings.Join(content, "\n"))

	return lipgloss.JoinVertical(lipgloss.Left, box)
}

func (m *initModel) renderFieldsStep(width int) string {
	sections := []string{
		renderPaddedTextLineOn("SETTINGS", width, m.styles.label, m.styles.panelFill),
	}

	for index, spec := range m.fieldSpecs {
		boxStyle := m.styles.fieldBox
		fillStyle := m.styles.fieldFill
		if index == m.activeField {
			boxStyle = m.styles.fieldBoxActive
			fillStyle = m.styles.fieldFillActive
		}
		boxBodyWidth := innerWidth(width, boxStyle, 20)
		content := []string{
			renderPaddedTextLineOn(strings.ToUpper(spec.Label), boxBodyWidth, m.styles.label, fillStyle),
			padRenderedLineWithStyle(m.fieldInputs[index].View(), boxBodyWidth, m.styles.inputFill),
		}
		content = append(content, renderStyledLinesOn(spec.Help, boxBodyWidth, m.styles.meta, fillStyle)...)
		sections = append(sections, blankLineWithStyle(width, m.styles.panelFill), boxStyle.Width(width).Render(strings.Join(content, "\n")))
	}

	if m.validating {
		bodyWidth := innerWidth(width, m.styles.fieldBoxActive, 20)
		body := strings.Join(renderStyledLinesOn("Testing provider access, model availability, and saving the config...", bodyWidth, m.styles.description, m.styles.fieldFillActive), "\n")
		sections = append(sections, blankLineWithStyle(width, m.styles.panelFill), m.styles.fieldBoxActive.Width(width).Render(body))
	}

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}
func (m *initModel) renderFooter(width int) string {
	hintLine := wrapTokensOn(width, m.renderHints(), m.styles.panelFill)
	footerLines := []string{hintLine}
	if strings.TrimSpace(m.config.Paths.ConfigFile) != "" {
		footerLines = append(footerLines, blankLineWithStyle(width, m.styles.panelFill), renderPaddedTextLineOn("config  "+compactPath(m.config.Paths.ConfigFile, width), width, m.styles.meta, m.styles.panelFill))
	}
	if status := strings.TrimSpace(m.status); status != "" {
		footerLines = append(footerLines, blankLineWithStyle(width, m.styles.panelFill), renderPaddedTextLineOn(status, width, m.statusTone, m.styles.panelFill))
	}
	return lipgloss.JoinVertical(lipgloss.Left, footerLines...)
}

func (m *initModel) renderHints() []string {
	if m.validating {
		return []string{m.renderHintToken("wait", "testing setup")}
	}
	if m.step == initStepProvider {
		return []string{
			m.renderHintToken("up/down", "choose"),
			m.renderHintToken("1-3", "jump"),
			m.renderHintToken("enter", "continue"),
			m.renderHintToken("esc", "close"),
		}
	}
	if m.step == initStepModel {
		return []string{
			m.renderHintToken("enter", "continue"),
			m.renderHintToken("ctrl+b", "back"),
			m.renderHintToken("esc", "close"),
		}
	}
	return []string{
		m.renderHintToken("up/down", "focus"),
		m.renderHintToken("enter", "next or test"),
		m.renderHintToken("ctrl+b", "back"),
		m.renderHintToken("esc", "close"),
	}
}

func (m *initModel) renderHintToken(key string, text string) string {
	return m.styles.hintKey.Render(strings.ToUpper(key)) + m.styles.panelFill.Render(" ") + styleOn(m.styles.hintText, m.styles.panelFill).Render(text)
}

func (m *initModel) stepSummary() string {
	switch m.step {
	case initStepProvider:
		return "Step 1 of 3: choose the provider you want to configure."
	case initStepModel:
		return "Step 2 of 3: enter the model name you want ennacmd to use."
	case initStepFields:
		return "Step 3 of 3: enter the provider settings, test them, and save the config."
	default:
		return ""
	}
}

func (m *initModel) stepBadge() string {
	switch m.step {
	case initStepProvider:
		return "step 1/3"
	case initStepModel:
		return "step 2/3"
	case initStepFields:
		return "step 3/3"
	default:
		return "setup"
	}
}

func (m *initModel) currentModel() string {
	if m.step == initStepModel {
		return strings.TrimSpace(m.modelInput.Value())
	}
	return strings.TrimSpace(m.config.Model)
}

func (m *initModel) setStatus(message string, isError bool) {
	m.status = strings.TrimSpace(message)
	m.statusIsError = isError
	if isError {
		m.statusTone = m.styles.statusErr
		return
	}
	m.statusTone = m.styles.statusOK
}

func newInitStyles() initStyles {
	return initStyles{
		panel: lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#0f172a", Dark: "#e2e8f0"}).
			Padding(1, 2).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.AdaptiveColor{Light: "#b8c5db", Dark: "#20304a"}),
		panelFill:          lipgloss.NewStyle(),
		providerFill:       lipgloss.NewStyle(),
		providerFillActive: lipgloss.NewStyle(),
		fieldFill:          lipgloss.NewStyle(),
		fieldFillActive:    lipgloss.NewStyle(),
		inputFill:          lipgloss.NewStyle(),
		headerTitle:        lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "#0f172a", Dark: "#f8fafc"}),
		meta:               lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#66758b", Dark: "#8ea0b8"}),
		label: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.AdaptiveColor{Light: "#56a6b8", Dark: "#74d5e3"}),
		description: lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#334155", Dark: "#d7e3f4"}),
		providerBox: lipgloss.NewStyle().
			Padding(1, 2).
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.AdaptiveColor{Light: "#cbd5e1", Dark: "#1b2b42"}),
		providerBoxActive: lipgloss.NewStyle().
			Padding(1, 2).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.AdaptiveColor{Light: "#56a6b8", Dark: "#2b5c88"}),
		fieldBox: lipgloss.NewStyle().
			Padding(1, 2).
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.AdaptiveColor{Light: "#cbd5e1", Dark: "#1b2b42"}),
		fieldBoxActive: lipgloss.NewStyle().
			Padding(1, 2).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.AdaptiveColor{Light: "#b9c7dc", Dark: "#2b5c88"}),
		input:      lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#0f172a", Dark: "#f8fafc"}),
		inputShell: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "#56a6b8", Dark: "#7dd3fc"}),
		placeholder: lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#8ea0b8", Dark: "#64748b"}),
		hint:     lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#64748b", Dark: "#91a4bc"}),
		hintKey:  badgeStyle("#f8fafc", "#334155", "#dbeafe", "#172235"),
		hintText: lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#64748b", Dark: "#8ea0b8"}),
		statusOK: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "#0f766e", Dark: "#7ee7c7"}),
		statusErr: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.AdaptiveColor{Light: "#b91c1c", Dark: "#fda4af"}),
		badgeInfo:         badgeStyle("#eef2ff", "#3b4d7a", "#c7d2fe", "#27314f"),
		badgeModel:        badgeStyle("#e7fbf7", "#0f4f49", "#99f6e4", "#083b39"),
		badgeBusy:         badgeStyle("#ffe7c2", "#8a4b00", "#fdba74", "#6a3d05"),
		selectionMarker:   lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "#0f766e", Dark: "#7ee7c7"}),
		selectionInactive: lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#8ea0b8", Dark: "#64748b"}),
	}
}

func newInitTextInput(styles initStyles, placeholder string) textinput.Model {
	input := textinput.New()
	input.Prompt = "› "
	input.Placeholder = placeholder
	input.CharLimit = 0
	input.TextStyle = styleOn(styles.input, styles.inputFill)
	input.PromptStyle = styleOn(styles.inputShell, styles.inputFill)
	input.PlaceholderStyle = styleOn(styles.placeholder, styles.inputFill)
	input.Cursor.Style = styleOn(lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#38bdf8", Dark: "#7dd3fc"}), styles.inputFill)
	return input
}

func initProviderChoices() []initProviderChoice {
	return []initProviderChoice{
		{
			ID:           "openai",
			Title:        "OpenAI",
			Description:  "Hosted OpenAI models and compatible deployments.",
			DefaultURL:   config.DefaultBaseURL("openai"),
			DefaultModel: config.DefaultModel("openai"),
		},
		{
			ID:           "openrouter",
			Title:        "OpenRouter",
			Description:  "Hosted routing across many upstream models.",
			DefaultURL:   config.DefaultBaseURL("openrouter"),
			DefaultModel: config.DefaultModel("openrouter"),
		},
		{
			ID:           "ollama",
			Title:        "Ollama",
			Description:  "Local Ollama server on your machine or network.",
			DefaultURL:   config.DefaultBaseURL("ollama"),
			DefaultModel: config.DefaultModel("ollama"),
		},
	}
}

func providerChoiceIndex(choices []initProviderChoice, providerName string) int {
	for index, choice := range choices {
		if strings.EqualFold(choice.ID, providerName) {
			return index
		}
	}
	return -1
}

func providerIndexFromKey(value string, count int) (int, bool) {
	if len(value) != 1 {
		return 0, false
	}
	index, err := strconv.Atoi(value)
	if err != nil || index < 1 || index > count {
		return 0, false
	}
	return index - 1, true
}

func initFieldSpecs(providerName string) []initFieldSpec {
	if providerRequiresAPIKey(providerName) {
		return []initFieldSpec{
			{
				Key:         "api_key",
				Label:       "API key",
				Placeholder: "paste the API key for this provider",
				Help:        "Required for hosted providers. The test call will fail fast if the key is rejected.",
				Required:    true,
				Sensitive:   true,
			},
			{
				Key:         "base_url",
				Label:       "Base URL",
				Placeholder: "provider endpoint",
				Help:        "The default endpoint is prefilled. Change it only if you use a different deployment URL.",
				Required:    true,
			},
		}
	}

	return []initFieldSpec{
		{
			Key:         "base_url",
			Label:       "Base URL",
			Placeholder: "http://localhost:11434",
			Help:        "The default points to Ollama on localhost. Change it only if the server is elsewhere.",
			Required:    true,
		},
	}
}

func providerRequiresAPIKey(providerName string) bool {
	return !strings.EqualFold(strings.TrimSpace(providerName), "ollama")
}

func normalizeInitConfig(cfg config.Config) config.Config {
	defaults := config.DefaultForProvider(cfg.Provider)
	normalized := cfg
	if strings.TrimSpace(normalized.Provider) == "" {
		normalized.Provider = defaults.Provider
	} else {
		normalized.Provider = strings.ToLower(strings.TrimSpace(normalized.Provider))
	}
	normalized.Model = strings.TrimSpace(normalized.Model)
	if normalized.Model == "" {
		normalized.Model = config.DefaultModel(normalized.Provider)
	}
	normalized.BaseURL = strings.TrimSpace(normalized.BaseURL)
	if normalized.BaseURL == "" {
		normalized.BaseURL = config.DefaultBaseURL(normalized.Provider)
	}
	normalized.APIKey = strings.TrimSpace(normalized.APIKey)
	if normalized.Temperature == 0 {
		normalized.Temperature = defaults.Temperature
	}
	normalized.Shell = strings.TrimSpace(normalized.Shell)
	if normalized.Shell == "" {
		normalized.Shell = defaults.Shell
	}
	return normalized
}

func validateAndSaveConfig(ctx context.Context, cfg config.Config) tea.Cmd {
	return func() tea.Msg {
		testCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
		defer cancel()

		validatedProvider, err := provider.New(cfg)
		if err != nil {
			return initValidationMsg{err: err}
		}

		_, err = validatedProvider.Complete(testCtx, ai.Prompt{
			Model:       cfg.Model,
			Temperature: 0,
			Stream:      false,
			Messages: []ai.Message{{
				Role:    ai.RoleUser,
				Content: "Reply with ok.",
			}},
		})
		if err != nil {
			return initValidationMsg{err: err}
		}

		saved, err := config.Save(cfg)
		if err != nil {
			return initValidationMsg{err: err}
		}

		return initValidationMsg{config: saved}
	}
}
