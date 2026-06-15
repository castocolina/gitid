package filewriter

import "strings"

// BeginPrefix and EndPrefix are the sentinel line prefixes that delimit a gitid
// managed block. A block for identity <name> spans:
//
//	# BEGIN gitid managed: <name>
//	<body>
//	# END gitid managed: <name>
const (
	BeginPrefix = "# BEGIN gitid managed: "
	EndPrefix   = "# END gitid managed: "
)

// ReplaceBlock returns existing with the gitid managed block for name set to
// blockBody, using a bounded line-range splice that never touches foreign
// content.
//
// If a block for name already exists, only the lines from its BEGIN marker
// through its END marker (inclusive) are replaced; every other line is byte
// identical before and after. If no such block exists, a new block is appended
// after all existing content. Calling ReplaceBlock twice with the same name and
// body yields byte-identical output (SAFE-02 idempotency).
//
// blockBody is stored verbatim between the markers with a single trailing
// newline, so repeated runs are stable regardless of trailing whitespace in the
// input body.
func ReplaceBlock(existing []byte, name, blockBody string) []byte {
	beginMarker := BeginPrefix + name
	endMarker := EndPrefix + name

	// Canonical block: markers wrapping the trimmed body, each line newline
	// terminated. Trimming trailing newlines keeps repeated writes stable.
	body := strings.TrimRight(blockBody, "\n")
	block := beginMarker + "\n" + body + "\n" + endMarker + "\n"

	lines := strings.SplitAfter(string(existing), "\n")

	beginIdx, endIdx := -1, -1
	for i, line := range lines {
		trimmed := strings.TrimRight(line, "\n")
		switch {
		case beginIdx == -1 && trimmed == beginMarker:
			beginIdx = i
		case beginIdx != -1 && trimmed == endMarker:
			endIdx = i
		}
		if beginIdx != -1 && endIdx != -1 {
			break
		}
	}

	// No complete existing block — append after preserving all content.
	if beginIdx == -1 || endIdx == -1 {
		head := string(existing)
		if head != "" && !strings.HasSuffix(head, "\n") {
			head += "\n"
		}
		return []byte(head + block)
	}

	// Splice: everything before the BEGIN line, the new block, everything after
	// the END line — all foreign content preserved byte-for-byte.
	var b strings.Builder
	b.WriteString(strings.Join(lines[:beginIdx], ""))
	b.WriteString(block)
	b.WriteString(strings.Join(lines[endIdx+1:], ""))
	return []byte(b.String())
}
