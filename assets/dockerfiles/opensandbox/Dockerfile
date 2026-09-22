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
ARG NPM_CACHE
RUN curl -o /dev/null -s -w "%{time_total}\n" https://registry.npmmirror.com/hyperframes
RUN --mount=type=cache,target=/root/.npm,sharing=locked \
    npm install -g hyperframes@v0.8.35 \
        --registry=https://registry.npmmirror.com \
        --no-audit \
        --no-fund \
        --loglevel=http \
        --foreground-scripts \
        --onnxruntime-node-install-cuda=skip

COPY --from=chrome-cache . /root/.cache/hyperframes/
RUN npx hyperframes browser ensure
RUN hyperframes doctor || true

CMD ["/bin/bash"]
