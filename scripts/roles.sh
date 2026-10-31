#!/usr/bin/env bash
# Prints which roles every fdbserver process holds: "address roles...".
# Usage: scripts/roles.sh
set -euo pipefail

cd "$(dirname "$0")/../cluster"
docker compose exec -T fdb1 fdbcli --exec 'status json' --timeout 10 |
  python3 -c '
import json, sys
status = json.load(sys.stdin)
for proc in sorted(status["cluster"]["processes"].values(), key=lambda p: p["address"]):
    roles = sorted(r["role"] for r in proc.get("roles", []))
    print(proc["address"], proc["locality"].get("zoneid", "?"), " ".join(roles))
print("recovery_state:", status["cluster"].get("recovery_state", {}).get("name"))
'
