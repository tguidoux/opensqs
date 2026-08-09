#!/usr/bin/env bash
#
# release.sh — Create a versioned release for OpenSQS
#
# This script:
#   1. Validates the working tree is clean
#   2. Determines the next version (or uses a user-supplied one)
#   3. Updates version references (Helm Chart.yaml)
#   4. Commits the version bump
#   5. Creates and pushes a git tag
#   6. Builds the multi-arch OCI image with Bazel
#   7. Pushes the image to GHCR (ghcr.io/tguidoux/opensqs/opensqs-server)
#   8. Creates a GitHub Release
#
# Usage:
#   ./scripts/release.sh                  # Auto-increment patch version (v0.1.0 → v0.1.1)
#   ./scripts/release.sh v1.0.0           # Use a specific version
#   ./scripts/release.sh v1.0.0 --skip-image  # Skip image build/push
#
# Prerequisites:
#   - Docker logged in to GHCR:  echo "$GITHUB_TOKEN" | docker login ghcr.io -u tguidoux --password-stdin
#   - GitHub CLI (gh) installed and authenticated
#   - Bazel installed
#

set -euo pipefail

# ─── Colors ──────────────────────────────────────────────────────────────────
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
BOLD='\033[1m'
NC='\033[0m' # No Color

info()  { echo -e "${BLUE}ℹ️  $*${NC}"; }
ok()    { echo -e "${GREEN}✅ $*${NC}"; }
warn()  { echo -e "${YELLOW}⚠️  $*${NC}"; }
error() { echo -e "${RED}❌ $*${NC}" >&2; }

# ─── Configuration ────────────────────────────────────────────────────────────
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
CHART_FILE="$REPO_ROOT/deploy/helm/Chart.yaml"
GHCR_REPO="ghcr.io/tguidoux/opensqs/opensqs-server"
REMOTE_NAME="origin"

cd "$REPO_ROOT"

# ─── Helpers ─────────────────────────────────────────────────────────────────

usage() {
    cat <<EOF
Usage: $(basename "$0") [VERSION] [OPTIONS]

Arguments:
  VERSION                 Semantic version (e.g., v1.0.0). If omitted, auto-increments patch.

Options:
  --skip-image            Skip Bazel image build and push
  --skip-tag              Skip git tag creation and push (use existing tag)
  --skip-release          Skip GitHub Release creation
  --dry-run               Show what would happen without making changes
  -h, --help              Show this help message

Examples:
  $(basename "$0")                        # Auto-increment: v0.1.0 → v0.1.1
  $(basename "$0") v1.0.0                # Release v1.0.0
  $(basename "$0") v2.0.0 --skip-image   # Tag v2.0.0, skip image push
  $(basename "$0") --dry-run             # Preview what would happen
EOF
}

check_command() {
    if ! command -v "$1" &>/dev/null; then
        error "Required command not found: $1"
        exit 1
    fi
}

get_latest_tag() {
    # Get the latest git tag, or fall back to Helm chart version
    local latest
    latest=$(git describe --tags --abbrev=0 2>/dev/null || echo "")
    if [ -n "$latest" ]; then
        echo "$latest"
        return
    fi

    # No git tags exist — use the Helm chart version as starting point
    local chart_ver
    chart_ver=$(get_chart_version)
    if [ -n "$chart_ver" ]; then
        echo "v${chart_ver}"
    else
        echo "v0.1.0"
    fi
}

bump_patch_version() {
    local version="$1"
    # Strip leading 'v'
    local stripped="${version#v}"
    # Split into major.minor.patch
    local major minor patch
    IFS='.' read -r major minor patch <<< "$stripped"
    # Increment patch
    patch=$((patch + 1))
    echo "v${major}.${minor}.${patch}"
}

get_chart_version() {
    # Extract version from Chart.yaml (strip quotes if present)
    grep '^version:' "$CHART_FILE" | awk '{print $2}' | tr -d '"'
}

get_chart_app_version() {
    grep '^appVersion:' "$CHART_FILE" | awk '{print $2}' | tr -d '"'
}

update_chart_version() {
    local version="$1"
    local stripped="${version#v}"

    if [ "$DRY_RUN" = true ]; then
        info "[dry-run] Would update Chart.yaml: version=$stripped, appVersion=$version"
        return
    fi

    # Update both version and appVersion in Chart.yaml
    # Using sed with careful patterns to match the YAML structure
    sed -i.bak "s/^version:.*/version: \"${stripped}\"/" "$CHART_FILE"
    sed -i.bak "s/^appVersion:.*/appVersion: \"${version}\"/" "$CHART_FILE"
    rm -f "$CHART_FILE.bak"

    ok "Updated Chart.yaml: version=${stripped}, appVersion=${version}"
}

# ─── Parse Arguments ─────────────────────────────────────────────────────────
VERSION=""
SKIP_IMAGE=false
SKIP_TAG=false
SKIP_RELEASE=false
DRY_RUN=false

while [[ $# -gt 0 ]]; do
    case "$1" in
        --skip-image)    SKIP_IMAGE=true; shift ;;
        --skip-tag)      SKIP_TAG=true; shift ;;
        --skip-release)  SKIP_RELEASE=true; shift ;;
        --dry-run)       DRY_RUN=true; shift ;;
        -h|--help)       usage; exit 0 ;;
        v*)              VERSION="$1"; shift ;;
        *)
            error "Unknown argument: $1"
            usage
            exit 1
            ;;
    esac
done

# ─── Preflight Checks ────────────────────────────────────────────────────────
info "Running preflight checks..."

check_command git
check_command bazel

if [ "$SKIP_RELEASE" = false ]; then
    check_command gh
fi

if [ "$SKIP_IMAGE" = false ]; then
    check_command docker
fi

# Check working tree is clean
if [ -n "$(git status --porcelain)" ]; then
    error "Working tree is not clean. Please commit or stash changes before releasing."
    git status --short
    exit 1
fi

# Check we're on a branch (not detached HEAD)
CURRENT_BRANCH=$(git rev-parse --abbrev-ref HEAD)
if [ "$CURRENT_BRANCH" = "HEAD" ]; then
    error "Detached HEAD state. Please checkout a branch first."
    exit 1
fi

ok "On branch: $CURRENT_BRANCH"

# ─── Determine Version ───────────────────────────────────────────────────────
if [ -z "$VERSION" ]; then
    LATEST_TAG=$(get_latest_tag)
    VERSION=$(bump_patch_version "$LATEST_TAG")
    info "No version specified. Auto-incrementing: $LATEST_TAG → $VERSION"
else
    info "Using specified version: $VERSION"
fi

# Validate version format
if ! [[ "$VERSION" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    error "Invalid version format. Expected: v1.2.3 (semver with 'v' prefix)"
    exit 1
fi

# Check if tag already exists
if git rev-parse "$VERSION" >/dev/null 2>&1; then
    error "Tag $VERSION already exists!"
    error "Existing tags:"
    git tag --list | head -20
    exit 1
fi

STRIPPED_VERSION="${VERSION#v}"
info "Release version: ${BOLD}$VERSION${NC}"

# ─── Step 1: Update Helm Chart ───────────────────────────────────────────────
echo ""
info "${BOLD}Step 1: Update Helm Chart${NC}"

CURRENT_CHART_VERSION=$(get_chart_version)
info "Current Chart.yaml: version=$CURRENT_CHART_VERSION"

update_chart_version "$VERSION"

if [ "$DRY_RUN" = false ]; then
    git add "$CHART_FILE"
fi

# ─── Step 2: Commit Version Bump ─────────────────────────────────────────────
echo ""
info "${BOLD}Step 2: Commit version bump${NC}"

if [ "$DRY_RUN" = true ]; then
        info "[dry-run] Would commit: release($VERSION): bump Helm chart to $STRIPPED_VERSION"
    else
        git commit -m "release($VERSION): bump Helm chart to $STRIPPED_VERSION"
        ok "Committed version bump"
    fi

# ─── Step 3: Create and Push Git Tag ─────────────────────────────────────────
echo ""
info "${BOLD}Step 3: Create and push git tag${NC}"

if [ "$SKIP_TAG" = true ]; then
    warn "Skipping git tag creation (--skip-tag)"
else
    if [ "$DRY_RUN" = true ]; then
        info "[dry-run] Would create annotated tag: $VERSION"
        info "[dry-run] Would push tag $VERSION to $REMOTE_NAME"
    else
        git tag -a "$VERSION" -m "Release $VERSION"
        ok "Created annotated tag: $VERSION"

        git push "$REMOTE_NAME" "$VERSION"
        ok "Pushed tag $VERSION to $REMOTE_NAME"
    fi
fi

# Also push the version bump commit
if [ "$DRY_RUN" = false ]; then
    git push "$REMOTE_NAME" "$CURRENT_BRANCH"
    ok "Pushed commits to $REMOTE_NAME/$CURRENT_BRANCH"
fi

# ─── Step 4: Build and Push OCI Image ────────────────────────────────────────
echo ""
info "${BOLD}Step 4: Build and push OCI image${NC}"

# The image_tags in BUILD.bazel define the repo:tag used by oci_load.
# We load into Docker, then retag and push with docker — this avoids
# crane credential issues in the Bazel sandbox.
IMAGE_LOAD_TARGET="//apps/go/server:opensqs_server_image_platform_transition_load_docker"
LOCAL_IMAGE="opensqs-server:latest"

if [ "$SKIP_IMAGE" = true ]; then
    warn "Skipping image build and push (--skip-image)"
else
    if [ "$DRY_RUN" = true ]; then
        info "[dry-run] Would build and load image: bazel run $IMAGE_LOAD_TARGET"
        info "[dry-run] Would tag: docker tag $LOCAL_IMAGE ${GHCR_REPO}:${VERSION}"
        info "[dry-run] Would push: docker push ${GHCR_REPO}:${VERSION}"
        info "[dry-run] Would tag: docker tag $LOCAL_IMAGE ${GHCR_REPO}:latest"
        info "[dry-run] Would push: docker push ${GHCR_REPO}:latest"
    else
        info "Building and loading OCI image into Docker..."
        bazel run "$IMAGE_LOAD_TARGET"
        ok "Image loaded into Docker as $LOCAL_IMAGE"

        info "Tagging and pushing image to GHCR: ${GHCR_REPO}:${VERSION}"
        docker tag "$LOCAL_IMAGE" "${GHCR_REPO}:${VERSION}"
        docker push "${GHCR_REPO}:${VERSION}"
        ok "Image pushed: ${GHCR_REPO}:${VERSION}"

        info "Tagging and pushing image to GHCR: ${GHCR_REPO}:latest"
        docker tag "$LOCAL_IMAGE" "${GHCR_REPO}:latest"
        docker push "${GHCR_REPO}:latest"
        ok "Image pushed: ${GHCR_REPO}:latest"
    fi
fi

# ─── Step 5: Create GitHub Release ───────────────────────────────────────────
echo ""
info "${BOLD}Step 5: Create GitHub Release${NC}"

if [ "$SKIP_RELEASE" = true ]; then
    warn "Skipping GitHub Release creation (--skip-release)"
else
    if [ "$DRY_RUN" = true ]; then
        info "[dry-run] Would create GitHub Release: $VERSION"
    else
        # Generate release notes from commits since last tag
        PREVIOUS_TAG=$(get_latest_tag)
        if [ "$PREVIOUS_TAG" = "v0.0.0" ]; then
            # No previous tag, use all commits
            RELEASE_NOTES=$(git log --pretty=format:"- %s" HEAD)
        else
            RELEASE_NOTES=$(git log --pretty=format:"- %s" "${PREVIOUS_TAG}..HEAD")
        fi

        # Create the GitHub release
        gh release create "$VERSION" \
            --title "Release $VERSION" \
            --notes "## Changes

$RELEASE_NOTES

## Docker Image

\`\`\`
docker pull ${GHCR_REPO}:${VERSION}
\`\`\`

## Helm

\`\`\`bash
helm install opensqs deploy/helm \
  --set image.tag=${VERSION}
\`\`\`
" \
            --verify-tag

        ok "GitHub Release created: $VERSION"
    fi
fi

# ─── Summary ─────────────────────────────────────────────────────────────────
echo ""
echo -e "${GREEN}${BOLD}════════════════════════════════════════════════════${NC}"
echo -e "${GREEN}${BOLD}  🎉 Release $VERSION completed successfully!${NC}"
echo -e "${GREEN}${BOLD}════════════════════════════════════════════════════${NC}"
echo ""
echo -e "  ${BOLD}Git Tag:${NC}        $VERSION"
echo -e "  ${BOLD}Image:${NC}          ${GHCR_REPO}:${VERSION}"
echo -e "  ${BOLD}Image (latest):${NC} ${GHCR_REPO}:latest"
echo -e "  ${BOLD}Helm:${NC}           helm install opensqs deploy/helm --set image.tag=${VERSION}"
echo ""
