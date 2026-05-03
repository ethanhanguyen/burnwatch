#!/bin/bash
set -euo pipefail

BASE="${1:-main}"
RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m'

PASS=0
FAIL=0

pass() { echo -e "  ${GREEN}PASS${NC} $*"; PASS=$((PASS + 1)); }
fail() { echo -e "  ${RED}FAIL${NC} $*"; FAIL=$((FAIL + 1)); }

DIFF_FILES=""
if git rev-parse --verify "$BASE" >/dev/null 2>&1; then
	DIFF_FILES=$(git diff --name-only "$BASE"...HEAD 2>/dev/null || true)
fi

changed_go() {
	if [ -z "$DIFF_FILES" ]; then
		return 0
	fi
	echo "$DIFF_FILES" | grep '\.go$' || true
}

changed_non_test_go() {
	changed_go | grep -v '_test\.go$' || true
}

changed_test_go() {
	changed_go | grep '_test\.go$' || true
}

echo "=== Phase 0: Automated Checks ==="
echo ""

check_go_vet() {
	local out
	if out=$(go vet ./... 2>&1); then
		pass "go vet ./..."
	else
		fail "go vet ./... — vet failures:$out"
	fi
}

check_golangci_lint() {
	local out
	if out=$(golangci-lint run 2>&1); then
		pass "golangci-lint run"
	else
		fail "golangci-lint run — lint warnings:$out"
	fi
}

check_go_build() {
	local out
	if out=$(go build -o burnwatch . 2>&1); then
		pass "go build -o burnwatch ."
	else
		fail "go build -o burnwatch . — build errors:$out"
	fi
}

check_go_test_coverage() {
	local out
	if out=$(go test -cover ./... 2>&1); then
		local low=""
		local min=100
		while IFS= read -r line; do
			if [[ "$line" =~ coverage:\ ([0-9.]+)% ]]; then
				pct=${BASH_REMATCH[1]}
				pct_int=${pct%.*}
				if [[ "$line" =~ ^ok ]]; then
					pkg=$(echo "$line" | awk '{print $2}')
				else
					pkg=$(echo "$line" | awk '{print $1}')
				fi
				if [[ "$pkg" == *"/cmd" ]] || [[ "$pkg" == "$(go list -m)" ]]; then
					continue
				fi
				if [ "$pct_int" -lt "$min" ]; then
					min=$pct_int
				fi
				if [ "$pct_int" -lt 80 ]; then
					low="$low  $pkg: ${pct}%"
				fi
			fi
		done <<< "$out"
		if [ -n "$low" ]; then
			fail "go test -cover — packages below 80%:$low"
		else
			pass "go test -cover — all non-cmd packages >=80% (min: ${min}%)"
		fi
	else
		fail "go test -cover — test failures"
	fi
}

check_no_interface_or_mapany() {
	local hits
	hits=$(changed_go | xargs -I{} git diff "$BASE"...HEAD -- '{}' 2>/dev/null | grep -n '^\+\s*.*interface{}' || true)
	local hits2
	hits2=$(changed_go | xargs -I{} git diff "$BASE"...HEAD -- '{}' 2>/dev/null | grep -n '^\+\s*.*map\[string\]any' || true)
	if [ -z "$hits" ] && [ -z "$hits2" ]; then
		pass "no interface{} or map[string]any in diff"
	else
		fail "interface{} or map[string]any found — forbidden pattern:$(echo "$hits$hits2")"
	fi
}

check_no_panic_non_test() {
	local hits
	hits=$(changed_non_test_go | xargs -I{} git diff "$BASE"...HEAD -- '{}' 2>/dev/null | grep -n '^\+\s*.*panic(' || true)
	if [ -z "$hits" ]; then
		pass "no panic() in non-test Go files"
	else
		fail "panic() in non-test Go files — use t.Fatalf/t.Errorf:$(echo "$hits")"
	fi
}

check_testdata_paths() {
	local bad
	bad=""
	while IFS= read -r f; do
		[ -z "$f" ] && continue
		local refs
		refs=$(git diff "$BASE"...HEAD -- "$f" 2>/dev/null | grep -n '^\+\s*.*testdata' | grep -v 'filepath.Join.*"\.\.".*"testdata"' || true)
		if [ -n "$refs" ]; then
			bad="$bad  $f: testdata path not using filepath.Join(\"..\", \"testdata\", ...):$refs"
		fi
	done < <(changed_test_go)
	if [ -z "$bad" ]; then
		pass "testdata paths use filepath.Join(\"..\", \"testdata\", ...)"
	else
		fail "testdata paths must use filepath.Join(\"..\", \"testdata\", ...):$bad"
	fi
}

check_no_new_config_files() {
	local hits
	hits=$(echo "$DIFF_FILES" | grep -E '\.(env|ya?ml|toml|json)$' | grep -v '.golangci.yml' | grep -v '.goreleaser.yml' | grep -v 'go.sum' | grep -v 'testdata/' | grep -v '.github/' || true)
	if [ -z "$hits" ]; then
		pass "no new config files introduced"
	else
		fail "new config files detected — use env vars + CLI flags only:$(echo "$hits")"
	fi
}

check_no_concurrency_non_test() {
	local hits
	hits=$(changed_non_test_go | xargs -I{} git diff "$BASE"...HEAD -- '{}' 2>/dev/null | grep -nE '^\+\s*.*(sync\.Mutex|sync\.RWMutex)' || true)
	if [ -z "$hits" ]; then
		pass "no sync.Mutex / sync.RWMutex in non-test Go files"
	else
		fail "sync.Mutex / sync.RWMutex in non-test Go files — use single-goroutine pattern:$(echo "$hits")"
	fi
}

echo "--- Build & Lint ---"
check_go_vet
check_golangci_lint
check_go_build

echo ""
echo "--- Tests ---"
check_go_test_coverage

if [ -n "$DIFF_FILES" ]; then
	echo ""
	echo "--- Diff vs $BASE ---"
	check_no_interface_or_mapany
	check_no_panic_non_test
	check_testdata_paths
	check_no_new_config_files
	check_no_concurrency_non_test
else
	echo ""
	echo "--- Diff vs $BASE ---"
	pass "no staged changes (pushing existing commits or on main)"
fi

echo ""
echo "=== Result: $PASS passed, $FAIL failed ==="

if [ "$FAIL" -gt 0 ]; then
	exit 1
fi
exit 0
