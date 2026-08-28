// derive-closure-table.go — the written, re-runnable derivation behind the
// closure table in MANIFEST.md (07-06 Task 2).
//
// The closure table indexes every keyed bullet recorded under the `## Deviations`
// and `## Review` headings of the phase's five prior SUMMARY files (07-01 … 07-05).
// Those two headings are the only sections a plan in this phase is required to
// emit — the plan's own authority block (07-06-PLAN.md) makes a missing heading
// a defect in the source SUMMARY, never a case for the parser to guess around.
// This parser therefore FAILS CLOSED: any of the ten headings that is absent, or
// any section that yields zero list items, aborts with a non-zero exit so an
// under-count can never be mistaken for a clean derivation.
//
// Run (from anywhere):
//
//	go run .planning/phases/07-global-git-options/review-packet/derive-closure-table.go
//
// or with an explicit phase directory:
//
//	go run <path>/derive-closure-table.go <phase-dir>
//
// A "list item" is a TOP-LEVEL markdown list line — `- `, `* `, or a numbered
// `N. ` form with zero leading whitespace (the Deviations sections use numbered
// items). Indented sub-bullets and prose paragraphs belong to their parent item
// and are NOT counted separately, so a nested fix-list cannot inflate the row
// count. Each item's printed "key" is its first word (emphasis and trailing
// punctuation stripped) so a reviewer can map the closure table's rows straight
// back to the source bullets.
package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const wantHeadings = 2

var (
	listItemRE = regexp.MustCompile(`^(?:[-*]|\d+\.)\s+(.*)$`)
	headingRE  = regexp.MustCompile(`^##\s+(.+)$`)
)

// section is one extracted `## <name>` block of a summary.
type section struct {
	name string
	// items are the opening lines of every list item in the section, in order.
	items []string
}

func main() {
	dir := "."
	if len(os.Args) > 1 {
		dir = os.Args[1]
	}
	summaries, err := filepath.Glob(filepath.Join(dir, "07-0[1-5]-SUMMARY.md"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "glob: %v\n", err)
		os.Exit(1)
	}
	sort.Strings(summaries)
	if len(summaries) != 5 {
		fmt.Fprintf(os.Stderr, "expected exactly 5 summary files (07-01..07-05), found %d\n", len(summaries))
		os.Exit(1)
	}

	// perSummary["07-01"]["Deviations"] = *section
	perSummary := map[string]map[string]*section{}
	totalRows := 0
	missing := []string{}

	for _, path := range summaries {
		base := filepath.Base(path) // "07-0N-SUMMARY.md"
		id := base[:5]              // "07-0N"
		sections, err := parseSections(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", base, err)
			os.Exit(1)
		}
		perSummary[id] = sections
		for _, want := range []string{"Deviations", "Review"} {
			sec, ok := sections[want]
			if !ok {
				missing = append(missing, fmt.Sprintf("%s:## %s", id, want))
				continue
			}
			totalRows += len(sec.items)
		}
	}

	fmt.Println("closure-table derivation (07-06 Task 2)")
	fmt.Println("source: 07-01..07-05 SUMMARY '## Deviations' + '## Review' sections")
	fmt.Println("method: for each heading, count every markdown list item (keyed bullets /")
	fmt.Println("        numbered items) under it, up to the next '## ' heading; a section")
	fmt.Println("        with no list items, or a missing heading, aborts the derivation.")
	fmt.Println()

	if len(missing) > 0 {
		for _, m := range missing {
			fmt.Printf("MISSING heading: %s\n", m)
		}
		fmt.Printf("\nFAIL: %d of %d required headings not found — a summary is defective at its source; fix the SUMMARY, not this parser.\n",
			len(missing), len(perSummary)*wantHeadings)
		os.Exit(1)
	}

	for _, id := range []string{"07-01", "07-02", "07-03", "07-04", "07-05"} {
		fmt.Printf("== %s ==\n", id)
		for _, want := range []string{"Deviations", "Review"} {
			sec := perSummary[id][want]
			fmt.Printf("  ## %s (%d items)\n", want, len(sec.items))
			for _, item := range sec.items {
				fmt.Printf("    - %s\n", key(item))
			}
			if len(sec.items) == 0 {
				fmt.Fprintf(os.Stderr, "%s: ## %s contains zero list items — the section exists but carries no keyed rows\n", id, want)
				os.Exit(1)
			}
		}
	}

	fmt.Println()
	fmt.Printf("TOTAL closure-table rows: %d (10 headings across 5 summaries, all present)\n", totalRows)
}

// parseSections splits one SUMMARY file at every `## ` heading and returns the
// sections named exactly "Deviations" and "Review" (only those two are the
// closure table's contract), plus any other section as a terminator.
func parseSections(path string) (map[string]*section, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	out := map[string]*section{}
	var cur *section
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if m := headingRE.FindStringSubmatch(line); m != nil {
			name := m[1]
			if name == "Deviations" || name == "Review" {
				cur = &section{name: name}
				out[name] = cur
			} else {
				cur = nil
			}
			continue
		}
		if cur == nil {
			continue
		}
		if m := listItemRE.FindStringSubmatch(line); m != nil {
			cur.items = append(cur.items, strings.TrimSpace(m[1]))
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// key returns the item's first word — the stable identifier the closure table
// rows are keyed on (`R-1`, `D-07-05-1`, `T-07-04-CUE`, …). Emphasis markers,
// backticks, trailing colons, and trailing punctuation are stripped.
func key(item string) string {
	words := strings.Fields(item)
	if len(words) == 0 {
		return ""
	}
	return strings.Trim(words[0], "*`_:")
}
