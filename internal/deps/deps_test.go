package deps

import (
	"testing"
)

func TestMissingRequired(t *testing.T) {
	tests := []struct {
		name   string
		report Report
		want   []string
	}{
		{
			name:   "all required present reports nothing",
			report: Report{SSH: true, SSHKeygen: true, Git: true},
			want:   nil,
		},
		{
			name:   "missing ssh is reported",
			report: Report{SSH: false, SSHKeygen: true, Git: true},
			want:   []string{"ssh"},
		},
		{
			name:   "missing ssh-keygen is reported",
			report: Report{SSH: true, SSHKeygen: false, Git: true},
			want:   []string{"ssh-keygen"},
		},
		{
			name:   "missing git is reported",
			report: Report{SSH: true, SSHKeygen: true, Git: false},
			want:   []string{"git"},
		},
		{
			name:   "all required missing reports all three in order",
			report: Report{SSH: false, SSHKeygen: false, Git: false},
			want:   []string{"ssh", "ssh-keygen", "git"},
		},
		{
			name:   "optional tools never appear in MissingRequired",
			report: Report{SSH: true, SSHKeygen: true, Git: true, SSHAdd: false, Clipboard: false},
			want:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.report.MissingRequired()
			if len(got) != len(tt.want) {
				t.Fatalf("MissingRequired() = %v, want %v", got, tt.want)
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Fatalf("MissingRequired()[%d] = %q, want %q (full: %v)", i, got[i], tt.want[i], got)
				}
			}
		})
	}
}

// TestDetectSmoke confirms Detect() finds the required toolchain in this dev
// environment. ssh and git are guaranteed present here (the repo uses them);
// ssh-keygen ships with ssh. This asserts the LookPath wiring works without
// pinning the host's full PATH.
func TestDetectSmoke(t *testing.T) {
	report := Detect()
	if !report.SSH {
		t.Error("Detect(): expected ssh to be found in this environment")
	}
	if !report.Git {
		t.Error("Detect(): expected git to be found in this environment")
	}
	if missing := report.MissingRequired(); len(missing) != 0 {
		t.Errorf("Detect(): expected no missing required tools in this env, got %v", missing)
	}
}
