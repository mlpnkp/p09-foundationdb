#!/usr/bin/env bash
# Technical check: acknowledged writes survive a kill -9 of one process,
# and the cluster accepts writes again after recovery.
#
# Writes N keys through fdbcli (each `set` is acknowledged before the next one),
# kills the given container with SIGKILL, polls until a new write commits,
# then reads every key back.
#
# Usage: scripts/check-kill.sh [container|auto] [N]   (defaults: auto 100)
set -euo pipefail

victim="${1:-auto}"
n="${2:-100}"
here="$(cd "$(dirname "$0")" && pwd)"
compose="$here/compose.sh"
cd "$here/../cluster"

cmds="writemode on;"
for i in $(seq 1 "$n"); do cmds+=" set check/$i v$i;"; done
"$compose" exec -T fdb1 fdbcli --timeout 10 --exec "$cmds" >/dev/null
echo "acknowledged writes: $n"

roles="$("$here/roles.sh")"
if [[ "$victim" == "auto" ]]; then
  for service in fdb2 fdb3 fdb4 fdb5 fdb1; do
    ip="$("$compose" exec -T "$service" hostname -i | tr -d '\r' | awk '{print $1}')"
    line="$(printf '%s\n' "$roles" | awk -v address="$ip:4500" '$1 == address {print; exit}')"
    if [[ " $line " =~ [[:space:]]log[[:space:]] ]]; then
      victim="$service"
      victim_ip="$ip"
      role_line="$line"
      break
    fi
  done
elif [[ "$victim" =~ ^fdb[1-5]$ ]]; then
  victim_ip="$("$compose" exec -T "$victim" hostname -i | tr -d '\r' | awk '{print $1}')"
  role_line="$(printf '%s\n' "$roles" | awk -v address="$victim_ip:4500" '$1 == address {print; exit}')"
else
  echo "container must be fdb1..fdb5 or auto" >&2
  exit 2
fi
if [[ "$victim" == "auto" ]]; then
  echo "no process currently holds the log role" >&2
  exit 1
fi

cli_service=fdb1
if [[ "$victim" == "fdb1" ]]; then
  cli_service=fdb2
fi
cli() { "$compose" exec -T "$cli_service" fdbcli --timeout "${2:-1}" --exec "$1"; }

echo "roles of $victim before kill:"
echo "$role_line"
if [[ ! " $role_line " =~ [[:space:]]log[[:space:]] ]]; then
  echo "$victim does not currently hold the log role" >&2
  exit 1
fi

kill_ms=$(python3 -c 'import time; print(int(time.time()*1000))')
"$compose" kill -s KILL "$victim" >/dev/null
echo "killed $victim"

restart_needed=1
restart_victim() {
  if [[ "$restart_needed" -eq 1 ]]; then
    "$compose" up -d "$victim" >/dev/null || true
  fi
}
trap restart_victim EXIT

attempt=0
committed=0
deadline=$((SECONDS + 60))
while ((SECONDS < deadline)); do
  if cli "writemode on; set check/after-kill 1" 2>/dev/null | grep -q 'Committed'; then
    committed=1
    break
  fi
  attempt=$((attempt + 1))
  sleep 0.2
done
if [[ "$committed" -ne 1 ]]; then
  echo "cluster did not accept a write within 60 seconds after kill" >&2
  exit 1
fi
ok_ms=$(python3 -c 'import time; print(int(time.time()*1000))')
echo "first committed write after kill: $((ok_ms - kill_ms)) ms (failed attempts: $attempt)"

"$compose" up -d "$victim" >/dev/null
"$here/cluster-up.sh" >/dev/null
restart_needed=0

missing=0
for i in $(seq 1 "$n"); do
  if ! cli "get check/$i" 5 | grep -q "is \`v$i'"; then
    echo "MISSING check/$i"
    missing=$((missing + 1))
  fi
done
echo "missing acknowledged writes: $missing of $n"

"$here/roles.sh" | tail -1

if [[ "$missing" -ne 0 ]]; then
  exit 1
fi
