#!/bin/bash
# Nightly database dumps: every day at 03:30 Europe/Warsaw, /backups/librehold-YYYYMMDD.dump (pg_dump custom format);
# dumps older than 7 days are deleted. With --once it dumps right away and exits.
# Connection settings come from PGHOST, PGUSER, PGPASSWORD and PGDATABASE.
set -euo pipefail

export TZ=Europe/Warsaw

dump() {
  local file="/backups/librehold-$(date +%Y%m%d).dump"
  pg_dump -Fc -f "$file.part"
  mv "$file.part" "$file"
  find /backups -name 'librehold-*.dump' -mtime +7 -delete
  echo "$(date -Iseconds) backup: $file"
}

if [[ "${1:-}" == "--once" ]]; then
  dump
  exit 0
fi

while true; do
  now=$(date +%s)
  next=$(date -d 'today 03:30' +%s)
  if (( next <= now )); then
    next=$(date -d 'tomorrow 03:30' +%s)
  fi
  sleep $(( next - now ))
  dump || echo "$(date -Iseconds) backup failed" >&2
done
