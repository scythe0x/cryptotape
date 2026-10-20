#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

printf 'revision: %s\n' "$(git rev-parse --short HEAD)"
if [[ -n "$(git status --porcelain)" ]]; then
  printf 'worktree: dirty\n'
else
  printf 'worktree: clean\n'
fi
go version

go test ./internal/core ./internal/binance ./internal/storage ./internal/desktop \
  -run '^$' -bench 'Benchmark(ProcessMarkets|CombinedBookDecode|PersistAlertBatch|AlertPipelineBurst)$' \
  -benchmem -benchtime=300ms -count=3 "$@"

go test ./internal/desktop -run '^TestQueuedCrashOpens750MarketsWithSlowResponses$' -count=1 -v
