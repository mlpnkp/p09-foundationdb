#!/usr/bin/env bash
# Stops the cluster and deletes its data, so the next cluster-up starts from scratch.
# Usage: scripts/cluster-down.sh
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
cd "$here/../cluster"
"$here/compose.sh" down -v
