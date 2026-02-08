// ABOUTME: Unit tests for the setup TUI wizard bubbletea model.
// ABOUTME: Uses synthetic tea.Msg values to test state machine transitions.
package tui

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func TestNewSetupModel_DefaultValues(t *testing.T) {
	m := NewSetupModel("", "", "")
	if m.step != StepAPIURL {
		t.Errorf("expected initial step StepAPIURL, got %d", m.step)
	}
	if m.inputs[0].Value() != "" {
		t.Error("expected empty API URL input for new config")
	}
}

func TestNewSetupModel_ExistingConfig(t *testing.T) {
	m := NewSetupModel("https://example.com/api", "my-team", "secret-key")
	if m.inputs[0].Value() != "https://example.com/api" {
		t.Errorf("expected pre-filled API URL, got %q", m.inputs[0].Value())
	}
	if m.inputs[1].Value() != "my-team" {
		t.Errorf("expected pre-filled team ID, got %q", m.inputs[1].Value())
	}
	if m.inputs[2].Value() != "secret-key" {
		t.Errorf("expected pre-filled API key, got %q", m.inputs[2].Value())
	}
}

func TestSetupModel_StepTransitions(t *testing.T) {
	m := NewSetupModel("", "", "")

	// Set a value and press Enter to advance from StepAPIURL to StepTeamID
	m.inputs[0].SetValue("https://botboard.biz/api/v1")
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(SetupModel)
	if m.step != StepTeamID {
		t.Errorf("expected StepTeamID after Enter on API URL, got %d", m.step)
	}
	// cmd is textinput.Blink for the newly focused input
	_ = cmd

	// Set team ID and press Enter to advance to StepAPIKey
	m.inputs[1].SetValue("my-team")
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(SetupModel)
	if m.step != StepAPIKey {
		t.Errorf("expected StepAPIKey after Enter on team ID, got %d", m.step)
	}

	// Set API key and press Enter to start validation
	m.inputs[2].SetValue("my-key")
	updated, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(SetupModel)
	if m.step != StepValidating {
		t.Errorf("expected StepValidating after Enter on API key, got %d", m.step)
	}
	if cmd == nil {
		t.Error("expected non-nil cmd (validation + spinner tick) when entering validation")
	}
}

func TestSetupModel_DefaultAPIURL(t *testing.T) {
	m := NewSetupModel("", "", "")

	// Press Enter on empty API URL field — should use default
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(SetupModel)
	if m.inputs[0].Value() != DefaultAPIURL {
		t.Errorf("expected default API URL %q, got %q", DefaultAPIURL, m.inputs[0].Value())
	}
	if m.step != StepTeamID {
		t.Errorf("expected StepTeamID after default URL applied, got %d", m.step)
	}
}

func TestSetupModel_ValidationSuccess(t *testing.T) {
	m := NewSetupModel("", "", "")
	m.validateFn = func(_ context.Context, apiURL, apiKey, teamID string) error {
		return nil
	}
	m.step = StepValidating

	updated, _ := m.Update(validationResultMsg{err: nil})
	m = updated.(SetupModel)
	if m.step != StepDone {
		t.Errorf("expected StepDone after successful validation, got %d", m.step)
	}
}

func TestSetupModel_ValidationFailure(t *testing.T) {
	m := NewSetupModel("", "", "")
	m.step = StepValidating

	updated, _ := m.Update(validationResultMsg{err: fmt.Errorf("connection refused")})
	m = updated.(SetupModel)
	if m.step != StepFailed {
		t.Errorf("expected StepFailed after validation error, got %d", m.step)
	}
	if m.validationErr == nil {
		t.Error("expected validationErr to be set")
	}
}

func TestSetupModel_FailedRetry(t *testing.T) {
	m := NewSetupModel("", "", "")
	m.step = StepFailed
	m.validationErr = fmt.Errorf("some error")

	// Press 'r' to retry
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	m = updated.(SetupModel)
	if m.step != StepValidating {
		t.Errorf("expected StepValidating after retry, got %d", m.step)
	}
	if cmd == nil {
		t.Error("expected non-nil cmd on retry")
	}
}

func TestSetupModel_FailedSaveAnyway(t *testing.T) {
	m := NewSetupModel("", "", "")
	m.step = StepFailed

	// Press 's' to save anyway
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	m = updated.(SetupModel)
	if m.step != StepDone {
		t.Errorf("expected StepDone after save anyway, got %d", m.step)
	}
}

func TestSetupModel_FailedQuit(t *testing.T) {
	m := NewSetupModel("", "", "")
	m.step = StepFailed

	// Press 'q' to quit
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	m2 := updated.(SetupModel)
	if cmd == nil {
		t.Error("expected quit cmd")
	}
	if !m2.quitting {
		t.Error("expected quitting to be true after 'q'")
	}
	if m2.ShouldSave() {
		t.Error("expected ShouldSave false after quit")
	}
}

func TestSetupModel_QuitOnCtrlC(t *testing.T) {
	m := NewSetupModel("", "", "")
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	m = updated.(SetupModel)
	if cmd == nil {
		t.Error("expected quit cmd on ctrl+c")
	}
	if !m.quitting {
		t.Error("expected quitting to be true")
	}
	if m.ShouldSave() {
		t.Error("expected ShouldSave false after ctrl+c")
	}
}

func TestSetupModel_QuitOnEsc(t *testing.T) {
	m := NewSetupModel("", "", "")
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	m = updated.(SetupModel)
	if cmd == nil {
		t.Error("expected quit cmd on escape")
	}
	if !m.quitting {
		t.Error("expected quitting to be true")
	}
	if m.ShouldSave() {
		t.Error("expected ShouldSave false after escape")
	}
}

func TestSetupModel_Result(t *testing.T) {
	m := NewSetupModel("", "", "")
	m.inputs[0].SetValue("https://botboard.biz/api/v1")
	m.inputs[1].SetValue("team-123")
	m.inputs[2].SetValue("key-456")
	m.step = StepDone

	apiURL, teamID, apiKey := m.Result()
	if apiURL != "https://botboard.biz/api/v1" {
		t.Errorf("expected API URL from result, got %q", apiURL)
	}
	if teamID != "team-123" {
		t.Errorf("expected team ID from result, got %q", teamID)
	}
	if apiKey != "key-456" {
		t.Errorf("expected API key from result, got %q", apiKey)
	}
}

func TestSetupModel_ShouldSave(t *testing.T) {
	t.Run("done means save", func(t *testing.T) {
		m := NewSetupModel("", "", "")
		m.step = StepDone
		if !m.ShouldSave() {
			t.Error("expected ShouldSave true when done")
		}
	})

	t.Run("quit means no save", func(t *testing.T) {
		m := NewSetupModel("", "", "")
		m.step = StepFailed
		m.quitting = true
		if m.ShouldSave() {
			t.Error("expected ShouldSave false when quitting from failed")
		}
	})

	t.Run("save anyway means save", func(t *testing.T) {
		m := NewSetupModel("", "", "")
		m.step = StepFailed
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
		m = updated.(SetupModel)
		if !m.ShouldSave() {
			t.Error("expected ShouldSave true after save anyway")
		}
	})
}

func TestSetupModel_ViewContainsBrainwaveNitro(t *testing.T) {
	m := NewSetupModel("", "", "")
	view := m.View()
	if !strings.Contains(view, "BRAINWAVE NITRO") {
		t.Error("expected view to contain BRAINWAVE NITRO branding")
	}
}

func TestSetupModel_ViewShowsCurrentStep(t *testing.T) {
	m := NewSetupModel("", "", "")

	m.step = StepAPIURL
	if !strings.Contains(m.View(), "API URL") {
		t.Error("expected StepAPIURL view to mention API URL")
	}

	m.step = StepTeamID
	if !strings.Contains(m.View(), "Team ID") {
		t.Error("expected StepTeamID view to mention Team ID")
	}

	m.step = StepAPIKey
	if !strings.Contains(m.View(), "API Key") {
		t.Error("expected StepAPIKey view to mention API Key")
	}
}

func TestSetupModel_ViewValidating(t *testing.T) {
	m := NewSetupModel("", "", "")
	m.step = StepValidating
	view := m.View()
	if !strings.Contains(view, "Validating connection") {
		t.Error("expected StepValidating view to mention Validating connection")
	}
}

func TestSetupModel_ViewDone(t *testing.T) {
	m := NewSetupModel("", "", "")
	m.step = StepDone
	view := m.View()
	if !strings.Contains(view, "Connected") {
		t.Error("expected StepDone view to mention Connected")
	}
}

func TestSetupModel_ViewFailed(t *testing.T) {
	m := NewSetupModel("", "", "")
	m.step = StepFailed
	m.validationErr = fmt.Errorf("timeout")
	view := m.View()
	if !strings.Contains(view, "Validation failed") {
		t.Error("expected StepFailed view to mention Validation failed")
	}
	if !strings.Contains(view, "timeout") {
		t.Error("expected StepFailed view to show error message")
	}
	if !strings.Contains(view, "[r]etry") {
		t.Error("expected StepFailed view to show retry option")
	}
	if !strings.Contains(view, "[s]ave anyway") {
		t.Error("expected StepFailed view to show save anyway option")
	}
	if !strings.Contains(view, "[q]uit") {
		t.Error("expected StepFailed view to show quit option")
	}
}

func TestSetupModel_EmptyTeamIDBlocked(t *testing.T) {
	m := NewSetupModel("", "", "")
	m.step = StepTeamID
	// Press Enter on empty team ID — should not advance
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(SetupModel)
	if m.step != StepTeamID {
		t.Errorf("expected to stay on StepTeamID with empty input, got %d", m.step)
	}
}

func TestSetupModel_EmptyAPIKeyBlocked(t *testing.T) {
	m := NewSetupModel("", "", "")
	m.step = StepAPIKey
	// Press Enter on empty API key — should not advance
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(SetupModel)
	if m.step != StepAPIKey {
		t.Errorf("expected to stay on StepAPIKey with empty input, got %d", m.step)
	}
}

func TestSetupModel_CtrlCDuringValidation(t *testing.T) {
	cancelled := false
	m := NewSetupModel("", "", "")
	m.validateFn = func(ctx context.Context, _, _, _ string) error {
		<-ctx.Done()
		cancelled = true
		return ctx.Err()
	}
	m.inputs[0].SetValue("https://botboard.biz/api/v1")
	m.inputs[1].SetValue("team")
	m.inputs[2].SetValue("key")
	m.step = StepAPIKey

	// Press Enter to start validation — sets cancelCtx.cancel via startValidation
	updated, batchCmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(SetupModel)
	if m.step != StepValidating {
		t.Fatalf("expected StepValidating, got %d", m.step)
	}

	// Execute the batch cmd to get individual cmds, then run the validation cmd
	// in a goroutine so we can cancel it with Ctrl+C.
	batchMsg := batchCmd().(tea.BatchMsg)
	done := make(chan tea.Msg)
	go func() {
		// batchMsg[0] is the validation cmd, batchMsg[1] is the spinner tick
		done <- batchMsg[0]()
	}()

	// Press Ctrl+C — should cancel the validation context
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	m = updated.(SetupModel)
	if !m.quitting {
		t.Error("expected quitting to be true after Ctrl+C during validation")
	}

	// Wait for the validation goroutine to finish (it should unblock from ctx.Done())
	<-done
	if !cancelled {
		t.Error("expected validation context to be cancelled")
	}
}

func TestSetupModel_ValidationPassesCorrectArgs(t *testing.T) {
	var gotURL, gotKey, gotTeam string
	m := NewSetupModel("", "", "")
	m.validateFn = func(_ context.Context, apiURL, apiKey, teamID string) error {
		gotURL = apiURL
		gotKey = apiKey
		gotTeam = teamID
		return nil
	}
	m.inputs[0].SetValue("https://example.com/api")
	m.inputs[1].SetValue("team-xyz")
	m.inputs[2].SetValue("secret-abc")
	m.step = StepAPIKey

	// Press Enter to start validation
	_, batchCmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	// Execute batch to get individual cmds, then run the validation cmd
	batchMsg := batchCmd().(tea.BatchMsg)
	batchMsg[0]() // validation cmd

	if gotURL != "https://example.com/api" {
		t.Errorf("expected apiURL %q, got %q", "https://example.com/api", gotURL)
	}
	if gotKey != "secret-abc" {
		t.Errorf("expected apiKey %q, got %q", "secret-abc", gotKey)
	}
	if gotTeam != "team-xyz" {
		t.Errorf("expected teamID %q, got %q", "team-xyz", gotTeam)
	}
}

func TestSetupModel_TrailingSlashNormalized(t *testing.T) {
	m := NewSetupModel("", "", "")
	m.inputs[0].SetValue("https://example.com/api/")

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(SetupModel)
	if m.inputs[0].Value() != "https://example.com/api" {
		t.Errorf("expected trailing slash stripped, got %q", m.inputs[0].Value())
	}
}

func TestSetupModel_V1SuffixStripped(t *testing.T) {
	m := NewSetupModel("", "", "")
	m.inputs[0].SetValue("https://api.example.com/v1")

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(SetupModel)
	if m.inputs[0].Value() != "https://api.example.com" {
		t.Errorf("expected /v1 suffix stripped, got %q", m.inputs[0].Value())
	}
}

func TestSetupModel_V1WithTrailingSlashStripped(t *testing.T) {
	m := NewSetupModel("", "", "")
	m.inputs[0].SetValue("https://api.example.com/v1/")

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(SetupModel)
	if m.inputs[0].Value() != "https://api.example.com" {
		t.Errorf("expected /v1/ stripped, got %q", m.inputs[0].Value())
	}
}

func TestSetupModel_FullPrefilledFlow(t *testing.T) {
	m := NewSetupModel("https://example.com/api", "my-team", "my-key")
	m.validateFn = func(_ context.Context, _, _, _ string) error { return nil }

	t.Logf("Initial: step=%d, quitting=%v, ShouldSave=%v", m.step, m.quitting, m.ShouldSave())

	// Enter on pre-filled API URL
	u, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = u.(SetupModel)
	t.Logf("After URL Enter: step=%d, quitting=%v", m.step, m.quitting)
	if m.step != StepTeamID {
		t.Fatalf("expected StepTeamID, got %d", m.step)
	}

	// Enter on pre-filled Team ID
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = u.(SetupModel)
	t.Logf("After TeamID Enter: step=%d, quitting=%v", m.step, m.quitting)
	if m.step != StepAPIKey {
		t.Fatalf("expected StepAPIKey, got %d", m.step)
	}

	// Enter on pre-filled API Key -> starts validation
	u, batchCmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = u.(SetupModel)
	t.Logf("After APIKey Enter: step=%d, quitting=%v", m.step, m.quitting)
	if m.step != StepValidating {
		t.Fatalf("expected StepValidating, got %d", m.step)
	}

	// Execute the validation cmd from the batch
	batchMsg := batchCmd().(tea.BatchMsg)
	resultMsg := batchMsg[0]()
	t.Logf("Validation result: %+v", resultMsg)

	// Feed result back
	u, _ = m.Update(resultMsg)
	m = u.(SetupModel)
	t.Logf("After validation: step=%d, quitting=%v, ShouldSave=%v", m.step, m.quitting, m.ShouldSave())

	if m.step != StepDone {
		t.Errorf("expected StepDone, got %d", m.step)
	}
	if !m.ShouldSave() {
		t.Errorf("expected ShouldSave=true, got false (quitting=%v)", m.quitting)
	}
}

func TestSetupModel_FullFlowWithTeaProgram(t *testing.T) {
	m := NewSetupModel("https://example.com/api", "my-team", "my-key")
	m.validateFn = func(_ context.Context, _, _, _ string) error {
		time.Sleep(50 * time.Millisecond)
		return nil
	}

	p := tea.NewProgram(m, tea.WithInput(nil), tea.WithoutRenderer())

	go func() {
		p.Send(tea.KeyMsg{Type: tea.KeyEnter}) // URL
		p.Send(tea.KeyMsg{Type: tea.KeyEnter}) // Team ID
		p.Send(tea.KeyMsg{Type: tea.KeyEnter}) // API Key -> validates -> done -> quit
	}()

	result, err := p.Run()
	if err != nil {
		t.Fatalf("tea.Program error: %v", err)
	}

	final := result.(SetupModel)
	t.Logf("Final: step=%d, quitting=%v, ShouldSave=%v", final.step, final.quitting, final.ShouldSave())
	if !final.ShouldSave() {
		t.Errorf("expected ShouldSave=true after successful validation, got false (step=%d, quitting=%v)", final.step, final.quitting)
	}
}

func TestSetupModel_ViewFailedNilError(t *testing.T) {
	m := NewSetupModel("", "", "")
	m.step = StepFailed
	// validationErr is nil — View should not panic or show %!s(<nil>)
	view := m.View()
	if strings.Contains(view, "<nil>") {
		t.Error("expected nil error to be rendered gracefully, not as <nil>")
	}
	if !strings.Contains(view, "unknown error") {
		t.Error("expected nil error to show 'unknown error' fallback")
	}
}
