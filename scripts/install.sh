#!/usr/bin/env sh
set -eu

version="${AISTAT_VERSION:-v0.1.0}"

case "$(uname -m)" in
  x86_64|amd64) arch=amd64 ;;
  aarch64|arm64) arch=arm64 ;;
  *) echo "Unsupported architecture: $(uname -m)" >&2; exit 2 ;;
esac

base="https://github.com/arpingblue/AIStat/releases/download/${version}"
archive="aistat_${version#v}_linux_${arch}.tar.gz"
tmp_dir="$(mktemp -d)"
trap 'rm -rf -- "$tmp_dir"' EXIT INT TERM

curl --fail --location --proto '=https' --tlsv1.2 "$base/$archive" -o "$tmp_dir/$archive"
curl --fail --location --proto '=https' --tlsv1.2 "$base/checksums.txt" -o "$tmp_dir/checksums.txt"
(cd "$tmp_dir" && grep "  $archive\$" checksums.txt | sha256sum --check --status -)
tar -xzf "$tmp_dir/$archive" -C "$tmp_dir" aistat
install -m 0755 "$tmp_dir/aistat" "${AISTAT_INSTALL_DIR:-/usr/local/bin}/aistat"
echo "Installed aistat ${version}."
