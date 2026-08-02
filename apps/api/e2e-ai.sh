#!/usr/bin/env bash
set -euo pipefail

# Ask Docs / vector search / assistant chat were removed from the product.
# Keep this wrapper so CI/docs that invoke e2e-ai.sh do not fail unexpectedly.
# P0 coverage lives in ./e2e-test.sh (Ask Host remains covered elsewhere).

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

echo "=== DealSignal e2e-ai.sh ==="
echo "SKIP: Ask Docs AI E2E removed (no /search or /assistant/chat)."
echo "Run ./e2e-test.sh for P0 backend verification."
exit 0
