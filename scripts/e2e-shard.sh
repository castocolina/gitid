#!/bin/sh
# e2e-shard.sh: runs a fixed slice of the e2e package's top-level Test
# functions, selected by round-robin index. Six consecutive v0.1.0-rc.1..6
# CI-only test-e2e failures on GitHub Actions' shared runners (never
# reproducing locally, root-causes documented in the git history of
# e2e/ui_pty_e2e_test.go) were most consistent with cumulative resource
# contention building up over one long-running, single-process ~950s
# -race suite rather than any one test's own defect. Splitting the same
# suite across several shorter, parallel CI jobs is the mitigation the
# user chose after exhausting per-assertion timing fixes.
#
# Usage: e2e-shard.sh <shard-index-1-based> <shard-count> [go test flags...]
# Example: e2e-shard.sh 2 4 -timeout 900s
set -eu

shard_index="$1"
shard_count="$2"
shift 2

if [ "$shard_index" -lt 1 ] || [ "$shard_index" -gt "$shard_count" ]; then
	echo "e2e-shard.sh: shard index $shard_index out of range [1,$shard_count]" >&2
	exit 1
fi

# `go test -list '.*'` prints one matching function name per line, plus a
# trailing "ok  	<pkg>	<duration>" summary line this strips.
all_tests=$(go test -tags e2e -list '.*' ./e2e/... | grep -v '^ok')

pattern=$(printf '%s\n' "$all_tests" | awk -v idx="$shard_index" -v cnt="$shard_count" '
	(NR % cnt) == (idx % cnt) {
		printf "%s%s$", (n > 0 ? "|" : ""), $0
		n++
	}
	END { if (n == 0) { print "^$" } }
')

test_count=$(printf '%s\n' "$all_tests" | wc -l | tr -d ' ')
echo "e2e-shard.sh: shard $shard_index/$shard_count of $test_count top-level tests"

exec go test -tags e2e -race -run "^($pattern)$" "$@" ./e2e/...
