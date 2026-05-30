#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CASE_NAME="${1:-all}"
LANGUAGE="${2:-all}"
PYTHON_BIN="${PYTHON_BIN:-/tmp/e2b-python-sdk-venv/bin/python}"
RUNNER_TIMEOUT_SEC="${LIVE_PARITY_TIMEOUT_SEC:-1200}"

if [[ -f "$ROOT_DIR/.env" ]]; then
  set -a
  # shellcheck disable=SC1091
  source "$ROOT_DIR/.env"
  set +a
fi

emit_runner_timeout_result() {
  local language="$1"
  cat <<EOF
[
  {
    "language": "$language",
    "case": "$CASE_NAME",
    "status": "env_blocked",
    "detail": "runner wall timeout after ${RUNNER_TIMEOUT_SEC}s",
    "extra": {
      "failure_kind": "runner_timeout",
      "runner_timeout_sec": "${RUNNER_TIMEOUT_SEC}"
    }
  }
]
EOF
}

run_with_timeout() {
  local language="$1"
  shift

  local exit_code=0
  set +e
  timeout --foreground "${RUNNER_TIMEOUT_SEC}s" "$@"
  exit_code=$?
  set -e

  if [[ "$exit_code" -eq 0 ]]; then
    return 0
  fi

  if [[ "$exit_code" -eq 124 || "$exit_code" -eq 137 || "$exit_code" -eq 143 ]]; then
    emit_runner_timeout_result "$language"
    return 0
  fi

  return "$exit_code"
}

run_go() {
  echo "== go =="
  run_with_timeout go bash -lc 'cd "$1" && go run ./scripts/live_parity_crosscheck --case "$2"' bash "$ROOT_DIR" "$CASE_NAME"
}

run_js() {
  echo "== js =="
  run_with_timeout js bash -lc 'cd "$1" && bun ./scripts/live_parity_crosscheck/js.ts --case "$2"' bash "$ROOT_DIR" "$CASE_NAME"
}

run_python() {
  if [[ ! -x "$PYTHON_BIN" ]]; then
    echo "python helper not found at $PYTHON_BIN" >&2
    exit 1
  fi
  echo "== python =="
  run_with_timeout python bash -lc 'cd "$1" && "$2" ./scripts/live_parity_crosscheck/py.py --case "$3"' bash "$ROOT_DIR" "$PYTHON_BIN" "$CASE_NAME"
}

case "$LANGUAGE" in
  all)
    run_go
    run_js
    run_python
    ;;
  go)
    run_go
    ;;
  js)
    run_js
    ;;
  python)
    run_python
    ;;
  *)
    echo "unknown language: $LANGUAGE" >&2
    echo "usage: bash scripts/live_parity_crosscheck.sh [all|claude|claude_derived|randomness|randomness_alias|volume|volume_api_payload|ubuntu|template_timeout|template_methods|config_headers|metrics|network_rules|network_egress|network_update_payload|template_api_payload|debug_root] [all|go|js|python]" >&2
    exit 1
    ;;
esac
