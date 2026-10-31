#!/usr/bin/env bash
# Stops the cluster and deletes its data, so the next cluster-up starts from scratch.
# Usage: scripts/cluster-down.sh
set -euo pipefail

cd "$(dirname "$0")/../cluster"
docker compose down -v
