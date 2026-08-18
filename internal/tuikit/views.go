package tuikit

// views.go defines the tuikit-LOCAL view DTOs every Backend method
// signature speaks.
//
// internal/tuikit imports ZERO first-party backend packages — no identity,
// tester, sshconfig, keygen, filewriter, doctor, adopter, platform,
// clipboard or uploader. That is a hard, mechanically-enforced contract
// (internal/dummytui's TestNoBackendAllowlist plus the Makefile's
// gate-no-backend-files target), so the seam can never name a backend
// type. The DTOs below exist precisely so it does not have to.
//
// CONVERSION HAPPENS IN EXACTLY ONE PLACE. keygen.ReusableKey becomes
// ReusableKeyView, and tester.Result / tester.Outcome become
// TestResultView / TestOutcome, inside the composition root
// cmd/gitid/wiring.go — never inside this package, and never inside a
// screen. The dummy's FixtureBackend constructs these DTOs directly from
// its fixtures. If a future screen appears to need a backend value, the
// answer is a new DTO here plus a conversion in cmd/gitid/wiring.go; it is
// never a new entry in the import allowlist.

// ReusableKeyView is one candidate key on the D-10 "reuse an existing key"
// picker — the view projection of a scanned private key. It REPLACES every
// use of keygen.ReusableKey inside this package.
type ReusableKeyView struct {
	// Path is the private key's path as shown to the user.
	Path string
	// Algorithm is the key's algorithm id (ed25519, rsa-4096, …).
	Algorithm string
	// Fingerprint is the SHA256 fingerprint shown next to the path.
	Fingerprint string
	// HasPub reports whether the sibling .pub file exists on disk.
	HasPub bool
	// Encrypted reports whether the private key is passphrase-protected —
	// informational (D-13), never a reason to hide the entry.
	Encrypted bool
	// InUseBy carries the D-12 "in use by: personal" label. Empty means the
	// key is not referenced by any existing identity.
	InUseBy string
}

// TestOutcome is the three-state result of a connectivity test — the view
// mirror of internal/tester's Outcome. It REPLACES every use of
// tester.Outcome inside this package. There are exactly three states; a
// fourth would be a design change, not an implementation detail.
type TestOutcome int

const (
	// TestOutcomePass is a successful authentication against the provider.
	TestOutcomePass TestOutcome = iota
	// TestOutcomeReachableNotUploaded means the provider answered but
	// rejected the key — reachable, key not registered yet. This is the
	// D-02 WARNING state, never a failure.
	TestOutcomeReachableNotUploaded
	// TestOutcomeFailure is everything else: unreachable host, timeout,
	// misconfiguration.
	TestOutcomeFailure
)

// TestResultView is one connectivity test's outcome as the create wizard
// renders it (TEST-01/TEST-02). It REPLACES every use of tester.Result
// inside this package.
type TestResultView struct {
	// Outcome classifies the attempt.
	Outcome TestOutcome
	// Command is the exact command that was run — TEST-01's shown ==
	// run contract, so the string displayed is never hand-built separately
	// from the one executed.
	Command string
	// Detail is the real ssh output line surfaced to the user (the
	// "Hi <name>! You've successfully authenticated…" greeting, the
	// "identityfile …" resolution proof, or the rejection line).
	Detail string
}

// CreateSpec is the create wizard's current SSH values, handed to the
// Backend for every create-flow effect. It carries only what the user has
// typed plus what the wizard derived from it — never a backend handle.
type CreateSpec struct {
	// Identity is the resolved identity name (the alias prefix, or the
	// provider host when the prefix is left blank).
	Identity string
	// Provider is the provider host inferred from the SSH Host alias and
	// the backend's known-provider table (D-20). It is preserved here so
	// multi-label/custom providers are not later truncated by a two-label
	// suffix reconstruction (WR-03).
	Provider string
	// Alias is the SSH Host alias the managed block declares.
	Alias string
	// Hostname is the real endpoint behind the alias.
	Hostname string
	// Port is the SSH port as typed (443 for the alt-SSH endpoint).
	Port string
	// KeyPath is the per-identity key the block points at.
	KeyPath string
	// Algorithm is the selected key algorithm id.
	Algorithm string
	// SimulateFailure is the D-16 demo control on the wizard's test step:
	// the dummy uses it to preview the error path. The real Backend
	// ignores it — a real test never simulates.
	SimulateFailure bool
	// ReuseKeyPath is the D-10 "reuse an existing key" selection: non-empty
	// means the create must NOT generate a new key pair, it must point the
	// identity at this existing key instead (KEY-06). Empty means generate.
	ReuseKeyPath string
}

// GitSpec is the create wizard's Git-identity values (wizard step 3).
type GitSpec struct {
	// Identity is the identity name the fragment and includeIf are keyed by.
	Identity string
	// Name is user.name.
	Name string
	// Email is user.email — kept byte-identical to the allowed_signers
	// entry (GITUI-04).
	Email string
	// Strategy is the includeIf match strategy (gitdir/hasconfig/both).
	Strategy string
	// KeyPath is the identity's key; user.signingkey is KeyPath + ".pub".
	KeyPath string
}

// WritePlanView is what a committed create will touch: the files written
// and the timestamped backups taken first (TEST-03/D-05..D-09). The dummy
// returns fixture paths; the real Backend returns the resolved storage
// target and the real backup paths.
type WritePlanView struct {
	// Targets are the files the write touches, in write order.
	Targets []string
	// Backups are the timestamped backup paths taken before writing.
	Backups []string
}

// WizardCommitMsg reports the result of an asynchronous create commit. It is
// delivered from the tea.Cmd returned by Backend.CommitCreate — success carries
// the real timestamped backup paths, failure carries the concrete error string.
type WizardCommitMsg struct {
	// Backups are the timestamped backup paths the transaction took.
	Backups []string
	// Err is non-empty when the transaction failed; the ceremony renders it
	// and offers retry/cancel instead of a receipt.
	Err string
}
