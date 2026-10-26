#!/usr/bin/env bash
set -euo pipefail
umask 077
: "${DATABASE_URL:?DATABASE_URL is required}"
: "${BACKUP_AGE_RECIPIENT:?public age recipient is required}"
: "${BACKUP_OUTPUT:?encrypted backup output path is required}"
work=$(mktemp -d)
container="catalogo-restore-$RANDOM-$RANDOM"
cleanup() { docker rm -f "$container" >/dev/null 2>&1 || true; rm -rf "$work"; }
trap cleanup EXIT
pg_dump --format=custom --no-owner --no-acl --schema=catalog --schema=catalog_migrations --file="$work/catalogo.dump" "$DATABASE_URL"
pg_restore --list "$work/catalogo.dump" >/dev/null
# The restore target is always an isolated disposable container, never DATABASE_URL.
docker run -d --name "$container" -e POSTGRES_HOST_AUTH_METHOD=trust postgres:17 >/dev/null
for attempt in $(seq 1 30); do
  if docker exec "$container" pg_isready -U postgres >/dev/null 2>&1; then break; fi
  sleep 1
done
docker exec "$container" psql -v ON_ERROR_STOP=1 -U postgres -c 'create role anon; create role authenticated;' >/dev/null
docker exec -i "$container" pg_restore --exit-on-error --no-owner --no-acl -U postgres -d postgres < "$work/catalogo.dump"
# Do not log personal records or restore output.
docker exec "$container" psql -v ON_ERROR_STOP=1 -U postgres -Atc "select 1 from catalog.books limit 0" >/dev/null
age --recipient "$BACKUP_AGE_RECIPIENT" --output "$BACKUP_OUTPUT" "$work/catalogo.dump"
test -s "$BACKUP_OUTPUT"
printf '%s\n' 'Backup encrypted and successfully restored in an isolated PostgreSQL container.'
