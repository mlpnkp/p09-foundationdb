#!/usr/bin/env bash
# Starts the five-process cluster and configures a new `double ssd` database.
# Usage: scripts/cluster-up.sh
set -euo pipefail

cd "$(dirname "$0")/../cluster"

docker compose up -d

for _ in $(seq 1 30); do
  if docker compose exec -T fdb1 fdbcli --exec 'status minimal' --timeout 3 2>/dev/null | grep -q 'available'; then
    echo "database is already configured"
    exit 0
  fi
  if docker compose exec -T fdb1 fdbcli --exec 'configure new double ssd' --timeout 10 2>/dev/null | grep -q 'Database created'; then
    break
  fi
  sleep 2
done

for _ in $(seq 1 30); do
  if docker compose exec -T fdb1 fdbcli --exec 'status minimal' --timeout 3 | grep -q 'available'; then
    docker compose exec -T fdb1 fdbcli --exec 'status' --timeout 10
    exit 0
  fi
  sleep 2
done

echo "cluster did not become available" >&2
exit 1
