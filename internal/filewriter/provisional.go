package filewriter

// ProvisionalBeginPrefix and ProvisionalEndPrefix are the sentinel line
// prefixes that delimit a gitid PROVISIONAL block. A provisional block for
// identity <name> spans:
//
//	# BEGIN gitid provisional: <name>
//	<body>
//	# END gitid provisional: <name>
//
// Provisional blocks are DISTINCT from managed blocks (BeginPrefix /
// EndPrefix). ListBlocks (the managed scan) never returns provisional blocks,
// and ListProvisionalBlocks never returns managed blocks — the two sentinel
// namespaces are mutually exclusive (T-05.7-14-01).
//
// A provisional block records a Host stanza written before the SSH alias has
// been tested, so the alias resolves to the staged (temp) key during the test.
// On test success the block is promoted to a managed block via Promote; on
// cancel or failure it is dropped via DropProvisional. Neither operation ever
// leaves a half-state (T-05.7-14-05) because every write goes through the
// filewriter chokepoint (backup + atomic + parse-validate).
const (
	ProvisionalBeginPrefix = "# BEGIN gitid provisional: "
	ProvisionalEndPrefix   = "# END gitid provisional: "
)

// ReplaceProvisionalBlock returns existing with the gitid provisional block for
// name set to blockBody, using the same bounded line-range splice as
// ReplaceBlock — it never touches managed blocks or foreign content
// (T-05.7-14-03). If a provisional block for name already exists, only the
// lines from its provisional BEGIN marker through its provisional END marker
// (inclusive) are replaced; every other line is byte-identical before and
// after. If no such provisional block exists, a new one is appended after all
// existing content. Calling twice with the same name and body yields
// byte-identical output (SAFE-02 idempotency, mirrored for provisional).
func ReplaceProvisionalBlock(existing []byte, name, blockBody string) []byte {
	return replaceBlockWith(existing, name, blockBody, ProvisionalBeginPrefix, ProvisionalEndPrefix)
}

// RemoveProvisionalBlock returns content with the gitid provisional block for
// name removed. If no such block exists the input is returned unchanged
// (idempotent). Only the provisional block's own lines (BEGIN..END inclusive)
// are removed; all surrounding content — including any blank line placed after
// the END marker as a separator, managed blocks, and foreign content — is
// preserved byte-for-byte (WR-04, T-05.7-14-03).
func RemoveProvisionalBlock(content []byte, name string) []byte {
	return removeBlockWith(content, name, ProvisionalBeginPrefix, ProvisionalEndPrefix)
}

// ListProvisionalBlocks scans content for all complete gitid provisional blocks
// and returns them in file order. Managed blocks are NOT returned (the two
// sentinel namespaces are mutually exclusive — T-05.7-14-01). Incomplete blocks
// (BEGIN with no matching END) are silently skipped. CRLF is normalised to LF
// before scanning so Windows-synced configs parse correctly.
func ListProvisionalBlocks(content []byte) []NamedBlock {
	return listBlocksWith(content, ProvisionalBeginPrefix, ProvisionalEndPrefix)
}
