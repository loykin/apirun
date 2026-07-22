#!/usr/bin/env bash
# Starts the local mockserver, runs `apirun stages up`, then tears the
# mockserver down. Keeps the stages_embedded example self-contained so it
# no longer depends on httpbin.org being reachable.
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"

ADDR="127.0.0.1:18080"

MOCK_BIN="$(mktemp -t apirun-mockserver.XXXXXX)"
go build -o "$MOCK_BIN" ./mockserver

"$MOCK_BIN" -addr ":18080" &
MOCK_PID=$!
trap 'kill "$MOCK_PID" 2>/dev/null || true; rm -f "$MOCK_BIN"' EXIT

for _ in $(seq 1 50); do
  if curl -sf "http://${ADDR}/json" >/dev/null 2>&1; then
    break
  fi
  sleep 0.2
done

if [ -x ../../apirun ]; then
  ../../apirun stages up "$@"
else
  go run ../../cmd/apirun stages up "$@"
fi
