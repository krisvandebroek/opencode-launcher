#!/usr/bin/env bash
set -euo pipefail

die() {
  echo "error: $*" >&2
  exit 1
}

need_cmd() {
  command -v "$1" >/dev/null 2>&1 || die "missing required command: $1"
}

need_cmd git

git rev-parse --git-dir >/dev/null 2>&1 || die "not a git repository"

REMOTE="${REMOTE:-origin}"
BASE_BRANCH="${BASE_BRANCH:-main}"

yes="${YES:-}"
release_type="${RELEASE_TYPE:-}"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --remote)
      REMOTE="$2"
      shift 2
      ;;
    --base-branch)
      BASE_BRANCH="$2"
      shift 2
      ;;
    --type)
      release_type="$2"
      shift 2
      ;;
    --yes|-y)
      yes="1"
      shift
      ;;
    --help|-h)
      cat <<'EOF'
Tag and push a new SemVer release.

Usage:
  scripts/release.sh [--type major|minor|patch] [--remote origin] [--base-branch main] [--yes]

Environment:
  REMOTE, BASE_BRANCH, RELEASE_TYPE, YES
EOF
      exit 0
      ;;
    *)
      die "unknown argument: $1"
      ;;
  esac
done

if [[ -n "$(git status --porcelain)" ]]; then
  die "working tree is dirty; commit or stash changes first"
fi

echo "Fetching ${REMOTE}/${BASE_BRANCH} and tags..."
git fetch --tags "$REMOTE" "$BASE_BRANCH" >/dev/null

latest_tag="$(git tag --merged "${REMOTE}/${BASE_BRANCH}" --list 'v*' --sort=-v:refname | head -n 1 || true)"
if [[ -z "$latest_tag" ]]; then
  latest_tag="v0.0.0"
fi

latest_ver="${latest_tag#v}"
IFS='.' read -r major minor patch <<<"$latest_ver" || true

[[ "$major" =~ ^[0-9]+$ ]] || die "latest tag is not semver: ${latest_tag}"
[[ "$minor" =~ ^[0-9]+$ ]] || die "latest tag is not semver: ${latest_tag}"
[[ "$patch" =~ ^[0-9]+$ ]] || die "latest tag is not semver: ${latest_tag}"

if [[ -z "$release_type" ]]; then
  echo
  echo "Latest tag on ${REMOTE}/${BASE_BRANCH}: ${latest_tag}"
  echo "Choose release type: (1) patch, (2) minor, (3) major"
  read -r -p "Release type [1]: " answer
  answer="${answer:-1}"
  case "$answer" in
    1|patch) release_type="patch" ;;
    2|minor) release_type="minor" ;;
    3|major) release_type="major" ;;
    *) die "invalid release type: ${answer}" ;;
  esac
fi

case "$release_type" in
  patch) new_major="$major"; new_minor="$minor"; new_patch=$((patch + 1)) ;;
  minor) new_major="$major"; new_minor=$((minor + 1)); new_patch=0 ;;
  major) new_major=$((major + 1)); new_minor=0; new_patch=0 ;;
  *) die "invalid --type: ${release_type} (expected major|minor|patch)" ;;
esac

new_tag="v${new_major}.${new_minor}.${new_patch}"

if git rev-parse -q --verify "refs/tags/${new_tag}" >/dev/null; then
  die "tag already exists: ${new_tag}"
fi

head_sha="$(git rev-parse HEAD)"
base_sha="$(git rev-parse "${REMOTE}/${BASE_BRANCH}")"

if [[ "$head_sha" != "$base_sha" ]]; then
  echo
  echo "warning: HEAD is not ${REMOTE}/${BASE_BRANCH}"
  echo "  HEAD:               ${head_sha}"
  echo "  ${REMOTE}/${BASE_BRANCH}:  ${base_sha}"
  if [[ "$yes" != "1" ]]; then
    read -r -p "Tag HEAD anyway? [y/N] " confirm
    [[ "${confirm}" == "y" || "${confirm}" == "Y" ]] || die "aborted"
  fi
fi

echo
echo "Creating tag ${new_tag} (latest on ${REMOTE}/${BASE_BRANCH} was ${latest_tag})"
echo "Target commit: ${head_sha}"

if [[ "$yes" != "1" ]]; then
  read -r -p "Create and push ${new_tag}? [y/N] " confirm
  [[ "${confirm}" == "y" || "${confirm}" == "Y" ]] || die "aborted"
fi

git tag -a "$new_tag" -m "Release ${new_tag}" "$head_sha"
git push "$REMOTE" "$new_tag"

echo "done: ${new_tag}"
