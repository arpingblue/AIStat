#!/usr/bin/env sh
set -eu

if [ "$#" -ne 2 ]; then
  echo "usage: sanitize-fixture.sh INPUT_DIR OUTPUT_DIR" >&2
  exit 2
fi

input=$1
output=$2
if [ ! -d "$input" ]; then
  echo "input is not a directory: $input" >&2
  exit 2
fi
if [ -e "$output" ]; then
  echo "output already exists: $output" >&2
  exit 2
fi

cp -R -- "$input" "$output"
find "$output" -type f -exec sed -i \
  -e 's/\([0-9A-Fa-f][0-9A-Fa-f]:\)\{5\}[0-9A-Fa-f][0-9A-Fa-f]/00:00:00:00:00:00/g' \
  -e 's/\b\([0-9]\{1,3\}\.\)\{3\}[0-9]\{1,3\}\b/192.0.2.1/g' \
  -e 's/\(token\|password\|secret\|api[_-]*key\)=[^ ,;]*/\1=[REDACTED]/Ig' {} +

echo "Sanitized copy created at $output. Manual privacy review is still required."
