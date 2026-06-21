package tui

import (
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/sahilm/fuzzy"
)

// paletteItem represents a single entry in the command palette.
type paletteItem struct {
	label  string // display text
	action string // internal action identifier
}

// paletteModel is the Ctrl+P command palette modal sub-model.
// It provides a textinput filter over a context-sensitive list of
// views and identity actions (D-14). Actions that are not yet wired
// (create, copy, etc.) emit the "coming next" toast stub until
// Plans 04-06 wire them.
type paletteModel struct {
	input     textinput.Model
	items     []paletteItem // all available items (unfiltered)
	filtered  []paletteItem // fuzzy-filtered subset
	cursor    int           // selected row in filtered list
	activated bool          // true when Enter pressed — caller reads selectedItem()
}

// paletteViewItems are the always-available view-switch items.
var paletteViewItems = []paletteItem{
	{label: "Identities — view 1", action: "view:identities"},
	{label: "Health — view 2", action: "view:health"},
	{label: "Global Options — view 3", action: "view:global"},
}

// newPaletteModel constructs a fresh palette model with a seeded filter input.
func newPaletteModel() paletteModel {
	ti := textinput.New()
	ti.Placeholder = "type to filter..."
	ti.Focus()

	items := make([]paletteItem, len(paletteViewItems))
	copy(items, paletteViewItems)

	return paletteModel{
		input:    ti,
		items:    items,
		filtered: items,
	}
}

// selectedItem returns the currently highlighted palette item.
// Should only be called when activated == true and len(filtered) > 0.
func (m paletteModel) selectedItem() paletteItem {
	if len(m.filtered) == 0 {
		return paletteItem{}
	}
	if m.cursor >= len(m.filtered) {
		return m.filtered[len(m.filtered)-1]
	}
	return m.filtered[m.cursor]
}

// update handles key and message events within the palette modal.
func (m paletteModel) update(msg tea.Msg) (paletteModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil

		case "down", "j":
			if m.cursor < len(m.filtered)-1 {
				m.cursor++
			}
			return m, nil

		case "enter":
			m.activated = true
			return m, nil
		}

	// Default: delegate to text input for filtering.
	default:
	}

	// Update text input.
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)

	// Re-apply fuzzy filter.
	query := m.input.Value()
	m.filtered = m.applyFilter(query)

	// Clamp cursor to filtered length.
	if m.cursor >= len(m.filtered) && len(m.filtered) > 0 {
		m.cursor = len(m.filtered) - 1
	} else if len(m.filtered) == 0 {
		m.cursor = 0
	}

	return m, cmd
}

// applyFilter returns items matching the query using sahilm/fuzzy.
// Falls back to case-insensitive substring matching when fuzzy returns no hits.
func (m paletteModel) applyFilter(query string) []paletteItem {
	if query == "" {
		return m.items
	}

	labels := make([]string, len(m.items))
	for i, it := range m.items {
		labels[i] = it.label
	}

	matches := fuzzy.Find(query, labels)
	if len(matches) == 0 {
		// Substring fallback.
		lower := strings.ToLower(query)
		var result []paletteItem
		for _, it := range m.items {
			if strings.Contains(strings.ToLower(it.label), lower) {
				result = append(result, it)
			}
		}
		return result
	}

	result := make([]paletteItem, len(matches))
	for i, match := range matches {
		result[i] = m.items[match.Index]
	}
	return result
}

// view renders the palette modal at the given terminal width.
func (m paletteModel) view(termW int) string {
	title := "Command Palette"

	var sb strings.Builder
	sb.WriteString(m.input.View())
	sb.WriteString("\n\n")

	if len(m.filtered) == 0 {
		sb.WriteString(StyleFaint.Render("No matching commands."))
	} else {
		for i, it := range m.filtered {
			if i == m.cursor {
				sb.WriteString(StyleSelected.Render("› " + it.label))
			} else {
				sb.WriteString(StyleBody.Render("  " + it.label))
			}
			if i < len(m.filtered)-1 {
				sb.WriteString("\n")
			}
		}
	}

	return modalBox(termW, title, sb.String())
}
