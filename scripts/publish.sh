#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Usage: scripts/publish.sh [patch|minor|major]

Bump and publish a semver tag (vX.Y.Z) that triggers the release workflow.

Defaults:
  increment: patch

Examples:
  scripts/publish.sh
  scripts/publish.sh minor
  scripts/publish.sh major
EOF
}

increment="${1:-patch}"
if [[ "$increment" == "-h" || "$increment" == "--help" ]]; then
  usage
  exit 0
fi

if [[ "$increment" != "patch" && "$increment" != "minor" && "$increment" != "major" ]]; then
  echo "Invalid increment: $increment"
  usage
  exit 1
fi

if ! git rev-parse --git-dir >/dev/null 2>&1; then
  echo "Not inside a git repository."
  exit 1
fi

branch="$(git rev-parse --abbrev-ref HEAD)"
if [[ "$branch" == "HEAD" ]]; then
  echo "Detached HEAD is not supported for publish."
  exit 1
fi

if [[ -n "$(git status --porcelain)" ]]; then
  echo "Working tree is not clean. Commit or stash changes before publishing."
  exit 1
fi

git fetch --tags origin

latest_tag="$(git tag -l 'v[0-9]*.[0-9]*.[0-9]*' | sort -V | awk 'NF{t=$0} END{print t}')"
if [[ -z "${latest_tag:-}" ]]; then
  latest_tag="v0.0.0"
fi

base="${latest_tag#v}"
IFS='.' read -r major minor patch <<<"$base"

case "$increment" in
  patch)
    patch=$((patch + 1))
    ;;
  minor)
    minor=$((minor + 1))
    patch=0
    ;;
  major)
    major=$((major + 1))
    minor=0
    patch=0
    ;;
esac

next_tag="v${major}.${minor}.${patch}"

if git rev-parse "$next_tag" >/dev/null 2>&1; then
  echo "Tag already exists locally: $next_tag"
  exit 1
fi

if git ls-remote --tags origin "refs/tags/${next_tag}" | awk 'NF{exit 0} END{exit 1}'; then
  echo "Tag already exists on origin: $next_tag"
  exit 1
fi

echo "Release plan:"
echo "  branch:      $branch"
echo "  latest tag:  $latest_tag"
echo "  increment:   $increment"
echo "  new tag:     $next_tag"
echo
read -r -p "Continue and publish ${next_tag}? [y/N] " confirm
if [[ ! "$confirm" =~ ^[Yy]$ ]]; then
  echo "Cancelled."
  exit 0
fi

echo "Pushing branch ${branch}..."
git push origin "$branch"

echo "Creating tag ${next_tag}..."
git tag -a "$next_tag" -m "$next_tag"

echo "Pushing tag ${next_tag}..."
git push origin "$next_tag"

echo
echo "Published ${next_tag}."
echo "Release workflow should start automatically."
