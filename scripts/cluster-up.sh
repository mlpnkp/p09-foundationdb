#!/usr/bin/env bash
# Starts the five-process cluster and configures a new `double ssd` database.
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
compose="$here/compose.sh"
cd "$here/../cluster"

"$compose" up -d

configured=0
for _ in $(seq 1 30); do
  if "$compose" exec -T fdb1 fdbcli --exec 'status minimal' --timeout 3 2>/dev/null | grep -q 'available'; then
    configured=1
    break
  fi
  if "$compose" exec -T fdb1 fdbcli --exec 'configure new double ssd' --timeout 10 2>/dev/null | grep -q 'Database created'; then
    configured=1
    break
  fi
  sleep 2
done

if [[ "$configured" -ne 1 ]]; then
  echo "cluster could not be configured" >&2
  exit 1
fi

for _ in $(seq 1 60); do
  status_json="$("$compose" exec -T fdb1 fdbcli --exec 'status json' --timeout 10 2>/dev/null || true)"
  if printf '%s' "$status_json" | python3 -c '
import json, sys
try:
    status = json.load(sys.stdin)
except (KeyError, TypeError, ValueError):
    raise SystemExit(1)
cluster = status["cluster"]
processes = list(cluster["processes"].values())
zones = {p.get("locality", {}).get("zoneid") for p in processes}
config = cluster.get("configuration", {})
coordinators = status.get("client", {}).get("coordinators", {}).get("coordinators", [])
coord_count = config.get("coordinators_count", len(coordinators))
ok = (
    len(processes) == 5
    and len(zones) == 5
    and config.get("redundancy_mode") == "double"
    and coord_count == 3
    and cluster.get("recovery_state", {}).get("name") == "fully_recovered"
)
raise SystemExit(0 if ok else 1)
'; then
    "$compose" exec -T fdb1 fdbcli --exec 'status' --timeout 10
    exit 0
  fi
  sleep 2
done

echo "cluster did not reach the expected fully recovered five-process configuration" >&2
"$here/roles.sh" >&2 || true
exit 1
