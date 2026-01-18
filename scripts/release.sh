#!/bin/bash
set -e

TYPE="$1"

if [[ ! "$TYPE" =~ ^(patch|minor|major)$ ]]; then
    echo "Usage: $0 <patch|minor|major>"
    exit 1
fi

# Ensure on develop branch
if [ "$(git branch --show-current)" != "develop" ]; then
    echo "Error: Must be on develop branch"
    exit 1
fi

# Ensure working directory is clean
if [ -n "$(git status --porcelain)" ]; then
    echo "Error: Working directory is not clean"
    exit 1
fi

# Pull latest
git pull origin develop

# Get current version
CURRENT=$(git describe --tags --abbrev=0 2>/dev/null | sed 's/^v//')
MAJOR=$(echo "$CURRENT" | cut -d. -f1)
MINOR=$(echo "$CURRENT" | cut -d. -f2)
PATCH=$(echo "$CURRENT" | cut -d. -f3)

# Calculate new version
case "$TYPE" in
    patch)
        NEW_VERSION="$MAJOR.$MINOR.$((PATCH + 1))"
        ;;
    minor)
        NEW_VERSION="$MAJOR.$((MINOR + 1)).0"
        ;;
    major)
        NEW_VERSION="$((MAJOR + 1)).0.0"
        ;;
esac

echo ""
echo "Current version: v$CURRENT"
echo "New version:     v$NEW_VERSION"
echo ""
read -p "Proceed with release? [y/N] " confirm
if [ "$confirm" != "y" ]; then
    echo "Aborted"
    exit 1
fi

# Run git flow release
git flow release start "$NEW_VERSION"
git flow release finish "$NEW_VERSION" -m "Release v$NEW_VERSION"
git push origin main develop --tags

echo ""
echo "Released v$NEW_VERSION"
