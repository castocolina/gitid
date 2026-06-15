package filewriter

import (
	"bytes"
	"strings"
)

// NamedBlock is one sentinel-delimited block extracted from a file.
type NamedBlock struct {
	Name string // the <name> token from "# BEGIN gitid managed: <name>"
	Body string // lines between (exclusive of) the sentinel markers, as written
}

// ListBlocks scans content for all complete gitid managed blocks and returns
// them in file order. Incomplete blocks (BEGIN with no matching END) are
// silently skipped. CRLF is normalised to LF before scanning so Windows-synced
// configs parse correctly.
func ListBlocks(content []byte) []NamedBlock {
	// Normalise CRLF → LF so Windows-synced configs parse correctly.
	normalised := bytes.ReplaceAll(content, []byte("\r\n"), []byte("\n"))
	lines := strings.SplitAfter(string(normalised), "\n")

	var result []NamedBlock
	beginIdx := -1
	currentName := ""
	for i, line := range lines {
		trimmed := strings.TrimRight(line, "\n\r")
		if strings.HasPrefix(trimmed, BeginPrefix) {
			// Ignore nested or duplicate begins — only the outermost matters.
			if beginIdx == -1 {
				beginIdx = i
				currentName = strings.TrimPrefix(trimmed, BeginPrefix)
			}
			continue
		}
		if beginIdx != -1 && strings.HasPrefix(trimmed, EndPrefix) {
			endName := strings.TrimPrefix(trimmed, EndPrefix)
			if endName == currentName {
				// Collect body lines between markers (exclusive).
				body := strings.Join(lines[beginIdx+1:i], "")
				body = strings.TrimRight(body, "\n")
				result = append(result, NamedBlock{Name: currentName, Body: body})
				beginIdx = -1
				currentName = ""
			}
			// Mismatched END name: skip silently (orphan sentinel; doctor handles).
		}
	}
	return result
}

// RemoveBlock returns content with the gitid managed block for name removed.
// If no such block exists the input is returned unchanged (idempotent). A single
// blank line immediately following the END marker is also consumed to avoid
// blank-line accumulation on repeated delete+recreate cycles.
func RemoveBlock(content []byte, name string) []byte {
	beginMarker := BeginPrefix + name
	endMarker := EndPrefix + name

	lines := strings.SplitAfter(string(content), "\n")

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

	// Block absent — return input unchanged (idempotent).
	if beginIdx == -1 || endIdx == -1 {
		return content
	}

	// Determine the slice boundary after the END line.
	afterEnd := endIdx + 1
	// Consume one trailing blank line to avoid accumulating blank lines after
	// repeated delete+recreate. Only remove it if it is genuinely empty.
	if afterEnd < len(lines) {
		if strings.TrimRight(lines[afterEnd], "\n\r") == "" {
			afterEnd++
		}
	}

	var b strings.Builder
	b.WriteString(strings.Join(lines[:beginIdx], ""))
	b.WriteString(strings.Join(lines[afterEnd:], ""))
	return []byte(b.String())
}

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
