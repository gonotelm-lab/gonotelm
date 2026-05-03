#!/usr/bin/env sh

# 用法:
#   source scripts/load-env.sh
#   source scripts/load-env.sh path/to/.env

ENV_FILE="${1:-.env}"

if [ ! -f "${ENV_FILE}" ]; then
  echo "Env file not found: ${ENV_FILE}" >&2
  return 1 2>/dev/null || exit 1
fi

set -a
. "${ENV_FILE}"
set +a

echo "Loaded env from ${ENV_FILE}"
