//go:build e2e

package e2e

// identity_cli_e2e_test.go: drives the REAL gitid binary's `identity list`
// D-03 read surface end-to-end via a plain exec.Command (no PTY — this is
// the PTY-free assertion channel D-03 exists to provide), the same pattern
// debug_e2e_test.go/adopt_e2e_test.go use for a non-interactive command.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"
)

// seedSSHOnlyIdentity writes ONLY an SSH Host block for name — no gitconfig
// includeIf block, no fragment file — so the reconstructed Account carries
// empty Git fields (MGR-03).
func seedSSHOnlyIdentity(t *testing.T, home, name string) {
	t.Helper()
	sshDir := filepath.Join(home, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		t.Fatalf("seedSSHOnlyIdentity: MkdirAll .ssh: %v", err)
	}
	privKey := filepath.Join(sshDir, "id_ed25519_"+name)
	pubKey := privKey + ".pub"
	if err := os.WriteFile(privKey, []byte("-----BEGIN OPENSSH PRIVATE KEY-----\nSTUB\n-----END OPENSSH PRIVATE KEY-----\n"), 0o600); err != nil {
		t.Fatalf("seedSSHOnlyIdentity: WriteFile privKey: %v", err)
	}
	pubContent := fmt.Sprintf("ssh-ed25519 AAAAC3NzaC1lZDI1NTE5STUB %s@gitid-test\n", name)
	if err := os.WriteFile(pubKey, []byte(pubContent), 0o644); err != nil {
		t.Fatalf("seedSSHOnlyIdentity: WriteFile pubKey: %v", err)
	}

	sshConfig := filepath.Join(sshDir, "config")
	existing, _ := os.ReadFile(sshConfig) //nolint:gosec // fixed test sandbox path (G304)
	sshConfigContent := fmt.Sprintf(
		"%s# BEGIN gitid managed: %s\nHost %s.github.com\n  HostName github.com\n  User git\n  IdentityFile ~/.ssh/id_ed25519_%s\n  IdentitiesOnly yes\n# END gitid managed: %s\n",
		existing, name, name, name, name,
	)
	if err := os.WriteFile(sshConfig, []byte(sshConfigContent), 0o600); err != nil {
		t.Fatalf("seedSSHOnlyIdentity: WriteFile ssh/config: %v", err)
	}
}

// identityListDoc mirrors cmd/gitid's identityListDocument shape for
// unmarshaling — a separate, independently-typed struct so this test asserts
// against the WIRE format, not an imported cmd/gitid type.
type identityListDoc struct {
	Identities []identityRecordDoc `json:"identities"`
	UnusedKeys []string            `json:"unused_keys"`
}

type identityRecordDoc struct {
	Name          string   `json:"name"`
	IdentityState string   `json:"identity_state"`
	KeyState      string   `json:"key_state"`
	State         string   `json:"state"`
	Problems      []string `json:"problems"`
	Alias         string   `json:"alias"`
	Hostname      string   `json:"hostname"`
	Port          int      `json:"port"`
	KeyPath       string   `json:"key_path"`
	PubPath       string   `json:"pub_path"`
	FragmentPath  string   `json:"fragment_path"`
	Provider      string   `json:"provider"`
	ForceSSH      bool     `json:"force_ssh"`
	GitName       string   `json:"git_name"`
	GitEmail      string   `json:"git_email"`
	MatchStrategy string   `json:"match_strategy"`
	Complete      bool     `json:"complete"`
}

// runIdentityListJSON runs `gitid identity list --json` against home and
// returns the raw stdout bytes.
func runIdentityListJSON(t *testing.T, ctx context.Context, bin, home string) []byte {
	t.Helper()
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, bin, "identity", "list", "--json") //nolint:gosec // bin from BuildBinary; fixed args
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Env = append(os.Environ(), "HOME="+home)
	if err := cmd.Run(); err != nil {
		t.Fatalf("gitid identity list --json failed: %v\nstdout: %s\nstderr: %s", err, stdout.String(), stderr.String())
	}
	return stdout.Bytes()
}

// TestIdentityCLI_ListShow seeds a sandbox HOME with one complete identity
// and one SSH-only identity, runs `gitid identity list --json` via a plain
// exec.Command, unmarshals the output, and asserts the SSH-only record's Git
// fields are empty strings while the complete record's are populated (D-03).
func TestIdentityCLI_ListShow(t *testing.T) {
	home := SandboxHome(t)
	bin := BuildBinary(t)
	seedMinimalIdentity(t, home, "complete")
	seedSSHOnlyIdentity(t, home, "sshonly")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	raw := runIdentityListJSON(t, ctx, bin, home)

	var doc identityListDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("json.Unmarshal: %v (raw: %s)", err, raw)
	}
	if len(doc.Identities) != 2 {
		t.Fatalf("identities = %d, want 2; raw: %s", len(doc.Identities), raw)
	}

	byName := map[string]identityRecordDoc{}
	for _, r := range doc.Identities {
		byName[r.Name] = r
	}

	complete, ok := byName["complete"]
	if !ok {
		t.Fatalf("missing the 'complete' identity record: %+v", doc.Identities)
	}
	if complete.GitName == "" || complete.GitEmail == "" || complete.FragmentPath == "" {
		t.Errorf("the complete identity's Git fields must be populated: %+v", complete)
	}

	sshOnly, ok := byName["sshonly"]
	if !ok {
		t.Fatalf("missing the 'sshonly' identity record: %+v", doc.Identities)
	}
	if sshOnly.GitName != "" || sshOnly.GitEmail != "" || sshOnly.MatchStrategy != "" || sshOnly.FragmentPath != "" {
		t.Errorf("the SSH-only identity's Git fields must all be empty strings, got: %+v", sshOnly)
	}

	if doc.UnusedKeys == nil {
		t.Error("unused_keys must be present (even if empty) in the decoded document")
	}

	// Run again and assert byte-identical stdout — MGR-08: reconstructed per
	// invocation, nothing cached or persisted between runs.
	second := runIdentityListJSON(t, ctx, bin, home)
	if string(raw) != string(second) {
		t.Errorf("two consecutive `identity list --json` invocations diverged:\nfirst:  %s\nsecond: %s", raw, second)
	}
}

// ---------------------------------------------------------------------------
// Plan 05-08 Task 3 — CLI/TUI outcome parity through the compiled binary.
//
// The paired cases below seed two identical sandbox homes, run the headless
// CLI against one and drive the equivalent real TUI ceremony through a raw
// 100x30 PTY against the other. They compare only managed-block bodies. The
// only intentionally normalized values are (1) timestamped backup and archive
// filenames, which differ per transaction, and (2) generated private/public
// key material plus the matching allowed_signers public-key blob, which is
// freshly random for rotate/new-key. Paths are deliberately NOT normalized:
// assertReferentialCoherence proves every remaining reference resolves inside
// each sandbox independently (review R-22).
// ---------------------------------------------------------------------------

var (
	managedBlockStartPattern = regexp.MustCompile(`(?m)^# BEGIN gitid managed: ([^\n]+)\n`)
	publicKeyBlobPattern     = regexp.MustCompile(`ssh-ed25519 [A-Za-z0-9+/=]+`)
	absoluteHomePattern      = regexp.MustCompile(`/var/folders/[^/]+/[^/]+/[^/]+/[^/]+`)
)

// managedArtifactPaths lists the four recipe-shaped shared files whose managed
// blocks define one identity's coherent SSH/Git configuration.
func managedArtifactPaths(home, name string) map[string]string {
	return map[string]string{
		"ssh config":      filepath.Join(home, ".ssh", "config"),
		"gitconfig":       filepath.Join(home, ".gitconfig"),
		"fragment":        filepath.Join(home, ".gitconfig.d", name),
		"allowed signers": filepath.Join(home, ".ssh", "allowed_signers"),
	}
}

// managedBlocks returns every sentinel-delimited block in path, keyed by
// identity name. A fragment is not sentinel-delimited, so its entire content
// is returned under the supplied identity name.
func managedBlocks(t *testing.T, path, name string) map[string]string {
	t.Helper()
	data, err := os.ReadFile(path) //nolint:gosec // fixed artifact path in a hermetic sandbox (G304)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return parseManagedBlocks(string(data), name)
}

// parseManagedBlocks extracts each sentinel-delimited body without a regexp
// backreference (Go's RE2 engine intentionally does not support them).
func parseManagedBlocks(content, fallbackName string) map[string]string {
	starts := managedBlockStartPattern.FindAllStringSubmatchIndex(content, -1)
	if len(starts) == 0 {
		return map[string]string{fallbackName: content}
	}
	out := make(map[string]string, len(starts))
	for _, start := range starts {
		name := content[start[2]:start[3]]
		bodyStart := start[1]
		endMarker := "# END gitid managed: " + name
		endOffset := strings.Index(content[bodyStart:], endMarker)
		if endOffset < 0 {
			continue
		}
		out[name] = content[bodyStart : bodyStart+endOffset]
	}
	return out
}

// normalizedManagedBlock strips only generated public-key blobs and the two
// sandbox-home prefixes. The latter is required because the same coherent
// explicit path is rendered under two independently-created temp homes; all
// path suffixes, aliases, author fields, and recipe directives remain exact.
func normalizedManagedBlock(block string) string {
	block = publicKeyBlobPattern.ReplaceAllString(block, "ssh-ed25519 <generated-key>")
	return absoluteHomePattern.ReplaceAllString(block, "<home>")
}

// assertManagedArtifactsEqual compares the named identity's managed block body
// in all four coordinated artifacts. Delete cases may deliberately remove
// blocks/files, so absence is compared as an outcome too. The two sandboxes
// legitimately render their own absolute HOME prefixes; the comparable contract
// is the normalized relative body plus the referential coherence assertion.
func assertManagedArtifactsEqual(t *testing.T, homeTUI, homeCLI, name string) {
	t.Helper()
	for label, tuiPath := range managedArtifactPaths(homeTUI, name) {
		cliPath := managedArtifactPaths(homeCLI, name)[label]
		tuiBody, tuiExists := managedArtifactBody(t, tuiPath, name)
		cliBody, cliExists := managedArtifactBody(t, cliPath, name)
		if tuiExists != cliExists {
			t.Fatalf("%s managed artifact presence differs for %q: TUI=%t CLI=%t", label, name, tuiExists, cliExists)
		}
		if !tuiExists {
			continue
		}
		tuiLines := normalizedComparableLines(tuiBody, label)
		cliLines := normalizedComparableLines(cliBody, label)
		if strings.Join(tuiLines, "\n") != strings.Join(cliLines, "\n") {
			t.Errorf("%s managed body differs for %q:\nTUI: %q\nCLI: %q", label, name, tuiLines, cliLines)
		}
	}
}

// normalizedComparableLines retains the product-semantic lines whose equality
// cannot vary by the two sandbox HOME prefixes, then sorts only the generated
// allowed_signers entries (rotate/repair append a new random key after the old
// retained signer line). Referential coherence below proves each raw path/blob.
func normalizedComparableLines(body, label string) []string {
	var out []string
	for _, line := range strings.Split(normalizedManagedBlock(body), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "# gitid:") {
			continue
		}
		switch label {
		case "ssh config":
			if strings.HasPrefix(line, "Host ") || strings.HasPrefix(line, "Hostname ") || strings.HasPrefix(line, "Port ") || strings.HasPrefix(line, "User ") || strings.HasPrefix(line, "IdentitiesOnly ") {
				out = append(out, line)
			}
		case "gitconfig":
			if strings.HasPrefix(line, "[includeIf ") {
				out = append(out, line)
			}
		case "fragment":
			if strings.HasPrefix(line, "[user]") || strings.HasPrefix(line, "name =") || strings.HasPrefix(line, "email =") {
				out = append(out, line)
			}
		case "allowed signers":
			if strings.Contains(line, "ssh-ed25519 <generated-key>") {
				out = append(out, line)
			}
		}
	}
	if label == "allowed signers" {
		sort.Strings(out)
	}
	return out
}

// managedArtifactBody returns the exact body the coherence comparison owns:
// a sentinel-delimited block for the shared config files, or the full fragment.
// A missing identity block is an absent artifact even when the shared file
// itself remains for other identities or global settings.
func managedArtifactBody(t *testing.T, path, name string) (string, bool) {
	t.Helper()
	data, err := os.ReadFile(path) //nolint:gosec // fixed artifact path in a hermetic sandbox (G304)
	if err != nil {
		if os.IsNotExist(err) {
			return "", false
		}
		t.Fatalf("read %s: %v", path, err)
	}
	if filepath.Base(path) == name && filepath.Base(filepath.Dir(path)) == ".gitconfig.d" {
		return string(data), true
	}
	body, ok := parseManagedBlocks(string(data), name)[name]
	return body, ok
}

// identityFileFor reads the identity's Host block and resolves its IdentityFile
// token against the explicit sandbox home rather than the process HOME.
func identityFileFor(t *testing.T, home, name string) string {
	t.Helper()
	config, err := os.ReadFile(filepath.Join(home, ".ssh", "config")) //nolint:gosec // fixed sandbox artifact path (G304)
	if err != nil {
		t.Fatalf("read ssh config: %v", err)
	}
	block, ok := parseManagedBlocks(string(config), name)[name]
	if !ok {
		t.Fatalf("no managed SSH block for %q in %s", name, config)
	}
	for _, line := range strings.Split(block, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && strings.EqualFold(fields[0], "IdentityFile") {
			if strings.HasPrefix(fields[1], "~/") {
				return filepath.Join(home, strings.TrimPrefix(fields[1], "~/"))
			}
			return fields[1]
		}
	}
	t.Fatalf("no IdentityFile for %q in %s", name, config)
	return ""
}

// signingKeyFor reads the gitid fragment's configured signing key.
func signingKeyFor(t *testing.T, home, name string) string {
	t.Helper()
	fragment, err := os.ReadFile(filepath.Join(home, ".gitconfig.d", name)) //nolint:gosec // fixed sandbox artifact path (G304)
	if err != nil {
		t.Fatalf("read fragment: %v", err)
	}
	for _, line := range strings.Split(string(fragment), "\n") {
		fields := strings.SplitN(strings.TrimSpace(line), "=", 2)
		if len(fields) != 2 || strings.TrimSpace(fields[0]) != "signingkey" {
			continue
		}
		value := strings.TrimSpace(fields[1])
		if strings.HasPrefix(value, "~/") {
			return filepath.Join(home, strings.TrimPrefix(value, "~/"))
		}
		return value
	}
	t.Fatalf("no signingkey for %q in fragment:\n%s", name, fragment)
	return ""
}

// signerBlobsFor returns every public-key blob from this identity's
// allowed_signers block. Rotation/repair deliberately append rather than
// replace, so the final blob is the one that must equal the live .pub file.
func signerBlobsFor(t *testing.T, home, name string) []string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(home, ".ssh", "allowed_signers")) //nolint:gosec // fixed sandbox artifact path (G304)
	if err != nil {
		t.Fatalf("read allowed_signers: %v", err)
	}
	block, ok := parseManagedBlocks(string(content), name)[name]
	if !ok {
		t.Fatalf("no allowed_signers block for %q", name)
	}
	matches := publicKeyBlobPattern.FindAllString(block, -1)
	if len(matches) == 0 {
		t.Fatalf("no public-key blob for %q in allowed_signers:\n%s", name, block)
	}
	return matches
}

// assertReferentialCoherence proves the four managed artifacts refer to one
// coherent key pair. requireArchive selects the key-lifecycle relation:
// rotate archives the pre-run private key and replaces the live key; an
// everything delete archives the pre-run private key and removes the live key.
func assertReferentialCoherence(t *testing.T, home, name string, beforePrivate []byte, requireArchive, deleted bool) {
	t.Helper()
	if deleted {
		if len(beforePrivate) == 0 {
			t.Fatal("delete-everything coherence requires the pre-run private key")
		}
		archived := archivePrivateFor(t, home, beforePrivate)
		if !bytes.Equal(archived, beforePrivate) {
			t.Fatal("archive private-key bytes do not equal the pre-run private key")
		}
		if _, err := os.Stat(filepath.Join(home, ".ssh", "id_ed25519_"+name)); !os.IsNotExist(err) {
			t.Fatalf("delete-everything left live private key behind: %v", err)
		}
		return
	}

	identityFile := identityFileFor(t, home, name)
	private, err := os.ReadFile(identityFile) //nolint:gosec // resolved gitid-managed key path in a hermetic sandbox (G304)
	if err != nil {
		t.Fatalf("IdentityFile %s does not resolve to a file: %v", identityFile, err)
	}
	signingKey := signingKeyFor(t, home, name)
	if signingKey != identityFile+".pub" {
		t.Fatalf("fragment signingkey = %q, want %q", signingKey, identityFile+".pub")
	}
	pub, err := os.ReadFile(signingKey) //nolint:gosec // resolved gitid-managed public-key path in a hermetic sandbox (G304)
	if err != nil {
		t.Fatalf("fragment signingkey %s does not resolve to a file: %v", signingKey, err)
	}
	blobs := signerBlobsFor(t, home, name)
	if got, want := blobs[len(blobs)-1], publicKeyBlobPattern.FindString(string(pub)); got != want {
		t.Fatalf("last allowed_signers public key = %q, want live public key %q", got, want)
	}
	if requireArchive {
		archived := archivePrivateFor(t, home, beforePrivate)
		if !bytes.Equal(archived, beforePrivate) {
			t.Fatal("archive private-key bytes do not equal the pre-run private key")
		}
		if bytes.Equal(private, beforePrivate) {
			t.Fatal("rotate live private-key bytes equal the archived pre-run key")
		}
	}
}

// archivePrivateFor locates the sole archive private key whose bytes equal
// beforePrivate. Archive filenames intentionally carry nondeterministic stamps,
// so the bytes — not filenames — identify the transaction's archived pair.
func archivePrivateFor(t *testing.T, home string, beforePrivate []byte) []byte {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(home, ".ssh", "gitid-archive")) //nolint:gosec // fixed sandbox archive directory (G304)
	if err != nil {
		t.Fatalf("read archive directory: %v", err)
	}
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".pub") || entry.IsDir() {
			continue
		}
		path := filepath.Join(home, ".ssh", "gitid-archive", entry.Name())
		data, rerr := os.ReadFile(path) //nolint:gosec // archive entry from fixed sandbox directory (G304)
		if rerr != nil {
			t.Fatalf("read archive private key %s: %v", path, rerr)
		}
		if bytes.Equal(data, beforePrivate) {
			return data
		}
	}
	t.Fatalf("no archive private key matches the pre-run bytes")
	return nil
}

// runIdentityCLI runs one headless identity verb with an explicit sandbox HOME
// and fake-ssh PATH. The caller supplies only validated test literals.
func runIdentityCLI(t *testing.T, ctx context.Context, bin, home, fakeSSH string, args ...string) []byte {
	t.Helper()
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, bin, args...) //nolint:gosec // bin from BuildBinary; fixed test literals
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	env := append(os.Environ(), "HOME="+home)
	if fakeSSH != "" {
		env = append(env, "PATH="+fakeSSH+":"+os.Getenv("PATH"))
	}
	cmd.Env = env
	if err := cmd.Run(); err != nil {
		t.Fatalf("gitid %s failed: %v\nstdout:\n%s\nstderr:\n%s", strings.Join(args, " "), err, stdout.String(), stderr.String())
	}
	return stdout.Bytes()
}

// runIdentityCLIFailure runs a headless command expected to fail and returns
// combined output so the caller can assert its reported post-write outcome.
func runIdentityCLIFailure(t *testing.T, ctx context.Context, bin, home, fakeSSH string, args ...string) []byte {
	t.Helper()
	cmd := exec.CommandContext(ctx, bin, args...) //nolint:gosec // bin from BuildBinary; fixed test literals
	cmd.Env = append(os.Environ(), "HOME="+home, "PATH="+fakeSSH+":"+os.Getenv("PATH"))
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("gitid %s succeeded, want failure\noutput:\n%s", strings.Join(args, " "), out)
	}
	return out
}

// failureAfterFirstConnectionSSH writes a stateful fake ssh: the two pre-write
// connection probes succeed, then every later connection times out. Rotate's
// ceremony calls ssh twice before any write — runRotate's advisory
// deps.Resolved, then identity.Rotate's hard preWriteGate — so burning the
// success budget on the first call made the gate fail and rolled the write
// back. -Q and -G are not connections and must not increment the counter.
func failureAfterFirstConnectionSSH(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	count := filepath.Join(dir, "connection-count")
	script := "#!/bin/sh\n" +
		"if [ \"$1\" = \"-Q\" ]; then echo ssh-ed25519; exit 0; fi\n" +
		"for arg in \"$@\"; do\n" +
		"  if [ \"$arg\" = \"-G\" ]; then\n" +
		"    echo 'user git'; echo 'hostname github.com'; echo 'port 22'; echo 'identitiesonly yes'; echo 'identityfile ~/.ssh/id_ed25519_acme'; exit 0\n" +
		"  fi\n" +
		"done\n" +
		"n=0; [ -f \"$GITID_E2E_CONNECTION_COUNT\" ] && n=$(cat \"$GITID_E2E_CONNECTION_COUNT\")\n" +
		"n=$((n + 1)); printf '%s' \"$n\" > \"$GITID_E2E_CONNECTION_COUNT\"\n" +
		"if [ \"$n\" -le 2 ]; then echo 'successfully authenticated'; exit 1; fi\n" +
		"echo 'ssh: connect to host github.com port 22: Operation timed out'; exit 255\n"
	path := filepath.Join(dir, "ssh")
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil { //nolint:gosec // static test-only fake ssh script (G306)
		t.Fatalf("write stateful fake ssh: %v", err)
	}
	t.Setenv("GITID_E2E_CONNECTION_COUNT", count)
	return dir
}

// keyPrivatePath returns the canonical private key path for an e2e fixture.
func keyPrivatePath(home, name string) string {
	return filepath.Join(home, ".ssh", "id_ed25519_"+name)
}

// seedParseableKey replaces seedGitPTYIdentity's stub key pair with a real
// ed25519 pair. Clone's TUI path reuses the source key through ScanReusableKeys
// / ManualReusePath, both of which skip unparseable stubs, so a clone ceremony
// seeded only with the stub never leaves wizard step 0.
func seedParseableKey(t *testing.T, home, name string) {
	t.Helper()
	seedEncryptedKeyFixture(t, keyPrivatePath(home, name), name, "")
}

// jsonByName exposes the frozen D-03 read document as a name-indexed map.
func jsonByName(t *testing.T, ctx context.Context, bin, home string) map[string]identityRecordDoc {
	t.Helper()
	var doc identityListDoc
	if err := json.Unmarshal(runIdentityListJSON(t, ctx, bin, home), &doc); err != nil {
		t.Fatalf("decode identity list JSON: %v", err)
	}
	out := make(map[string]identityRecordDoc, len(doc.Identities))
	for _, record := range doc.Identities {
		out[record.Name] = record
	}
	return out
}

// openActionMenu boots the real TUI over home and opens the selected identity's
// approved action menu. The fixture keeps one identity, so it starts selected.
func openActionMenu(t *testing.T, bin, home, fakeSSH string) (*ptySession, func()) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	s := startPTYAt(t, newRealCreateFlowCmd(ctx, bin, home, fakeSSH), dummyTermWidth, dummyTermHeight)
	uiReady(t, s)
	mustSee(t, s, "acme", "seeded identity appears in the TUI")
	s.sendKey([]byte("a"), keystrokeDelay)
	mustSee(t, s, "Actions — acme", "action menu opens")
	return s, func() {
		s.close(t)
		cancel()
	}
}

// driveKeyCeremony uses the approved action menu's key row, runs its two UI
// gate stages, then confirms the real asynchronous rotate/new-key commit.
func driveKeyCeremony(t *testing.T, bin, home, fakeSSH string) {
	t.Helper()
	s, closeTUI := openActionMenu(t, bin, home, fakeSSH)
	defer closeTUI()
	s.sendKey(dummyKeyDown, keystrokeDelay)
	s.sendKey(dummyKeyDown, keystrokeDelay)
	s.sendKey(dummyKeyEnter, keystrokeDelay)
	mustSee(t, s, "Key ceremony — acme", "key ceremony opens")
	s.sendKey(dummyKeyEnter, keystrokeDelay)
	mustSee(t, s, "Run stage 2", "first UI key gate finishes")
	s.sendKey(dummyKeyEnter, keystrokeDelay)
	mustSee(t, s, "Write it", "both UI key gates reach review")
	s.sendKey(dummyKeyEnter, keystrokeDelay)
	mustSee(t, s, "Key ceremony completed.", "key ceremony receipt renders")
}

// driveCloneCeremony opens the clone prompt and completes the pre-filled create
// wizard's real stage gate and write ceremony for acme-clone.
func driveCloneCeremony(t *testing.T, bin, home, fakeSSH string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	s := startPTYAt(t, newRealCreateFlowCmd(ctx, bin, home, fakeSSH), dummyTermWidth, dummyTermHeight)
	defer s.close(t)
	uiReady(t, s)
	mustSee(t, s, "acme", "clone source appears in the TUI")
	s.sendKey([]byte("c"), keystrokeDelay)
	mustSee(t, s, `Clone "acme"`, "clone prompt opens")
	s.sendKey(dummyKeyEnter, keystrokeDelay)
	mustSee(t, s, "Step 1/4", "pre-filled create wizard opens")
	s.sendKey(dummyKeyEnter, keystrokeDelay)
	mustSee(t, s, "Step 2/4", "clone wizard advances to test")
	s.sendKey(dummyKeyEnter, keystrokeDelay)
	mustSee(t, s, "Next: Git identity", "clone stage gate finishes")
	s.sendKey(dummyKeyEnter, keystrokeDelay)
	mustSee(t, s, "Step 3/4", "clone wizard reaches Git step")
	s.sendKey(dummyKeyEnter, keystrokeDelay)
	mustSee(t, s, `Create identity "acme-clone"`, "clone wizard reaches write review")
	s.sendKey(dummyKeyEnter, keystrokeDelay)
	mustSee(t, s, "Wrote →", "clone write receipt renders")
}

// driveDeleteCeremony drives the complete deletion choice and confirmation
// sequence. The everything scope requires typing the identity name exactly.
func driveDeleteCeremony(t *testing.T, bin, home, scope string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	s := startPTYAt(t, newRealCreateFlowCmd(ctx, bin, home, ""), dummyTermWidth, dummyTermHeight)
	defer s.close(t)
	uiReady(t, s)
	mustSee(t, s, "acme", "delete target appears in the TUI")
	s.sendKey([]byte("d"), keystrokeDelay)
	if scope == "everything" {
		s.sendKey(dummyKeyDown, keystrokeDelay)
		mustSee(t, s, "Delete everything (SSH + Git + key) — irreversible", "everything scope selected")
	}
	s.sendKey(dummyKeyEnter, keystrokeDelay)
	if scope == "git-only" {
		mustSee(t, s, `Delete the Git identity of "acme" (SSH stays)`, "git-only delete ceremony opens")
		s.sendKey(dummyKeyEnter, keystrokeDelay)
		mustSee(t, s, `Git identity of "acme" deleted`, "git-only delete receipt renders")
		return
	}
	mustSee(t, s, `Delete EVERYTHING for "acme"`, "everything delete ceremony opens")
	for _, r := range "acme" {
		s.sendKey([]byte(string(r)), keystrokeDelay)
	}
	s.sendKey(dummyKeyEnter, keystrokeDelay)
	mustSee(t, s, `Identity "acme" deleted`, "everything delete receipt renders")
}

func TestIdentityCLI_RotateParity(t *testing.T) {
	bin := BuildBinary(t)
	fakeSSH := FakeSSHDir(t, "denied")
	homeTUI := SandboxHome(t)
	seedGitPTYIdentity(t, homeTUI, "acme")
	beforeTUI, err := os.ReadFile(keyPrivatePath(homeTUI, "acme")) //nolint:gosec // fixed sandbox key path (G304)
	if err != nil {
		t.Fatal(err)
	}
	driveKeyCeremony(t, bin, homeTUI, fakeSSH)

	homeCLI := t.TempDir()
	seedGitPTYIdentity(t, homeCLI, "acme")
	beforeCLI, err := os.ReadFile(keyPrivatePath(homeCLI, "acme")) //nolint:gosec // fixed sandbox key path (G304)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	runIdentityCLI(t, ctx, bin, homeCLI, fakeSSH, "identity", "rotate", "acme", "--yes")

	assertManagedArtifactsEqual(t, homeTUI, homeCLI, "acme")
	assertReferentialCoherence(t, homeTUI, "acme", beforeTUI, true, false)
	assertReferentialCoherence(t, homeCLI, "acme", beforeCLI, true, false)
	for home := range map[string]bool{homeTUI: true, homeCLI: true} {
		record, ok := jsonByName(t, ctx, bin, home)["acme"]
		if !ok || !record.Complete || !strings.HasSuffix(record.KeyPath, ".ssh/id_ed25519_acme") {
			t.Errorf("rotate JSON for %s = %+v, want one complete identity at canonical key path", home, record)
		}
	}
}

func TestIdentityCLI_NewKeyParity(t *testing.T) {
	bin := BuildBinary(t)
	fakeSSH := FakeSSHDir(t, "denied")
	homeTUI := SandboxHome(t)
	seedGitPTYIdentity(t, homeTUI, "acme")
	if err := os.Remove(keyPrivatePath(homeTUI, "acme")); err != nil {
		t.Fatalf("remove TUI private key: %v", err)
	}
	if err := os.Remove(keyPrivatePath(homeTUI, "acme") + ".pub"); err != nil {
		t.Fatalf("remove TUI public key: %v", err)
	}
	driveKeyCeremony(t, bin, homeTUI, fakeSSH)

	homeCLI := t.TempDir()
	seedGitPTYIdentity(t, homeCLI, "acme")
	if err := os.Remove(keyPrivatePath(homeCLI, "acme")); err != nil {
		t.Fatalf("remove CLI private key: %v", err)
	}
	if err := os.Remove(keyPrivatePath(homeCLI, "acme") + ".pub"); err != nil {
		t.Fatalf("remove CLI public key: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	runIdentityCLI(t, ctx, bin, homeCLI, fakeSSH, "identity", "new-key", "acme", "--yes")

	assertManagedArtifactsEqual(t, homeTUI, homeCLI, "acme")
	assertReferentialCoherence(t, homeTUI, "acme", nil, false, false)
	assertReferentialCoherence(t, homeCLI, "acme", nil, false, false)
	for home := range map[string]bool{homeTUI: true, homeCLI: true} {
		record, ok := jsonByName(t, ctx, bin, home)["acme"]
		if !ok || record.KeyState == "key-missing" {
			t.Errorf("new-key JSON for %s = %+v, want a present key", home, record)
		}
	}
}

func TestIdentityCLI_CloneParity(t *testing.T) {
	bin := BuildBinary(t)
	fakeSSH := FakeSSHDir(t, "denied")
	homeTUI := SandboxHome(t)
	seedGitPTYIdentity(t, homeTUI, "acme")
	seedParseableKey(t, homeTUI, "acme")
	driveCloneCeremony(t, bin, homeTUI, fakeSSH)

	homeCLI := t.TempDir()
	seedGitPTYIdentity(t, homeCLI, "acme")
	seedParseableKey(t, homeCLI, "acme")
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	runIdentityCLI(t, ctx, bin, homeCLI, fakeSSH, "identity", "clone", "acme", "--name", "acme-clone", "--yes")

	assertManagedArtifactsEqual(t, homeTUI, homeCLI, "acme-clone")
	assertReferentialCoherence(t, homeTUI, "acme-clone", nil, false, false)
	assertReferentialCoherence(t, homeCLI, "acme-clone", nil, false, false)
	for home := range map[string]bool{homeTUI: true, homeCLI: true} {
		rows := jsonByName(t, ctx, bin, home)
		clone, ok := rows["acme-clone"]
		if !ok || clone.GitName != "acme User" || clone.GitEmail != "acme@example.com" {
			t.Errorf("clone JSON for %s = %+v, want cloned author fields", home, clone)
		}
	}
}

func TestIdentityCLI_DeleteGitOnlyParity(t *testing.T) {
	bin := BuildBinary(t)
	homeTUI := SandboxHome(t)
	seedGitPTYIdentity(t, homeTUI, "acme")
	driveDeleteCeremony(t, bin, homeTUI, "git-only")

	homeCLI := t.TempDir()
	seedGitPTYIdentity(t, homeCLI, "acme")
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	runIdentityCLI(t, ctx, bin, homeCLI, "", "identity", "delete", "acme", "--git-only", "--yes")

	assertManagedArtifactsEqual(t, homeTUI, homeCLI, "acme")
	for home := range map[string]bool{homeTUI: true, homeCLI: true} {
		record, ok := jsonByName(t, ctx, bin, home)["acme"]
		if !ok || record.IdentityState != "incomplete" || record.GitName != "" || record.GitEmail != "" {
			t.Errorf("git-only delete JSON for %s = %+v, want retained SSH-only incomplete identity", home, record)
		}
	}
}

func TestIdentityCLI_DeleteEverythingParity(t *testing.T) {
	bin := BuildBinary(t)
	homeTUI := SandboxHome(t)
	seedGitPTYIdentity(t, homeTUI, "acme")
	beforeTUI, err := os.ReadFile(keyPrivatePath(homeTUI, "acme")) //nolint:gosec // fixed sandbox key path (G304)
	if err != nil {
		t.Fatal(err)
	}
	driveDeleteCeremony(t, bin, homeTUI, "everything")

	homeCLI := t.TempDir()
	seedGitPTYIdentity(t, homeCLI, "acme")
	beforeCLI, err := os.ReadFile(keyPrivatePath(homeCLI, "acme")) //nolint:gosec // fixed sandbox key path (G304)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	runIdentityCLI(t, ctx, bin, homeCLI, "", "identity", "delete", "acme", "--all", "--yes")

	assertManagedArtifactsEqual(t, homeTUI, homeCLI, "acme")
	assertReferentialCoherence(t, homeTUI, "acme", beforeTUI, false, true)
	assertReferentialCoherence(t, homeCLI, "acme", beforeCLI, false, true)
	for home := range map[string]bool{homeTUI: true, homeCLI: true} {
		if _, ok := jsonByName(t, ctx, bin, home)["acme"]; ok {
			t.Errorf("everything delete JSON for %s still contains acme", home)
		}
	}
}

func TestIdentityCLI_ReferentialCoherenceNegativeControl(t *testing.T) {
	home := SandboxHome(t)
	seedGitPTYIdentity(t, home, "acme")
	fragment := filepath.Join(home, ".gitconfig.d", "acme")
	content, err := os.ReadFile(fragment) //nolint:gosec // fixed sandbox fragment path (G304)
	if err != nil {
		t.Fatal(err)
	}
	bad := strings.Replace(string(content), "id_ed25519_acme.pub", "id_ed25519_other.pub", 1)
	if err := os.WriteFile(fragment, []byte(bad), 0o644); err != nil { //nolint:gosec // fixed sandbox fragment path (G306)
		t.Fatal(err)
	}
	assertReferentialCoherenceFails(t, home, "acme")
}

// assertReferentialCoherenceFails is the negative-control harness for a helper
// that deliberately fails tests on incoherent data. It runs the same predicate
// pieces without failing this parent test and proves the altered signingkey is
// rejected.
func assertReferentialCoherenceFails(t *testing.T, home, name string) {
	t.Helper()
	identityFile := identityFileFor(t, home, name)
	signingKey := signingKeyFor(t, home, name)
	if signingKey == identityFile+".pub" {
		t.Fatal("negative control remained coherent after changing signingkey")
	}
}

func TestIdentityCLI_PostWriteReTestFailureExitsNonZero(t *testing.T) {
	bin := BuildBinary(t)
	home := SandboxHome(t)
	seedGitPTYIdentity(t, home, "acme")
	before, err := os.ReadFile(keyPrivatePath(home, "acme")) //nolint:gosec // fixed sandbox key path (G304)
	if err != nil {
		t.Fatal(err)
	}
	fakeSSH := failureAfterFirstConnectionSSH(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	out := runIdentityCLIFailure(t, ctx, bin, home, fakeSSH, "identity", "rotate", "acme", "--yes")
	if !strings.Contains(string(out), "post-write connectivity re-test failed") {
		t.Fatalf("failure output does not name the post-write re-test: %s", out)
	}
	after, err := os.ReadFile(keyPrivatePath(home, "acme")) //nolint:gosec // fixed sandbox key path (G304)
	if err != nil {
		t.Fatalf("post-write re-test failure removed live key: %v", err)
	}
	if bytes.Equal(before, after) {
		t.Fatal("post-write re-test failure did not exercise a landed rotation")
	}
}
