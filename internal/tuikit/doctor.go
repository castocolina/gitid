package tuikit

// doctor.go holds the helpers SHARED by the Health tab (health_screen.go,
// read-only) and the Fixer tab (fixer_screen.go, the fix-ceremony half) —
// 08-01-PLAN.md Task 2's split of the former single Doctor tab (FIX-02,
// 02-REDESIGN-SPEC.md §5) into two real, distinct tabs driven by the same
// findings source. Findings group `SSH · <identity|global>` then `Git · …`,
// severity-ordered, with the LOCKED severity contract (~ info cyan !
// warning yellow ✗ error AND critical red — the word disambiguates, NEVER ✗
// for a warning).

import (
	"sort"
)

// doctorScanMsg completes the brief scanning state.
type doctorScanMsg struct{}

// doctorBatch tracks a Fix-all walk.
type doctorBatch struct {
	queue []string
	total int
}

// severityRank orders findings critical > error > warning > info.
var severityRank = map[HealthSeverity]int{
	SeverityCritical: 0,
	SeverityError:    1,
	SeverityWarning:  2,
	SeverityInfo:     3,
}

// orderedFindings returns the live findings severity-sorted (stable).
func orderedFindings(s DemoState) []DemoFinding {
	out := append([]DemoFinding(nil), s.Findings...)
	sort.SliceStable(out, func(i, j int) bool {
		return severityRank[out[i].Severity] < severityRank[out[j].Severity]
	})
	return out
}

// doctorGroup is one `<Section> · <identity|global>` group.
type doctorGroup struct {
	label    string
	findings []DemoFinding
}

// groupFindings groups the ordered findings SSH-first then Git, one group
// per identity (or "global") in flat selection order.
func groupFindings(ordered []DemoFinding) []doctorGroup {
	var groups []doctorGroup
	for _, section := range []string{"SSH", "Git"} {
		var seen []string
		for _, f := range ordered {
			if f.Section != section {
				continue
			}
			id := f.Identity
			if id == "" {
				id = "global"
			}
			present := false
			for _, existing := range seen {
				if existing == id {
					present = true
				}
			}
			if !present {
				seen = append(seen, id)
			}
		}
		for _, id := range seen {
			group := doctorGroup{label: section + " · " + id}
			for _, f := range ordered {
				fid := f.Identity
				if fid == "" {
					fid = "global"
				}
				if f.Section == section && fid == id {
					group.findings = append(group.findings, f)
				}
			}
			groups = append(groups, group)
		}
	}
	return groups
}

// selectFinding resolves the finding matching id within ordered (falls back
// to the first). Extracted as a free function (was a doctorModel method)
// since both healthModel and fixerModel need it and neither owns the other.
func selectFinding(ordered []DemoFinding, id string) (DemoFinding, int, bool) {
	for i, f := range ordered {
		if f.ID == id {
			return f, i, true
		}
	}
	if len(ordered) > 0 {
		return ordered[0], 0, true
	}
	return DemoFinding{}, -1, false
}

// fixableFindings filters the ordered findings that carry a REAL fix (the
// Fixable bit, set from doctor.Finding.Fix != nil) -- NOT SuggestedFix
// non-emptiness, which many report-only findings also carry as advisory
// "do this by hand" prose (08-08 code review CR-01).
func fixableFindings(ordered []DemoFinding) []DemoFinding {
	var out []DemoFinding
	for _, f := range ordered {
		if f.Fixable {
			out = append(out, f)
		}
	}
	return out
}

// pluralS returns "s" for counts other than 1.
func pluralS(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
