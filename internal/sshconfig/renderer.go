package sshconfig

import (
	"fmt"
	"strings"

	"github.com/castocolina/gitid/internal/platform"
)

// hostIndent is the two-space indentation OpenSSH config conventionally uses for
// directives nested under a Host stanza.
const hostIndent = "  "

// RenderHostBlock renders a managed SSH Host stanza for an identity.
//
// It emits, in order (SSH-01): the Host line for alias, then Hostname, Port,
// `User git`, IdentityFile, and `IdentitiesOnly yes`. The alias is the real
// provider host for a default identity or an `<identity>.<provider>` alias for
// an additional identity (SSH-02); both forms render identically here.
//
// `IdentitiesOnly yes` together with the explicit IdentityFile prevents the
// agent offering the wrong key to the provider (T-02-13).
//
// The returned text is the block BODY only (no sentinel markers); the writer
// wraps it in a gitid managed block keyed by the identity name.
func RenderHostBlock(alias, hostname string, port int, identityFile string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Host %s\n", alias)
	fmt.Fprintf(&b, "%sHostname %s\n", hostIndent, hostname)
	fmt.Fprintf(&b, "%sPort %d\n", hostIndent, port)
	fmt.Fprintf(&b, "%sUser git\n", hostIndent)
	fmt.Fprintf(&b, "%sIdentityFile %s\n", hostIndent, identityFile)
	fmt.Fprintf(&b, "%sIdentitiesOnly yes\n", hostIndent)
	return b.String()
}

// RenderGlobalBlock renders the macOS-only `Host *` keychain/agent stanza
// (SSH-03). On any OS where UseKeychain is unsupported (everything but darwin)
// it returns the empty string so no Apple-only directive is written.
//
// On darwin it emits, in order (Pitfall 4 / T-02-14): `IgnoreUnknown
// UseKeychain` first — so a Linux `ssh -G` reading a synced config does not
// error on the unknown directive — then `UseKeychain yes`, then
// `AddKeysToAgent yes`.
//
// The writer MUST place this block (keyed `_global`) LAST, after all specific
// host blocks, because ssh resolves Host patterns first-match-wins and a
// leading `Host *` would shadow the specific aliases (Pitfall 5 / T-02-15).
//
// The returned text is the block BODY only; the writer wraps it in a gitid
// managed block.
func RenderGlobalBlock(os string) string {
	if !platform.SupportsUseKeychain(os) {
		return ""
	}
	var b strings.Builder
	b.WriteString("Host *\n")
	fmt.Fprintf(&b, "%sIgnoreUnknown UseKeychain\n", hostIndent)
	fmt.Fprintf(&b, "%sUseKeychain yes\n", hostIndent)
	fmt.Fprintf(&b, "%sAddKeysToAgent yes\n", hostIndent)
	return b.String()
}
