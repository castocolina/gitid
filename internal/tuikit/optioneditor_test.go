package tuikit

import (
	"testing"
)

// Test 1: With StrictHostKeyChecking selected and no editor open, the edit key
// opens an arrow-cycle editor whose cursor sits on the row's CURRENT resolved value;
// a current value absent from the declared set positions the cursor at index 0.
func TestOptionEditorOpenEnumCycle(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		values       []string
		currentValue string
		expectedCursor int
	}{
		{
			name:           "cursor on current value",
			key:            "StrictHostKeyChecking",
			values:         []string{"ask", "accept-new", "reject-new", "off"},
			currentValue:   "accept-new",
			expectedCursor: 1,
		},
		{
			name:           "exotic value positions cursor at 0",
			key:            "AddKeysToAgent",
			values:         []string{"yes", "no", "ask", "30m"},
			currentValue:   "30m",
			expectedCursor: 3,
		},
		{
			name:           "out-of-set current value positions cursor at 0",
			key:            "StrictHostKeyChecking",
			values:         []string{"ask", "accept-new", "reject-new", "off"},
			currentValue:   "/custom/socket",
			expectedCursor: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			editor := NewOptionEditor(tt.key, tt.values, tt.currentValue)
			if editor.IsOpen() {
				t.Errorf("expected editor closed, got open")
			}
			if editor.Mode() != OptionEditorModeClosed {
				t.Errorf("expected mode %q, got %q", OptionEditorModeClosed, editor.Mode())
			}

			editor.OpenEnumCycle(tt.currentValue)
			if !editor.IsOpen() {
				t.Errorf("expected editor open after OpenEnumCycle")
			}
			if editor.Mode() != OptionEditorModeEnumCycle {
				t.Errorf("expected mode %q, got %q", OptionEditorModeEnumCycle, editor.Mode())
			}
			if editor.Cursor() != tt.expectedCursor {
				t.Errorf("expected cursor %d, got %d", tt.expectedCursor, editor.Cursor())
			}
		})
	}
}

// Test 2: While the editor is open, `right` advances the cursor modulo the
// value-set length and `left` retreats it modulo the value-set length, and
// NEITHER key changes sub-tab. Driving `right` more times than the set has
// entries wraps rather than overflowing.
func TestOptionEditorCycling(t *testing.T) {
	editor := NewOptionEditor("StrictHostKeyChecking",
		[]string{"ask", "accept-new", "reject-new", "off"}, "ask")
	editor.OpenEnumCycle("ask")

	// Test right cycling
	tests := []struct {
		action   string
		expected int
	}{
		{"right", 1},
		{"right", 2},
		{"right", 3},
		{"right", 0}, // wrap around
		{"left", 3},  // wrap around backwards
		{"left", 2},
		{"left", 1},
		{"left", 0},
		{"left", 3}, // wrap backwards again
	}

	for _, tt := range tests {
		if tt.action == "right" {
			editor.CycleRight()
		} else {
			editor.CycleLeft()
		}
		if editor.Cursor() != tt.expected {
			t.Errorf("after %s, expected cursor %d, got %d", tt.action, tt.expected, editor.Cursor())
		}
	}
}

// Test 3: While the editor is open, Enter closes it, records the cycled value
// as a staged override for that key, and marks the row chosen.
func TestOptionEditorCommit(t *testing.T) {
	editor := NewOptionEditor("StrictHostKeyChecking",
		[]string{"ask", "accept-new", "reject-new", "off"}, "ask")
	editor.OpenEnumCycle("ask")

	// Cycle to a different value
	editor.CycleRight() // now at "accept-new"
	editor.CycleRight() // now at "reject-new"

	// Commit should return the current value and close the editor
	committed := editor.Commit()
	if committed != "reject-new" {
		t.Errorf("expected committed value %q, got %q", "reject-new", committed)
	}
	if editor.IsOpen() {
		t.Errorf("expected editor closed after Commit")
	}
	if editor.Mode() != OptionEditorModeClosed {
		t.Errorf("expected mode %q, got %q", OptionEditorModeClosed, editor.Mode())
	}
}

// Test 4: While the editor is open, Esc closes it, restores the pre-edit value,
// and leaves NO staged override for that key. Re-opening after an Enter snapshots
// the COMMITTED value, so a second Esc reverts to what the first Enter staged,
// never to the original probe value.
func TestOptionEditorDismissAndReopen(t *testing.T) {
	editor := NewOptionEditor("StrictHostKeyChecking",
		[]string{"ask", "accept-new", "reject-new", "off"}, "ask")
	editor.OpenEnumCycle("ask")

	// Cycle to a different value
	editor.CycleRight()
	editor.CycleRight() // now at "reject-new"

	// First Dismiss should restore the snapshot ("ask")
	dismissed := editor.Dismiss()
	if dismissed != "ask" {
		t.Errorf("expected dismissed value %q, got %q", "ask", dismissed)
	}
	if editor.IsOpen() {
		t.Errorf("expected editor closed after Dismiss")
	}
	if editor.CurrentValue() != "ask" {
		t.Errorf("expected current value to stay at snapshot %q, got %q", "ask", editor.CurrentValue())
	}

	// Re-open with a different value (simulating a second edit session)
	editor.OpenEnumCycle("accept-new")
	if editor.Snapshot() != "accept-new" {
		t.Errorf("expected new snapshot %q, got %q", "accept-new", editor.Snapshot())
	}

	// Cycle and then dismiss again
	editor.CycleRight() // now at "reject-new"
	dismissed2 := editor.Dismiss()
	if dismissed2 != "accept-new" {
		t.Errorf("expected second dismissed value %q, got %q", "accept-new", dismissed2)
	}
}

// Test 5: Edit eligibility per row state. Only enum or text rows that are
// needs-action, already-set, or differs are edit-eligible.
func TestOptionEditEligibility(t *testing.T) {
	tests := []struct {
		name     string
		view     GlobalSSHOptionView
		eligible bool
	}{
		{
			name: "enum needs-action eligible",
			view: GlobalSSHOptionView{
				State:             GlobalSSHNeedsAction,
				Kind:              OptionValueKindEnum,
				WritableToHostStar: true,
				ProbeError:        "",
			},
			eligible: true,
		},
		{
			name: "enum already-set eligible",
			view: GlobalSSHOptionView{
				State:             GlobalSSHAlreadySet,
				Kind:              OptionValueKindEnum,
				WritableToHostStar: true,
				ProbeError:        "",
			},
			eligible: true,
		},
		{
			name: "enum differs eligible",
			view: GlobalSSHOptionView{
				State:             GlobalSSHDiffers,
				Kind:              OptionValueKindEnum,
				WritableToHostStar: true,
				ProbeError:        "",
			},
			eligible: true,
		},
		{
			name: "toggle not eligible",
			view: GlobalSSHOptionView{
				State:             GlobalSSHNeedsAction,
				Kind:              OptionValueKindToggle,
				WritableToHostStar: true,
				ProbeError:        "",
			},
			eligible: false,
		},
		{
			name: "bundle not eligible",
			view: GlobalSSHOptionView{
				State:             GlobalSSHNeedsAction,
				Kind:              OptionValueKindBundle,
				WritableToHostStar: true,
				ProbeError:        "",
			},
			eligible: false,
		},
		{
			name: "not-applicable not eligible",
			view: GlobalSSHOptionView{
				State:             GlobalSSHNotApplicable,
				Kind:              OptionValueKindEnum,
				WritableToHostStar: true,
				ProbeError:        "",
			},
			eligible: false,
		},
		{
			name: "probe error not eligible",
			view: GlobalSSHOptionView{
				State:             GlobalSSHNeedsAction,
				Kind:              OptionValueKindEnum,
				WritableToHostStar: true,
				ProbeError:        "connection failed",
			},
			eligible: false,
		},
		{
			name: "not writable to host star not eligible",
			view: GlobalSSHOptionView{
				State:             GlobalSSHNeedsAction,
				Kind:              OptionValueKindEnum,
				WritableToHostStar: false,
				ProbeError:        "",
			},
			eligible: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eligible := OptionEditEligibility(tt.view)
			if eligible != tt.eligible {
				t.Errorf("expected eligible %v, got %v", tt.eligible, eligible)
			}
		})
	}
}

// Test 6: Apply eligibility. A staged override on an ALREADY-SET row still
// appears in the apply-chosen key set — the pre-existing needs-action filter
// does not drop it.
func TestOptionApplyEligibility(t *testing.T) {
	tests := []struct {
		name        string
		state       GlobalSSHOptionState
		hasOverride bool
		eligible    bool
	}{
		{
			name:        "needs-action always eligible",
			state:       GlobalSSHNeedsAction,
			hasOverride: false,
			eligible:    true,
		},
		{
			name:        "needs-action with override still eligible",
			state:       GlobalSSHNeedsAction,
			hasOverride: true,
			eligible:    true,
		},
		{
			name:        "already-set without override not eligible",
			state:       GlobalSSHAlreadySet,
			hasOverride: false,
			eligible:    false,
		},
		{
			name:        "already-set with override is eligible",
			state:       GlobalSSHAlreadySet,
			hasOverride: true,
			eligible:    true,
		},
		{
			name:        "differs without override not eligible",
			state:       GlobalSSHDiffers,
			hasOverride: false,
			eligible:    false,
		},
		{
			name:        "differs with override not eligible",
			state:       GlobalSSHDiffers,
			hasOverride: true,
			eligible:    false,
		},
		{
			name:        "not-applicable never eligible",
			state:       GlobalSSHNotApplicable,
			hasOverride: false,
			eligible:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			view := GlobalSSHOptionView{State: tt.state}
			eligible := OptionApplyEligibility(view, tt.hasOverride)
			if eligible != tt.eligible {
				t.Errorf("expected eligible %v, got %v", tt.eligible, eligible)
			}
		})
	}
}

// Test 11: Exotic current values (PD33). The editor opens with cursor at index 0
// for an out-of-set value, and never appends it to the set.
func TestOptionEditorExoticCurrentValues(t *testing.T) {
	tests := []struct {
		name         string
		values       []string
		currentValue string
		expectedCursor int
	}{
		{
			name:           "socket path positions at 0",
			values:         []string{"yes", "no", "ask"},
			currentValue:   "/tmp/agent.sock",
			expectedCursor: 0,
		},
		{
			name:           "environment variable positions at 0",
			values:         []string{"yes", "no", "ask"},
			currentValue:   "$SSH_AUTH_SOCK",
			expectedCursor: 0,
		},
		{
			name:           "time interval positions at 0",
			values:         []string{"yes", "no", "ask"},
			currentValue:   "30m",
			expectedCursor: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			editor := NewOptionEditor("AddKeysToAgent", tt.values, tt.currentValue)
			editor.OpenEnumCycle(tt.currentValue)

			if editor.Cursor() != tt.expectedCursor {
				t.Errorf("expected cursor %d, got %d", tt.expectedCursor, editor.Cursor())
			}
			if editor.Snapshot() != tt.currentValue {
				t.Errorf("expected snapshot %q, got %q", tt.currentValue, editor.Snapshot())
			}

			// Open and dismiss should leave the exotic value untouched
			dismissed := editor.Dismiss()
			if dismissed != tt.currentValue {
				t.Errorf("expected dismissed value %q, got %q", tt.currentValue, dismissed)
			}

			// The value set should never include the exotic value
			for _, v := range editor.Values() {
				if v == tt.currentValue {
					t.Errorf("expected exotic value %q to not be in declared set %v", tt.currentValue, editor.Values())
				}
			}
		})
	}
}

// Test 13: Detail pane row budget. The editor can accommodate the six-value set
// (PD46 worst case from diff.colorMoved policy).
func TestOptionEditorValueSetCapacity(t *testing.T) {
	// Six-value set from diff.colorMoved (PD7)
	colorMovedValues := []string{"no", "default", "plain", "blocks", "zebra", "dimmed-zebra"}

	editor := NewOptionEditor("diff.colorMoved", colorMovedValues, "no")
	editor.OpenEnumCycle("no")

	// Verify we can cycle through all six values
	expectedValues := colorMovedValues
	actualValues := editor.Values()

	if len(actualValues) != len(expectedValues) {
		t.Errorf("expected %d values, got %d", len(expectedValues), len(actualValues))
	}

	for i, expected := range expectedValues {
		if actualValues[i] != expected {
			t.Errorf("value %d: expected %q, got %q", i, expected, actualValues[i])
		}
	}

	// Cycle through all of them
	for i := 0; i < len(expectedValues); i++ {
		if editor.CurrentValue() != expectedValues[i] {
			t.Errorf("at position %d: expected %q, got %q", i, expectedValues[i], editor.CurrentValue())
		}
		if i < len(expectedValues)-1 {
			editor.CycleRight()
		}
	}
}

// Test: OptionEditor closes the editor and clears text input on Close()
func TestOptionEditorClose(t *testing.T) {
	editor := NewOptionEditor("StrictHostKeyChecking",
		[]string{"ask", "accept-new", "reject-new", "off"}, "ask")
	editor.OpenEnumCycle("ask")

	if !editor.IsOpen() {
		t.Errorf("expected editor open before Close")
	}

	editor.Close()

	if editor.IsOpen() {
		t.Errorf("expected editor closed after Close")
	}
	if editor.Mode() != OptionEditorModeClosed {
		t.Errorf("expected mode %q, got %q", OptionEditorModeClosed, editor.Mode())
	}
}

// Test: OptionEditor Key() returns the config key
func TestOptionEditorKey(t *testing.T) {
	editor := NewOptionEditor("StrictHostKeyChecking",
		[]string{"ask", "accept-new"}, "ask")

	if editor.Key() != "StrictHostKeyChecking" {
		t.Errorf("expected key %q, got %q", "StrictHostKeyChecking", editor.Key())
	}
}

// Test: OptionEditor error handling
func TestOptionEditorError(t *testing.T) {
	editor := NewOptionEditor("init.defaultBranch",
		[]string{"main", "master"}, "main")

	if editor.Error() != "" {
		t.Errorf("expected no error initially, got %q", editor.Error())
	}

	editor.SetError("invalid ref name")
	if editor.Error() != "invalid ref name" {
		t.Errorf("expected error %q, got %q", "invalid ref name", editor.Error())
	}
}
