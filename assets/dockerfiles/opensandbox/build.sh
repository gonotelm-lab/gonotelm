#!/bin/bash
set -euo pipefail

# --- Version configuration ---
VERSION="v0.0.1"
HYPERFRAMES_VERSION="v0.8.35"

IMAGE="ghcr.io/gonotelm-lab/opensandbox-amd64:${VERSION}"
CHROME_HOST_CACHE="$HOME/.cache/hyperframes"
CHROME_CTX=$(mktemp -d)
trap 'rm -rf "$CHROME_CTX"' EXIT

# --- Chrome cache: inject if exists, otherwise leave empty ---
if [ -d "$CHROME_HOST_CACHE" ]; then
    echo "==> Syncing host Chrome cache from $CHROME_HOST_CACHE"
    cp -a "$CHROME_HOST_CACHE/." "$CHROME_CTX/"
else
    echo "==> No host Chrome cache, container will download."
fi

echo "==> Building $IMAGE (hyperframes $HYPERFRAMES_VERSION)"
docker buildx build \
    --progress=plain \
    --build-arg "HYPERFRAMES_VERSION=$HYPERFRAMES_VERSION" \
    --build-context chrome-cache="$CHROME_CTX" \
    -t "$IMAGE" \
    .

echo "==> Done: $IMAGE"