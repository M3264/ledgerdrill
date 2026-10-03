#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
TASK_GO=${LEDGERDRILL_GO:-go}
if ! command -v "$TASK_GO" >/dev/null 2>&1; then
  if [[ -x /home/ubuntu/.local/lib/thinkpit/go/bin/go ]]; then TASK_GO=/home/ubuntu/.local/lib/thinkpit/go/bin/go; else echo 'Install Go 1.27+ or set LEDGERDRILL_GO to its executable.' >&2; exit 1; fi
fi
npm --prefix frontend ci --no-audit --no-fund
npm --prefix frontend run build
"$TASK_GO" build -p 1 -o ledgerdrill ./cmd/ledgerdrill
exec ./ledgerdrill "$@"
