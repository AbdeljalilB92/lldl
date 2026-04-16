#!/usr/bin/env bash
set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

PASS=0
FAIL=0

run_check() {
  local name="$1"
  shift
  printf "${YELLOW}[CHECK]${NC} %s ... " "$name"
  if output=$("$@" 2>&1); then
    printf "${GREEN}PASS${NC}\n"
    PASS=$((PASS + 1))
  else
    printf "${RED}FAIL${NC}\n"
    echo "$output"
    FAIL=$((FAIL + 1))
  fi
}

run_check_silent() {
  local name="$1"
  shift
  printf "${YELLOW}[CHECK]${NC} %s ... " "$name"
  if "$@" >/dev/null 2>&1; then
    printf "${GREEN}PASS${NC}\n"
    PASS=$((PASS + 1))
  else
    printf "${RED}FAIL${NC}\n"
    # Re-run to show output
    "$@" 2>&1 || true
    FAIL=$((FAIL + 1))
  fi
}

cd "$(dirname "$0")/.."

echo ""
echo "=============================="
echo "  llcd Quality Gate"
echo "=============================="
echo ""

# 1. Format check — no unformatted files
run_check_silent "gofmt"       gofmt -l .

# 2. Static analysis
run_check       "go vet"       go vet ./...

# 3. Linter
run_check       "golangci-lint" ~/go/bin/golangci-lint run ./...

# 4. Build
run_check_silent "go build"    go build -o llcd .

# 5. Tests
run_check       "go test"      go test ./...

echo ""
echo "=============================="
if [ "$FAIL" -eq 0 ]; then
  printf "${GREEN}All %d checks passed.${NC}\n" "$PASS"
else
  printf "${RED}%d check(s) failed, %d passed.${NC}\n" "$FAIL" "$PASS"
  exit 1
fi
echo "=============================="
