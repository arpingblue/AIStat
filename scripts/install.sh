#!/usr/bin/env sh
set -eu

version="${AISTAT_VERSION:-}"

case "$(uname -m)" in
  x86_64|amd64) arch=amd64 ;;
  aarch64|arm64) arch=arm64 ;;
  *) echo "Unsupported architecture: $(uname -m)" >&2; exit 2 ;;
esac

if [ -n "$version" ]; then
  base="https://github.com/arpingblue/AIStat/releases/download/${version}"
else
  base="https://github.com/arpingblue/AIStat/releases/latest/download"
fi
archive="aistat_linux_${arch}.tar.gz"
tmp_dir="$(mktemp -d)"
trap 'rm -rf -- "$tmp_dir"' EXIT INT TERM

curl --fail --location --proto '=https' --tlsv1.2 "$base/$archive" -o "$tmp_dir/$archive"
curl --fail --location --proto '=https' --tlsv1.2 "$base/checksums.txt" -o "$tmp_dir/checksums.txt"
(cd "$tmp_dir" && grep "  $archive\$" checksums.txt | sha256sum --check --status -)
tar -xzf "$tmp_dir/$archive" -C "$tmp_dir" aistat
install_dir="${AISTAT_INSTALL_DIR:-${HOME}/.local/bin}"
mkdir -p "$install_dir"
install -m 0755 "$tmp_dir/aistat" "$install_dir/aistat"

installed_version=$($install_dir/aistat version 2>/dev/null || true)
echo "Installed ${installed_version:-aistat} to $install_dir/aistat"
case ":${PATH}:" in
  *":${install_dir}:"*) ;;
  *)
    echo "Add AIStat to your PATH:"
    echo "  export PATH=\"$install_dir:\$PATH\""
    ;;
esac
