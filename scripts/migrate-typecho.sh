#!/usr/bin/env bash
# One-shot Typecho migration wrapper.
#
# Examples:
#   TYPECHO_EXPORT=typecho.json TYPECHO_UPLOADS=/backup/usr/uploads \
#     ./scripts/migrate-typecho.sh
#
#   TYPECHO_BUNDLE=typecho-bundle.zip ./scripts/migrate-typecho.sh
#
#   TYPECHO_ROOT=/www/blog TYPECHO_UPLOADS=/www/blog/usr/uploads \
#     ./scripts/migrate-typecho.sh --dry-run
#
#   ./scripts/migrate-typecho.sh --skip-import   # only repair links + audit
#
# Links inside imported content are rewritten to the new routes. To preserve
# old Typecho URLs, enable the typecho-redirects plugin in config.yaml.

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

CONFIG="${PIONUS_CONFIG:-config.yaml}"
EXPORT="${TYPECHO_EXPORT:-}"
BUNDLE="${TYPECHO_BUNDLE:-}"
UPLOADS="${TYPECHO_UPLOADS:-}"
ROOT_TYPECHO="${TYPECHO_ROOT:-}"

if [[ ! -f "$CONFIG" ]]; then
  echo "config not found: $CONFIG (set PIONUS_CONFIG)" >&2
  exit 1
fi

if [[ -z "$BUNDLE" && -z "$EXPORT" && -n "$ROOT_TYPECHO" ]]; then
  if ! command -v php >/dev/null 2>&1; then
    echo "php is required to export from TYPECHO_ROOT=$ROOT_TYPECHO" >&2
    exit 1
  fi
  EXPORT="$ROOT/typecho.json"
  echo "Exporting Typecho data from $ROOT_TYPECHO -> $EXPORT" >&2
  TYPECHO_ROOT="$ROOT_TYPECHO" php tools/typecho-export.php > "$EXPORT"
fi

ARGS=(-config "$CONFIG")
if [[ -n "$BUNDLE" ]]; then
  ARGS+=(-bundle "$BUNDLE")
fi
if [[ -n "$EXPORT" ]]; then
  ARGS+=(-export "$EXPORT")
fi
if [[ -n "$UPLOADS" ]]; then
  ARGS+=(-source-uploads "$UPLOADS")
fi

echo "Running: go run ./cmd/pionus-migrate ${ARGS[*]} $*" >&2
go run ./cmd/pionus-migrate "${ARGS[@]}" "$@"

cat >&2 <<'EOF'

Migration pass finished.

Next steps:
  1. Review the JSON report above, especially remaining_old_links and
     missing images.
  2. Reset the imported owner account:
       PIONUS_ADMIN_PASSWORD='new-password' ./pionus -config <config> -create-admin <username>
  3. Start the server and spot-check new URLs under /article, /category, /tag.
     Enable plugins.enabled: [typecho-redirects] to 301 common old Typecho URLs.
EOF
