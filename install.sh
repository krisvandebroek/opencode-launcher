#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Install or upgrade opencode-launcher (the `oc` CLI).

Usage:
  ./install.sh [--version vX.Y.Z] [--name oc] [--bin-dir <dir>] [--repo owner/name]

Examples:
  ./install.sh
  ./install.sh --version v0.1.0
  ./install.sh --name opencode-launcher
  ./install.sh --bin-dir "$HOME/.local/bin"
  ./install.sh --repo krisvandebroek/opencode-launcher

Notes:
  - Re-running the installer upgrades an existing install.
  - If an unrelated `oc` is already on your PATH, you must choose another name.
EOF
}

repo="${REPO:-krisvandebroek/opencode-launcher}"
version=""
name="oc"
bin_dir="${BIN_DIR:-${HOME}/.local/bin}"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --help|-h)
      usage
      exit 0
      ;;
    --repo)
      repo="$2"
      shift 2
      ;;
    --version)
      version="$2"
      shift 2
      ;;
    --name)
      name="$2"
      shift 2
      ;;
    --bin-dir)
      bin_dir="$2"
      shift 2
      ;;
    *)
      echo "error: unknown argument: $1" >&2
      echo >&2
      usage >&2
      exit 2
      ;;
  esac
done

need_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "error: missing required command: $1" >&2
    exit 1
  fi
}

need_cmd curl
need_cmd tar
need_cmd install

os_raw="$(uname -s)"
arch_raw="$(uname -m)"

case "$os_raw" in
  Darwin) goos="darwin" ;;
  Linux)  goos="linux" ;;
  *)
    echo "error: unsupported OS: $os_raw" >&2
    exit 1
    ;;
esac

case "$arch_raw" in
  x86_64|amd64) goarch="amd64" ;;
  arm64|aarch64) goarch="arm64" ;;
  *)
    echo "error: unsupported architecture: $arch_raw" >&2
    exit 1
    ;;
esac

if [[ -z "$version" ]]; then
  latest_url="$(curl -fsSL -o /dev/null -w '%{url_effective}' "https://github.com/${repo}/releases/latest")"
  version="${latest_url##*/}"
fi

asset="oc_${version}_${goos}_${goarch}.tar.gz"
base_url="https://github.com/${repo}/releases/download/${version}"

tmp="$(mktemp -d)"
cleanup() { rm -rf "$tmp"; }
trap cleanup EXIT

echo "repo:      ${repo}"
echo "version:   ${version}"
echo "platform:  ${goos}/${goarch}"
echo "asset:     ${asset}"
echo "bin dir:   ${bin_dir}"
echo "name:      ${name}"

curl -fsSL -o "${tmp}/${asset}" "${base_url}/${asset}"
curl -fsSL -o "${tmp}/checksums.txt" "${base_url}/checksums.txt"

if ! grep -F " ${asset}" "${tmp}/checksums.txt" >/dev/null 2>&1; then
  echo "error: checksums.txt does not contain ${asset}" >&2
  exit 1
fi

grep -F " ${asset}" "${tmp}/checksums.txt" > "${tmp}/checksums.one"

if command -v sha256sum >/dev/null 2>&1; then
  (cd "$tmp" && sha256sum -c "checksums.one")
elif command -v shasum >/dev/null 2>&1; then
  (cd "$tmp" && shasum -a 256 -c "checksums.one")
else
  echo "error: need sha256sum (linux) or shasum (macOS) to verify downloads" >&2
  exit 1
fi

tar -xzf "${tmp}/${asset}" -C "$tmp"
bin_src="${tmp}/oc_${version}_${goos}_${goarch}/oc"

if [[ ! -f "$bin_src" ]]; then
  echo "error: expected binary not found: ${bin_src}" >&2
  exit 1
fi

pick_new_name() {
  local default_name="opencode-launcher"
  if [[ -t 0 ]]; then
    echo >&2
    echo "A command named '${name}' already exists on your PATH and does not look like opencode-launcher." >&2
    echo "Pick another install name (press enter for '${default_name}'):" >&2
    read -r new_name
    new_name="${new_name:-${default_name}}"
    name="$new_name"
    return 0
  fi

  echo >&2
  echo "error: '${name}' already exists on PATH and does not look like opencode-launcher" >&2
  echo "re-run with: --name opencode-launcher (or another name)" >&2
  exit 1
}

looks_like_ours() {
  local exe="$1"
  # NOTE: We intentionally avoid piping into `grep -q` under `set -o pipefail`.
  # `grep -q` can exit early after a match, causing the producer to receive
  # SIGPIPE and the pipeline to look like a failure (even when the match exists).
  local help_out
  help_out="$($exe --help 2>&1 || true)"
  [[ "$help_out" == *"speed-first OpenCode launcher"* ]]
}

while true; do
  # Only consider real executables on PATH. Avoid being fooled by shell
  # functions/aliases (e.g. via BASH_ENV) which can make `command -v` return
  # non-path values.
  existing="$(type -P "$name" 2>/dev/null || true)"
  if [[ -z "$existing" ]]; then
    break
  fi

  if looks_like_ours "$existing"; then
    break
  fi

  pick_new_name
done

mkdir -p "$bin_dir"
dest="${bin_dir%/}/${name}"

existing_version=""
if [[ -x "$dest" ]] && looks_like_ours "$dest"; then
  existing_version="$($dest --version 2>/dev/null || true)"
fi

install -m 0755 "$bin_src" "$dest"

echo "installed: ${dest}"
echo "version:   $($dest --version)"

if [[ -n "$existing_version" ]]; then
  echo "previous:  ${existing_version}"
fi
