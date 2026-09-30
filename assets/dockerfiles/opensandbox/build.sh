#!/bin/bash
set -euo pipefail

# --- Version configuration ---
VERSION="v0.0.1"
HYPERFRAMES_VERSION="v0.8.35"

# --- Target architecture ---
# Usage: ./build.sh [amd64|arm64]   (defaults to the host architecture)
ARCH="${1:-}"
if [ -z "$ARCH" ]; then
    case "$(uname -m)" in
        x86_64)  ARCH="amd64" ;;
        aarch64) ARCH="arm64" ;;
        *)       echo "Unsupported host arch $(uname -m), pass amd64 or arm64 explicitly" >&2; exit 1 ;;
    esac
fi
case "$ARCH" in
    amd64|arm64) ;;
    *) echo "Usage: $0 [amd64|arm64]" >&2; exit 1 ;;
esac

HOST_ARCH="$(uname -m)"
case "$HOST_ARCH" in
    x86_64)  HOST_ARCH="amd64" ;;
    aarch64) HOST_ARCH="arm64" ;;
esac

PLATFORM="linux/${ARCH}"
IMAGE="ghcr.io/gonotelm-lab/opensandbox-${ARCH}:${VERSION}"
CHROME_HOST_CACHE="${HYPERFRAMES_CACHE_DIR:-$HOME/.cache/hyperframes}"
CHROME_CTX=$(mktemp -d)
trap 'rm -rf "$CHROME_CTX"' EXIT

# --- hyperframes cache: inject if exists, otherwise leave empty ---
# fonts/ is arch-neutral and shared; chrome/ holds an arch-specific browser
# binary (chrome-for-testing ships linux64 only, and hyperframes cannot use that
# on arm64), so it is only injected for amd64.
if [ -d "$CHROME_HOST_CACHE" ]; then
    echo "==> Syncing host hyperframes cache from $CHROME_HOST_CACHE (${ARCH})"
    if [ -d "$CHROME_HOST_CACHE/fonts" ]; then
        cp -a "$CHROME_HOST_CACHE/fonts" "$CHROME_CTX/"
    fi
    if [ "$ARCH" = "amd64" ] && [ -d "$CHROME_HOST_CACHE/chrome" ]; then
        cp -a "$CHROME_HOST_CACHE/chrome" "$CHROME_CTX/"
    fi
else
    echo "==> No host hyperframes cache, container will download."
fi

if [ "$ARCH" != "$HOST_ARCH" ]; then
    echo "==> WARNING: cross-building ${ARCH} on ${HOST_ARCH}; needs QEMU/binfmt, e.g."
    echo "    docker run --privileged --rm tonistiigi/binfmt --install ${ARCH}"
fi

echo "==> Building $IMAGE (hyperframes $HYPERFRAMES_VERSION, platform $PLATFORM)"
docker buildx build \
    --progress=plain \
    --platform "$PLATFORM" \
    --build-arg "TARGETARCH=$ARCH" \
    --build-arg "HYPERFRAMES_VERSION=$HYPERFRAMES_VERSION" \
    --build-context chrome-cache="$CHROME_CTX" \
    -t "$IMAGE" \
    .

echo "==> Done: $IMAGE"
