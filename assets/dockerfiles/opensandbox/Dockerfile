FROM node:22-bookworm-slim

# Use bash for RUN commands to support source and arrays
SHELL ["/bin/bash", "-c"]

ENV DEBIAN_FRONTEND=noninteractive \
    LANG=C.UTF-8

# Install common utils
RUN sed -i 's|deb.debian.org|mirrors.aliyun.com|g' /etc/apt/sources.list.d/debian.sources \
    && sed -i 's|security.debian.org|mirrors.aliyun.com|g' /etc/apt/sources.list.d/debian.sources \
    && apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    curl \
    unzip \
    python3 \
    ffmpeg \
    libgbm1 \
    libnss3 \
    libatk-bridge2.0-0 \
    libdrm2 \
    libxcomposite1 \
    libxdamage1 \
    libxrandr2 \
    libcups2 \
    libasound2 \
    libpangocairo-1.0-0 \
    libxshmfence1 \
    libgtk-3-0 \
    # Font support
    fonts-liberation \
    fonts-dejavu-core \
    fonts-noto-cjk \
    fonts-noto-cjk-extra \
    fonts-noto-color-emoji \
    fontconfig \
    && rm -rf /var/lib/apt/lists/* \
    && apt-get clean \
    && fc-cache -fv

# Install hyperframes
ARG HYPERFRAMES_VERSION=v0.8.35
ARG NPM_CACHE
RUN curl -o /dev/null -s -w "%{time_total}\n" https://registry.npmmirror.com/hyperframes
RUN --mount=type=cache,target=/root/.npm,sharing=locked \
    npm install -g hyperframes@${HYPERFRAMES_VERSION} \
        --registry=https://registry.npmmirror.com \
        --no-audit \
        --no-fund \
        --loglevel=http \
        --foreground-scripts \
        --onnxruntime-node-install-cuda=skip

# hyperframes cache from the host. build.sh only puts arch-compatible entries in
# here: fonts/ shares fine, chrome/ is an x86-64 binary and is skipped on arm64.
COPY --from=chrome-cache . /root/.cache/hyperframes/

# Browser install.
#   amd64 -> chrome-for-testing linux64, fetched by `hyperframes browser ensure`
#            (the COPY above already provides it when the host had a cache).
#   arm64 -> chrome-for-testing publishes no linux-arm64 build at all, and
#            Debian's rolling `chromium` arm64 package is broken (SIGTRAP on
#            start, heygen-com/hyperframes#2039), so install Playwright's pinned
#            linux-arm64 chrome-headless-shell instead.
# TARGETARCH is amd64 (default) or arm64. It is declared here rather than at the
# top of the file so that adding it does not invalidate the cached apt/npm layers
# above; BuildKit sets it from --platform automatically, and build.sh passes it.
ARG TARGETARCH=amd64
# Pinned Playwright release that ships the linux-arm64 chrome-headless-shell.
# Bump deliberately: the browser wired into the image below is tied to it.
ARG PLAYWRIGHT_VERSION=1.61.1
RUN if [ "$TARGETARCH" = "arm64" ]; then \
        npx --yes playwright-core@"${PLAYWRIGHT_VERSION}" install chromium-headless-shell; \
    else \
        npx hyperframes browser ensure; \
    fi

# hyperframes has no linux/arm64 entry in its cached-headless-shell table, and it
# only treats a binary whose basename is exactly `chrome-headless-shell` as a real
# headless shell (anything else silently degrades to full-Chrome screenshot
# capture), so expose whichever binary landed under that exact name on both arches.
# The --version smoke test fails the build rather than shipping an unrunnable
# browser.
RUN SHELL_BIN=$(find /root/.cache/ms-playwright /root/.cache/hyperframes /root/.cache/puppeteer \
        \( -name chrome-headless-shell -o -name headless_shell \) -type f 2>/dev/null | head -1); \
    if [ -z "$SHELL_BIN" ]; then \
        echo "ERROR: no chrome-headless-shell found for ${TARGETARCH}: the browser install step failed." >&2; \
        exit 1; \
    fi \
    && mkdir -p /opt/hyperframes \
    && ln -sf "$SHELL_BIN" /opt/hyperframes/chrome-headless-shell \
    && /opt/hyperframes/chrome-headless-shell --version

ENV HYPERFRAMES_BROWSER_PATH=/opt/hyperframes/chrome-headless-shell
RUN npx hyperframes browser ensure
RUN hyperframes doctor || true

CMD ["/bin/bash"]
