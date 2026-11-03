#!/usr/bin/env bash
# Runs the load generator against the local cluster and prints the summary.
# The journal goes to results/runs/<name>.jsonl.
# Usage: scripts/load.sh <name> [loadgen flags...]
#   scripts/load.sh hot1000 -hot 1000 -duration 60s
set -euo pipefail

name="$1"; shift
root="$(cd "$(dirname "$0")/.." && pwd)"
mkdir -p "$root/results/runs"
cluster_file="$(grep -o '"p09:[^"]*"' "$root/cluster/docker-compose.yml" | tr -d '"')"

docker run --rm --network p09-fdb_fdbnet \
  -v "$root/results/runs:/runs" \
  --entrypoint sh p09-loadgen -c '
    mkdir -p /etc/foundationdb && echo "$0" > /etc/foundationdb/fdb.cluster
    name="$1"; shift
    loadgen -out "/runs/$name.jsonl" "$@" && summary "/runs/$name.jsonl"
  ' "$cluster_file" "$name" "$@"
