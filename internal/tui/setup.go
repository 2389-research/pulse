// ABOUTME: Interactive TUI wizard for connecting a botboard.biz account.
// ABOUTME: 3-step bubbletea model collecting API URL, team ID, and API key.
package tui

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// DefaultAPIURL is the default botboard.biz API endpoint.
const DefaultAPIURL = "https://botboard.biz/api/v1"

// Step represents the current wizard step.
type Step int

const (
	StepAPIURL Step = iota
	StepTeamID
	StepAPIKey
	StepValidating
	StepDone
	StepFailed
)

// validationResultMsg carries the result of an async validation attempt.
type validationResultMsg struct {
	err error
}

// ValidateFn is the function signature for connection validation.
type ValidateFn func(ctx context.Context, apiURL, apiKey, teamID string) error

// cancelHolder shares a cancel function across bubbletea model copies.
// This MUST be stored as a pointer field on SetupModel so that value-receiver
// methods (required by tea.Model) can store the cancel func and have it
// visible to all copies of the model.
type cancelHolder struct {
	cancel context.CancelFunc
}

// SetupModel is the bubbletea model for the setup wizard.
type SetupModel struct {
	step          Step
	inputs        [3]textinput.Model
	spinner       spinner.Model
	validateFn    ValidateFn
	cancelCtx     *cancelHolder
	validationErr error
	quitting      bool
}

var (
	titleStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("99"))
	brandStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	stepStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	successStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
	errorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	promptStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
)

// NewSetupModel creates a new setup wizard model, pre-filling with existing config values.
func NewSetupModel(apiURL, teamID, apiKey string) SetupModel {
	urlInput := textinput.New()
	urlInput.Placeholder = DefaultAPIURL
	urlInput.Focus()
	urlInput.Width = 50
	if apiURL != "" {
		urlInput.SetValue(apiURL)
	}

	teamInput := textinput.New()
	teamInput.Placeholder = "your-team-id"
	teamInput.Width = 50
	if teamID != "" {
		teamInput.SetValue(teamID)
	}

	keyInput := textinput.New()
	keyInput.Placeholder = "your-api-key"
	keyInput.EchoMode = textinput.EchoPassword
	keyInput.Width = 50
	if apiKey != "" {
		keyInput.SetValue(apiKey)
	}

	s := spinner.New()
	s.Spinner = spinner.Dot

	return SetupModel{
		step:       StepAPIURL,
		inputs:     [3]textinput.Model{urlInput, teamInput, keyInput},
		spinner:    s,
		validateFn: ValidateConnection,
		cancelCtx:  &cancelHolder{},
	}
}

// Init implements tea.Model.
func (m SetupModel) Init() tea.Cmd {
	return textinput.Blink
}

// Update implements tea.Model.
func (m SetupModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEscape:
			m.quitting = true
			if m.cancelCtx.cancel != nil {
				m.cancelCtx.cancel()
			}
			return m, tea.Quit
		}

		switch m.step {
		case StepAPIURL, StepTeamID, StepAPIKey:
			return m.updateInput(msg)
		case StepFailed:
			return m.updateFailed(msg)
		}

	case validationResultMsg:
		m.cancelCtx.cancel = nil
		if msg.err == nil {
			m.step = StepDone
			return m, tea.Quit
		}
		m.validationErr = msg.err
		m.step = StepFailed
		return m, nil

	case spinner.TickMsg:
		if m.step == StepValidating {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
	}

	return m, nil
}

func (m SetupModel) updateInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type == tea.KeyEnter {
		idx := int(m.step)

		// Apply default API URL if empty, and normalize trailing slashes
		if m.step == StepAPIURL {
			val := m.inputs[0].Value()
			if val == "" {
				m.inputs[0].SetValue(DefaultAPIURL)
			} else {
				val = strings.TrimRight(val, "/")
				val = strings.TrimSuffix(val, "/v1")
				m.inputs[0].SetValue(val)
			}
		}

		// Don't advance on empty team ID or API key
		if m.step == StepTeamID && m.inputs[1].Value() == "" {
			return m, nil
		}
		if m.step == StepAPIKey && m.inputs[2].Value() == "" {
			return m, nil
		}

		m.inputs[idx].Blur()

		switch m.step {
		case StepAPIURL:
			m.step = StepTeamID
			m.inputs[1].Focus()
			return m, textinput.Blink
		case StepTeamID:
			m.step = StepAPIKey
			m.inputs[2].Focus()
			return m, textinput.Blink
		case StepAPIKey:
			m.step = StepValidating
			return m, tea.Batch(m.startValidation(), m.spinner.Tick)
		}
	}

	// Forward to the active input
	idx := int(m.step)
	var cmd tea.Cmd
	m.inputs[idx], cmd = m.inputs[idx].Update(msg)
	return m, cmd
}

func (m SetupModel) updateFailed(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type == tea.KeyRunes {
		switch msg.Runes[0] {
		case 'r':
			m.step = StepValidating
			m.validationErr = nil
			return m, tea.Batch(m.startValidation(), m.spinner.Tick)
		case 's':
			m.step = StepDone
			return m, tea.Quit
		case 'q':
			m.quitting = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m SetupModel) startValidation() tea.Cmd {
	ctx, cancel := context.WithCancel(context.Background())
	m.cancelCtx.cancel = cancel
	apiURL := m.inputs[0].Value()
	apiKey := m.inputs[2].Value()
	teamID := m.inputs[1].Value()
	fn := m.validateFn
	return func() tea.Msg {
		return validationResultMsg{err: fn(ctx, apiURL, apiKey, teamID)}
	}
}

// View implements tea.Model.
func (m SetupModel) View() string {
	var b strings.Builder

	b.WriteString("\n")
	b.WriteString(brandStyle.Render("   BRAINWAVE NITRO"))
	b.WriteString(titleStyle.Render(" - Setup"))
	b.WriteString("\n\n")
	b.WriteString("Connect your botboard.biz account.\n\n")

	switch m.step {
	case StepAPIURL:
		b.WriteString(stepStyle.Render("Step 1 of 3: API URL"))
		b.WriteString("\n")
		b.WriteString(promptStyle.Render("(press Enter for default)"))
		b.WriteString("\n")
		b.WriteString(m.inputs[0].View())
		b.WriteString("\n")

	case StepTeamID:
		b.WriteString(fmt.Sprintf("  API URL: %s\n\n", m.inputs[0].Value()))
		b.WriteString(stepStyle.Render("Step 2 of 3: Team ID"))
		b.WriteString("\n")
		b.WriteString(m.inputs[1].View())
		b.WriteString("\n")

	case StepAPIKey:
		b.WriteString(fmt.Sprintf("  API URL: %s\n", m.inputs[0].Value()))
		b.WriteString(fmt.Sprintf("  Team ID: %s\n\n", m.inputs[1].Value()))
		b.WriteString(stepStyle.Render("Step 3 of 3: API Key"))
		b.WriteString("\n")
		b.WriteString(m.inputs[2].View())
		b.WriteString("\n")

	case StepValidating:
		b.WriteString(fmt.Sprintf("  API URL: %s\n", m.inputs[0].Value()))
		b.WriteString(fmt.Sprintf("  Team ID: %s\n", m.inputs[1].Value()))
		b.WriteString(fmt.Sprintf("  API Key: %s\n\n", strings.Repeat("*", len(m.inputs[2].Value()))))
		b.WriteString(m.spinner.View())
		b.WriteString(" Validating connection...")
		b.WriteString("\n")

	case StepDone:
		b.WriteString(successStyle.Render("✓ Connected!"))
		b.WriteString("\n")

	case StepFailed:
		errMsg := "unknown error"
		if m.validationErr != nil {
			errMsg = m.validationErr.Error()
		}
		b.WriteString(errorStyle.Render(fmt.Sprintf("✗ Validation failed: %s", errMsg)))
		b.WriteString("\n\n")
		b.WriteString(promptStyle.Render("[r]etry  [s]ave anyway  [q]uit"))
		b.WriteString("\n")
	}

	return b.String()
}

// Result returns the entered values.
func (m SetupModel) Result() (apiURL, teamID, apiKey string) {
	return m.inputs[0].Value(), m.inputs[1].Value(), m.inputs[2].Value()
}

// ShouldSave returns true if the wizard completed (via validation success or
// "save anyway") and the user did not cancel with Ctrl+C, Escape, or 'q'.
func (m SetupModel) ShouldSave() bool {
	return m.step == StepDone && !m.quitting
}
