#!/usr/bin/env bash

set -euo pipefail

# 1. Resolve bump type.
# BUMP_TYPE comes from the workflow input (default: patch).
BUMP_TYPE="${BUMP_TYPE:-patch}"

# Validate required inputs
[ -n "${APP_NAME:-}" ] || { echo "ERROR: APP_NAME is required" >&2; exit 1; }

# 2. Find the latest existing tag
APP_PREFIX="${APP_NAME}-v"
LATEST_TAG=$(git tag -l "${APP_PREFIX}*" | sort -V | tail -n 1)

# 3. Parse current version components
MAJOR=1; MINOR=0; PATCH=0; BUILD=0

if [ -n "$LATEST_TAG" ]; then
  # Use the latest git tag (created automatically after each workflow run)
  VERSION_STR=${LATEST_TAG#"$APP_PREFIX"}

  if [[ "$VERSION_STR" == *"-build."* ]]; then
    SEMVER=${VERSION_STR%-build.*}
    BUILD=${VERSION_STR##*-build.}
    IFS='.' read -r MAJOR MINOR PATCH <<< "$SEMVER"
  elif [[ "$VERSION_STR" == *"+"* ]]; then
    SEMVER=${VERSION_STR%+*}
    BUILD=${VERSION_STR##*+}
    IFS='.' read -r MAJOR MINOR PATCH <<< "$SEMVER"
  
  else
    echo "ERROR: Unrecognised version format '${VERSION_STR}' from tag '${LATEST_TAG}'" >&2
    exit 1
  fi

  MAJOR=${MAJOR:-1}
  MINOR=${MINOR:-0}
  PATCH=${PATCH:-0}
  BUILD=${BUILD:-0}

elif [ -n "${INITIAL_VERSION:-}" ]; then
  # No git tag yet — use the version from the caller (last known DB version)
  # Format: 2.4.0-build.5
  SEMVER=${INITIAL_VERSION%-build.*}
  BUILD=${INITIAL_VERSION##*-build.}
  IFS='.' read -r MAJOR MINOR PATCH <<< "$SEMVER"
  MAJOR=${MAJOR:-1}
  MINOR=${MINOR:-0}
  PATCH=${PATCH:-0}
  BUILD=${BUILD:-0}
fi

# 4. Increment based on bump type
case "$BUMP_TYPE" in
  major)
    MAJOR=$((MAJOR + 1))
    MINOR=0; PATCH=0; BUILD=$((BUILD + 1))
    ;;
  minor)
    MINOR=$((MINOR + 1))
    PATCH=0; BUILD=$((BUILD + 1))
    ;;
  patch )
    PATCH=$((PATCH + 1))
    BUILD=$((BUILD + 1))
    ;;
     *)
    echo "ERROR: Unknown BUMP_TYPE '${BUMP_TYPE}'. Expected: major | minor | patch" >&2 #
    exit 1
    ;;
esac

# 5. Emit outputs
VERSION="${MAJOR}.${MINOR}.${PATCH}"
echo "version=${VERSION}" >> "$GITHUB_OUTPUT"
echo "build=${BUILD}" >> "$GITHUB_OUTPUT"
echo "Bump type: ${BUMP_TYPE} → version: ${VERSION}, build: ${BUILD}"
