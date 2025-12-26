#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 1 ]]; then
  echo "Usage: $0 <version>" >&2
  exit 1
fi

VERSION=$1

if ! git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  echo "Error: This script must be run inside a git repository." >&2
  exit 1
fi

REPO_ROOT=$(git rev-parse --show-toplevel)
cd "$REPO_ROOT"

VERSION_FILE="$REPO_ROOT/VERSION"

if [[ ! -f "$VERSION_FILE" ]]; then
  echo "Error: VERSION file not found in repository root ($REPO_ROOT)." >&2
  exit 1
fi

if git show-ref --tags --verify --quiet "refs/tags/$VERSION"; then
  echo "Error: Tag $VERSION already exists." >&2
  exit 1
fi

if [[ -n $(git status --porcelain) ]]; then
  echo "Error: Working tree has uncommitted changes. Please commit or stash them before continuing." >&2
  exit 1
fi

CURRENT_VERSION=$(cat "$VERSION_FILE" | tr -d '[:space:]')
if [[ "$CURRENT_VERSION" == "$VERSION" ]]; then
  echo "Error: VERSION is already set to $VERSION." >&2
  exit 1
fi

# Update VERSION file
echo "$VERSION" > "$VERSION_FILE"

git add VERSION

if git diff --cached --quiet; then
  echo "Error: No changes to commit after updating VERSION." >&2
  exit 1
fi

COMMIT_MESSAGE="release: $VERSION"
git commit -m "$COMMIT_MESSAGE"

git tag -a "$VERSION" -m "Release $VERSION"

echo "Version updated to $VERSION."
echo "Created commit: $COMMIT_MESSAGE"
echo "Created annotated tag: $VERSION"
