#!/bin/sh
# Packs every template (templates/<language>) into the archive that `steward plugin init`
# downloads from an SDK release, with the checksum it verifies:
#
#   ./pack-templates.sh <version> <out-dir>
#
# The archive holds a VERSION file and one folder per language under templates/. Attach
# templates.tar.gz and templates.tar.gz.sha256 to the release.

set -eu

version="${1:?usage: pack-templates.sh <version> <out-dir>}"
out="${2:?usage: pack-templates.sh <version> <out-dir>}"
root=$(cd "$(dirname "$0")" && pwd)
stage=$(mktemp -d)

trap 'rm -rf "$stage"' EXIT

mkdir -p "$stage/templates" "$out"

for folder in "$root"/templates/*/; do
  [ -d "$folder" ] || continue

  cp -R "$folder" "$stage/templates/$(basename "$folder")"
done

printf '%s\n' "$version" > "$stage/VERSION"

COPYFILE_DISABLE=1 tar -czf "$out/templates.tar.gz" -C "$stage" VERSION templates

(cd "$out" && shasum -a 256 templates.tar.gz > templates.tar.gz.sha256)

echo "packed $out/templates.tar.gz ($version)"
