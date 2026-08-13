#!/usr/bin/env sh
set -eu

project_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
work=$(mktemp -d)
trap 'rm -rf -- "$work"' EXIT INT TERM

payload="$work/payload"
fake_bin="$work/bin"
install_dir="$work/install"
mkdir -p "$payload" "$fake_bin" "$install_dir"

(cd "$project_root" && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -o "$payload/aistat" ./cmd/aistat)
archive="$payload/aistat_0.1.0_linux_amd64.tar.gz"
tar -czf "$archive" -C "$payload" aistat
(cd "$payload" && sha256sum "$(basename "$archive")" > checksums.txt)

cat > "$fake_bin/curl" <<'EOF'
#!/usr/bin/env sh
set -eu
destination=
while [ "$#" -gt 0 ]; do
  if [ "$1" = "-o" ]; then
    shift
    destination=$1
  fi
  shift
done
test -n "$destination"
cp "$AISTAT_INSTALL_TEST_PAYLOAD/$(basename "$destination")" "$destination"
EOF
chmod +x "$fake_bin/curl"

PATH="$fake_bin:$PATH" \
  AISTAT_INSTALL_TEST_PAYLOAD="$payload" \
  AISTAT_VERSION=v0.1.0 \
  AISTAT_INSTALL_DIR="$install_dir" \
  sh "$project_root/scripts/install.sh"

test -x "$install_dir/aistat"
"$install_dir/aistat" version >/dev/null
