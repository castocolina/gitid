package tui

import (
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"

	"github.com/castocolina/gitid/internal/doctor"
	"github.com/castocolina/gitid/internal/gitconfig"
	"github.com/castocolina/gitid/internal/identity"
)

// identityItem wraps an identity.Account to satisfy the bubbles/v2 list.Item
// interface. FilterValue and Title return the identity name; Description returns
// the provider (UI-SPEC Screen 2 item format).
type identityItem struct {
	account identity.Account
}

func (i identityItem) FilterValue() string { return i.account.Name }
func (i identityItem) Title() string       { return i.account.Name }
func (i identityItem) Description() string { return i.account.Provider }

// identityListModel is the bubbles/v2 list-backed Identity List screen (TUI-02).
type identityListModel struct {
	list       list.Model
	width      int
	height     int
	doctorDeps doctor.Deps
	deps       tuiDeps
}

// newIdentityListScreen builds the identity list screen by reconstructing
// accounts from disk via identity.Reconstruct (reading ~/.ssh/config and
// ~/.gitconfig using the injected ReadFile seam). When ReadFile is nil (test
// mode) or reconstruction yields no results, it falls back to d.Identities
// (the pre-reconstructed list populated by buildTUIDoctorDeps), ensuring both
// production and test paths produce a populated list.
// list.SetShowHelp(false) defers to the shared help footer bar (UI-SPEC).
// It accepts the full tuiDeps so the downstream create/detail/form/prove chain
// receives the real identity + update write seams (CR-02); reconstruction uses
// deps.doctor for the ReadFile/path seams.
func newIdentityListScreen(deps tuiDeps) screenModel {
	d := deps.doctor
	var accounts []identity.Account

	// Prefer a fresh Reconstruct from disk so the list reflects current state.
	if d.ReadFile != nil {
		var sshBytes, gcBytes []byte
		if d.SSHConfigPath != "" {
			sshBytes, _ = d.ReadFile(d.SSHConfigPath) //nolint:gosec // trusted gitid-managed path (G304)
		}
		if d.GitconfigPath != "" {
			gcBytes, _ = d.ReadFile(d.GitconfigPath) //nolint:gosec // trusted gitid-managed path (G304)
		}
		accounts, _ = identity.Reconstruct(sshBytes, gcBytes, gitconfig.ReadFragment)
	}

	// Fall back to pre-reconstructed identities (deps.Identities) for tests
	// where ReadFile is nil, and for production when Reconstruct returns nothing.
	if len(accounts) == 0 {
		accounts = d.Identities
	}

	items := make([]list.Item, len(accounts))
	for i, a := range accounts {
		items[i] = identityItem{account: a}
	}
	l := list.New(items, list.NewDefaultDelegate(), 0, 0)
	l.SetShowHelp(false)
	return identityListModel{
		list:       l,
		doctorDeps: d,
		deps:       deps,
	}
}

// update handles messages for the identity list screen.
func (m identityListModel) update(msg tea.Msg) (screenModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, keys.Back):
			return m, popCmd()
		case key.Matches(msg, keys.Add):
			return m, pushCmd(newCreateFormScreen(m.deps))
		case key.Matches(msg, keys.Select):
			if item, ok := m.list.SelectedItem().(identityItem); ok {
				acct := fillAccountPaths(item.account, m.doctorDeps)
				return m, pushCmd(newIdentityDetailScreen(acct, m.deps))
			}
			return m, nil
		case key.Matches(msg, keys.Delete):
			// D-03: delete is CLI-only; TUI shows handoff (not yet rendered in stub).
			return m, nil
		case key.Matches(msg, keys.Rotate):
			// D-03: rotate is CLI-only; TUI shows handoff (not yet rendered in stub).
			return m, nil
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.list.SetSize(msg.Width, msg.Height-4)
		return m, nil
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

// view renders the identity list screen with the title and bubbles list.
func (m identityListModel) view() string {
	return StyleTitle.Render("gitid — Identities") + "\n\n" + m.list.View()
}

// fillAccountPaths fills the gitid-managed target paths on a reconstructed
// Account from the doctor.Deps path seams when they are empty (CR-02/CR-03).
// Reconstruction populates the per-identity fields (Name, KeyPath, PubPath,
// Matches, ...) but not the global write-target paths (~/.ssh/config,
// ~/.gitconfig, allowed_signers, fragment); the update/add-account write modes
// need these to route ReplaceBlock at the correct managed blocks. Existing
// non-empty values are preserved.
func fillAccountPaths(acct identity.Account, d doctor.Deps) identity.Account {
	if acct.SSHConfigPath == "" {
		acct.SSHConfigPath = d.SSHConfigPath
	}
	if acct.GitconfigPath == "" {
		acct.GitconfigPath = d.GitconfigPath
	}
	if acct.AllowedSignersPath == "" {
		acct.AllowedSignersPath = d.AllowedSignersPath
	}
	if acct.FragmentPath == "" {
		acct.FragmentPath = fragmentPathDefault(acct.Name)
	}
	return acct
}

// fragmentPathDefault returns the conventional per-identity fragment path
// ~/.gitconfig.d/<name> keyed by identity name (mirrors identity.fragmentPathFor).
func fragmentPathDefault(name string) string {
	return "~/.gitconfig.d/" + name
}

// newCreateFormScreen is provided in tui/createform.go (05-04).
// newIdentityDetailScreen is provided in tui/identitydetail.go (05-04).
