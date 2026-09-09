package tuikit

import "charm.land/bubbles/v2/textinput"

// OptionEditorMode classifies the type of editor currently active.
// The editor supports four modes: closed (no editor open), enum-cycle,
// free-text, and fallback-pair (for multi-field entries like name+email).
type OptionEditorMode string

const (
	// OptionEditorModeClosed means no editor is currently open.
	OptionEditorModeClosed OptionEditorMode = "closed"
	// OptionEditorModeEnumCycle means the editor is cycling through a fixed value set.
	OptionEditorModeEnumCycle OptionEditorMode = "enum-cycle"
	// OptionEditorModeText means the editor is accepting free text input.
	OptionEditorModeText OptionEditorMode = "text"
	// OptionEditorModeFallbackPair means the editor is managing two fields (name and email).
	OptionEditorModeFallbackPair OptionEditorMode = "fallback-pair"
)

// OptionEditor is the shared state machine for editing a single option value.
// It is constructed from a key name, a set of declared values (for enum mode),
// and a current/snapshot value. The editor tracks the cursor position, the
// pre-edit snapshot, and an optional text input for free-text mode.
//
// The editor is mode-agnostic: it does not know which policy table supplied
// the value set, so a six-element literal test set needs no backend and no
// policy table. This enables Rule L-4's worst-case measurement in phase 09.6-02
// Task 1 behavior Test 13.
type OptionEditor struct {
	// mode is the current editor state machine state.
	mode OptionEditorMode
	// key is the config key being edited (e.g., "StrictHostKeyChecking").
	key string
	// values is the curated set of allowed values for enum-cycle mode.
	// An out-of-set current value positions the cursor at index 0 rather than
	// being appended to the set (PD33).
	values []string
	// cursor is the current index into values for enum-cycle mode.
	cursor int
	// snapshot is the pre-edit value. Pressing Esc restores this value and
	// clears any staged override.
	snapshot string
	// textInput is the text input model for free-text mode.
	textInput textinput.Model
	// error carries an inline validation error message (Rule I-10).
	error string
}

// NewOptionEditor constructs a new closed editor for the given key with
// the declared value set and current value. If the current value is not in
// the value set, the cursor lands at index 0 (PD33).
func NewOptionEditor(key string, values []string, currentValue string) *OptionEditor {
	cursor := 0
	// Find the cursor position in the value set. If not found, stays at 0.
	for i, v := range values {
		if v == currentValue {
			cursor = i
			break
		}
	}
	return &OptionEditor{
		mode:     OptionEditorModeClosed,
		key:      key,
		values:   values,
		cursor:   cursor,
		snapshot: currentValue,
		textInput: textinput.New(),
	}
}

// IsOpen reports whether the editor is currently open (any mode except closed).
func (e *OptionEditor) IsOpen() bool {
	return e.mode != OptionEditorModeClosed
}

// OpenEnumCycle opens the editor in enum-cycle mode with the cursor positioned
// at the current value (or index 0 if the current value is out of set).
func (e *OptionEditor) OpenEnumCycle(currentValue string) {
	e.mode = OptionEditorModeEnumCycle
	e.snapshot = currentValue
	// Reposition cursor if current value changed.
	e.cursor = 0
	for i, v := range e.values {
		if v == currentValue {
			e.cursor = i
			break
		}
	}
}

// OpenText opens the editor in free-text mode with the given initial value.
func (e *OptionEditor) OpenText(currentValue string) {
	e.mode = OptionEditorModeText
	e.snapshot = currentValue
	e.textInput.SetValue(currentValue)
	e.textInput.Focus()
}

// OpenFallbackPair opens the editor in fallback-pair mode (for multi-field editing).
// Currently a placeholder; the free-text mode will be consumed first.
func (e *OptionEditor) OpenFallbackPair(currentValue string) {
	e.mode = OptionEditorModeFallbackPair
	e.snapshot = currentValue
}

// Close closes the editor and resets to closed mode.
func (e *OptionEditor) Close() {
	e.mode = OptionEditorModeClosed
	e.textInput.Blur()
}

// CycleRight advances the cursor one position in enum mode, wrapping around.
// No-op if not in enum mode.
func (e *OptionEditor) CycleRight() {
	if e.mode != OptionEditorModeEnumCycle {
		return
	}
	if len(e.values) == 0 {
		return
	}
	e.cursor = (e.cursor + 1) % len(e.values)
}

// CycleLeft retreats the cursor one position in enum mode, wrapping around.
// No-op if not in enum mode.
func (e *OptionEditor) CycleLeft() {
	if e.mode != OptionEditorModeEnumCycle {
		return
	}
	if len(e.values) == 0 {
		return
	}
	e.cursor = (e.cursor + len(e.values) - 1) % len(e.values)
}

// CurrentValue returns the value at the current cursor position in enum mode,
// the text input value in text mode, or the snapshot in closed mode.
func (e *OptionEditor) CurrentValue() string {
	switch e.mode {
	case OptionEditorModeEnumCycle:
		if e.cursor < len(e.values) {
			return e.values[e.cursor]
		}
		return e.snapshot
	case OptionEditorModeText:
		return e.textInput.Value()
	case OptionEditorModeFallbackPair:
		// Placeholder for fallback-pair mode (plan 09.6-04).
		return e.snapshot
	default: // OptionEditorModeClosed
		return e.snapshot
	}
}

// Snapshot returns the pre-edit value that will be restored by calling Dismiss.
func (e *OptionEditor) Snapshot() string {
	return e.snapshot
}

// Dismiss closes the editor and returns the snapshot, leaving no staged override.
// The snapshot is the value the row held at the moment the editor opened.
func (e *OptionEditor) Dismiss() string {
	e.Close()
	return e.snapshot
}

// Commit closes the editor and returns the currently-selected value as a staged
// override. This value will reach disk only after the preview + confirm ceremony.
func (e *OptionEditor) Commit() string {
	defer e.Close()
	return e.CurrentValue()
}

// Key returns the config key this editor is editing.
func (e *OptionEditor) Key() string {
	return e.key
}

// Mode returns the current editor mode.
func (e *OptionEditor) Mode() OptionEditorMode {
	return e.mode
}

// SetError sets an inline validation error message.
func (e *OptionEditor) SetError(msg string) {
	e.error = msg
}

// Error returns the current error message, if any.
func (e *OptionEditor) Error() string {
	return e.error
}

// TextInput returns the text input model (for text mode rendering).
func (e *OptionEditor) TextInput() textinput.Model {
	return e.textInput
}

// Cursor returns the current cursor position in enum mode.
func (e *OptionEditor) Cursor() int {
	return e.cursor
}

// Values returns the declared value set (for rendering).
func (e *OptionEditor) Values() []string {
	return e.values
}

// OptionRowView is a common interface for row types that can be edited via the
// shared OptionEditor. Both GlobalSSHOptionView and GlobalGitOptionView satisfy
// this interface, allowing the eligibility predicates to work with both screens
// (PD26: shared editor, reused not ported).
type OptionRowView interface {
	// GetKind returns the OptionValueKind classification (enum, text, toggle, bundle).
	GetKind() OptionValueKind
	// GetProbeError returns the probe error if any (non-empty means read-only).
	GetProbeError() string
	// IsWritable reports whether this row can be written to.
	IsWritable() bool
}

// OptionEditEligibility reports whether a row can be edited based on its state
// and type. An edit-eligible row is:
// - An enum or text kind (not toggle, not bundle)
// - In needs-action, already-set, or differs state (not not-applicable)
// - Carrying no probe error
// - Writable to the target block (Host * for SSH, PolicyBacked+HasWritableMember for Git)
// (PD16, PD26)
func OptionEditEligibility(o OptionRowView) bool {
	// Bundle and toggle rows have no editors; the checkbox is the only control.
	if o.GetKind() == OptionValueKindBundle || o.GetKind() == OptionValueKindToggle {
		return false
	}
	// A row carrying a probe error is read-only.
	if o.GetProbeError() != "" {
		return false
	}
	// A row not writable to the target is read-only.
	if !o.IsWritable() {
		return false
	}
	// Note: We cannot check State equality here since SSH and Git have different
	// state enums. The concrete implementations (GlobalSSHOptionView.IsEditEligible,
	// GlobalGitOptionView.IsEditEligible) handle state-specific logic.
	// For now, assume all writable non-bundle/toggle rows with no probe error are
	// editable. The state checks happen at the call site in globalssh.go/globalgit.go.
	return true
}

// OptionApplyEligibility reports whether a row should be included in an apply
// set, taking into account any staged overrides (PD16). This is a simpler predicate
// that just checks for staged overrides; the screen-specific logic handles state
// eligibility (PD26).
func OptionApplyEligibility(o OptionRowView, hasOverride bool) bool {
	// A row with a staged override is eligible for apply.
	// (The needs-action check happens in the screen-specific callers)
	return hasOverride && o.IsWritable()
}
