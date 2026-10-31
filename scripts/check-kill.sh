#!/usr/bin/env bash
# Technical check: acknowledged writes survive a kill -9 of one process,
# and the cluster accepts writes again after recovery.
#
# Writes N keys through fdbcli (each `set` is acknowledged before the next one),
# kills the given container with SIGKILL, polls until a new write commits,
# then reads every key back.
#
# Usage: scripts/check-kill.sh [container] [N]   (defaults: fdb5 100)
set -euo pipefail

victim="${1:-fdb5}"
n="${2:-100}"
here="$(cd "$(dirname "$0")" && pwd)"
cd "$here/../cluster"

cli() { docker compose exec -T fdb1 fdbcli --timeout 1 --exec "$1"; }

cmds="writemode on;"
for i in $(seq 1 "$n"); do cmds+=" set check/$i v$i;"; done
cli "$cmds" >/dev/null
echo "acknowledged writes: $n"

echo "roles of $victim before kill:"
"$here/roles.sh" | grep "$(docker compose exec -T "$victim" hostname -i | tr -d '\r')" || true

kill_ms=$(python3 -c 'import time; print(int(time.time()*1000))')
docker compose kill -s KILL "$victim" >/dev/null
echo "killed $victim"

attempt=0
until cli "writemode on; set check/after-kill 1" 2>/dev/null | grep -q 'Committed'; do
  attempt=$((attempt + 1))
  sleep 0.2
done
ok_ms=$(python3 -c 'import time; print(int(time.time()*1000))')
echo "first committed write after kill: $((ok_ms - kill_ms)) ms (failed attempts: $attempt)"

missing=0
for i in $(seq 1 "$n"); do
  if ! cli "get check/$i" | grep -q "is \`v$i'"; then
    echo "MISSING check/$i"
    missing=$((missing + 1))
  fi
done
echo "missing acknowledged writes: $missing of $n"

"$here/roles.sh" | tail -1
