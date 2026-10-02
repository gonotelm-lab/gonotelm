#!/bin/sh
# gonotelm MinIO bootstrap, idempotent (existing objects are never touched).
#
#   gonotelm         anonymous none,       read/write via GONOTELM_MINIO_ACCESS_KEY
#   gonotelm-public  anonymous GetObject,  write via GONOTELM_PUBLIC_MINIO_ACCESS_KEY
#
# Admin (root) credentials come from GONOTELM_DEV_MINIO_ROOT_USER /
# GONOTELM_DEV_MINIO_ROOT_PASSWORD, default minioadmin.
#
# Env: GONOTELM_MINIO_ACCESS_KEY, GONOTELM_MINIO_SECRET_KEY            required
#      GONOTELM_MINIO_BUCKET (gonotelm), GONOTELM_MINIO_POLICY_NAME (gonotelm-rw)
#      GONOTELM_PUBLIC_MINIO_ACCESS_KEY, GONOTELM_PUBLIC_MINIO_SECRET_KEY  required
#      GONOTELM_PUBLIC_MINIO_BUCKET (gonotelm-public)
#      GONOTELM_PUBLIC_MINIO_POLICY_NAME (gonotelm-public-rw)
#      GONOTELM_DEV_MINIO_ROOT_USER / GONOTELM_DEV_MINIO_ROOT_PASSWORD (minioadmin)
#      GONOTELM_MINIO_ENDPOINT (127.0.0.1:9000), GONOTELM_MINIO_SECURE (false)
#
# Host:      set -a && . ./.env && set +a && migration/storage/minio.sh
# Container: compose service minio-init mounts this file and runs it with /bin/sh

set -eu

: "${GONOTELM_MINIO_ACCESS_KEY:?GONOTELM_MINIO_ACCESS_KEY is not set (credential of the private bucket)}"
: "${GONOTELM_MINIO_SECRET_KEY:?GONOTELM_MINIO_SECRET_KEY is not set (secret key of the private bucket credential)}"
: "${GONOTELM_PUBLIC_MINIO_ACCESS_KEY:?GONOTELM_PUBLIC_MINIO_ACCESS_KEY is not set (credential of the public-read bucket)}"
: "${GONOTELM_PUBLIC_MINIO_SECRET_KEY:?GONOTELM_PUBLIC_MINIO_SECRET_KEY is not set (secret key of the public-read bucket credential)}"

root_user="${GONOTELM_DEV_MINIO_ROOT_USER:-minioadmin}"
root_password="${GONOTELM_DEV_MINIO_ROOT_PASSWORD:-minioadmin}"
private_bucket="${GONOTELM_MINIO_BUCKET:-gonotelm}"
private_policy="${GONOTELM_MINIO_POLICY_NAME:-gonotelm-rw}"
private_ak="$GONOTELM_MINIO_ACCESS_KEY"
private_sk="$GONOTELM_MINIO_SECRET_KEY"
public_bucket="${GONOTELM_PUBLIC_MINIO_BUCKET:-gonotelm-public}"
public_policy="${GONOTELM_PUBLIC_MINIO_POLICY_NAME:-gonotelm-public-rw}"
public_ak="$GONOTELM_PUBLIC_MINIO_ACCESS_KEY"
public_sk="$GONOTELM_PUBLIC_MINIO_SECRET_KEY"
admin_alias="gonotelm-admin"

endpoint="${GONOTELM_MINIO_ENDPOINT:-127.0.0.1:9000}"
case "$endpoint" in
  http://* | https://*) ;;
  *)
    if [ "${GONOTELM_MINIO_SECURE:-false}" = "true" ]; then
      endpoint="https://${endpoint}"
    else
      endpoint="http://${endpoint}"
    fi
    ;;
esac

# throwaway MC_CONFIG_DIR so the caller's ~/.mc aliases stay untouched
work_dir="$(mktemp -d)"
MC_CONFIG_DIR="${work_dir}/mc"
export MC_CONFIG_DIR
trap 'rm -rf "$work_dir"' 0 1 2 15

mc alias set "$admin_alias" "$endpoint" "$root_user" "$root_password" >/dev/null

# check every credential before mutating anything, so a bad one cannot leave the run half applied
check_credential() {
  if [ "$1" = "$root_user" ]; then
    echo "[minio] $2 must not be the admin ${root_user}: each bucket needs its own credential" >&2
    exit 1
  fi
  if mc admin user svcacct info "$admin_alias" "$1" >/dev/null 2>&1; then
    echo "[minio] $2 (${1}) is already an existing service account, pick a fresh access key" >&2
    echo "[minio] or drop the old one first: mc admin user svcacct rm <alias> ${1}" >&2
    exit 1
  fi
}

check_credential "$private_ak" "GONOTELM_MINIO_ACCESS_KEY"
check_credential "$public_ak" "GONOTELM_PUBLIC_MINIO_ACCESS_KEY"
if [ "$private_ak" = "$public_ak" ]; then
  echo "[minio] the two buckets need different credentials, both are ${private_ak}" >&2
  exit 1
fi

# ensure_bucket <bucket> <policy-name> <access-key> <secret-key> <anonymous-mode>
#   anonymous-mode: none | object-read (anonymous GetObject only)
ensure_bucket() {
  eb_bucket="$1"
  eb_policy="$2"
  eb_ak="$3"
  eb_sk="$4"
  eb_anon="$5"

  mc mb --ignore-existing "${admin_alias}/${eb_bucket}"

  # not `mc anonymous set download`: that also grants ListBucket/GetBucketLocation,
  # letting anonymous clients enumerate every object
  case "$eb_anon" in
    none)
      mc anonymous set none "${admin_alias}/${eb_bucket}"
      ;;
    object-read)
      cat > "${work_dir}/anon-${eb_bucket}.json" <<JSON
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "PublicReadObjects",
      "Effect": "Allow",
      "Principal": {
        "AWS": [
          "*"
        ]
      },
      "Action": [
        "s3:GetObject"
      ],
      "Resource": [
        "arn:aws:s3:::${eb_bucket}/*"
      ]
    }
  ]
}
JSON
      # set-json takes the file first, then the target
      mc anonymous set-json "${work_dir}/anon-${eb_bucket}.json" "${admin_alias}/${eb_bucket}"
      ;;
    *)
      echo "[minio] unknown anonymous mode ${eb_anon}" >&2
      exit 1
      ;;
  esac

  cat > "${work_dir}/policy-${eb_bucket}.json" <<JSON
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "ListBucket",
      "Effect": "Allow",
      "Action": [
        "s3:ListBucket",
        "s3:ListBucketMultipartUploads",
        "s3:GetBucketLocation"
      ],
      "Resource": [
        "arn:aws:s3:::${eb_bucket}"
      ]
    },
    {
      "Sid": "ReadWriteBucketObjects",
      "Effect": "Allow",
      "Action": [
        "s3:GetObject",
        "s3:GetObjectVersion",
        "s3:GetObjectAttributes",
        "s3:PutObject",
        "s3:DeleteObject",
        "s3:AbortMultipartUpload",
        "s3:ListMultipartUploadParts",
        "s3:GetObjectTagging",
        "s3:PutObjectTagging",
        "s3:DeleteObjectTagging"
      ],
      "Resource": [
        "arn:aws:s3:::${eb_bucket}/*"
      ]
    }
  ]
}
JSON
  mc admin policy create "$admin_alias" "$eb_policy" "${work_dir}/policy-${eb_bucket}.json"

  # user add on an existing user just rotates the secret key
  mc admin user add "$admin_alias" "$eb_ak" "$eb_sk"

  # drop built-in global policies so the credential keeps only its bucket policy;
  # plain shell, not sed/grep: the minio image is UBI-micro and has neither
  eb_info="$(mc admin user info "$admin_alias" "$eb_ak" 2>/dev/null)" || eb_info=""
  for eb_canned in readwrite writeonly readonly; do
    case "$eb_info" in
      *"PolicyName:"*"${eb_canned}"*)
        mc admin policy detach "$admin_alias" "$eb_canned" --user "$eb_ak" >/dev/null
        echo "[minio] detached global policy ${eb_canned} from ${eb_ak}"
        ;;
    esac
  done

  mc admin policy attach "$admin_alias" "$eb_policy" --user "$eb_ak" >/dev/null

  echo "[minio] bucket ${eb_bucket} ready: $(mc anonymous get "${admin_alias}/${eb_bucket}" 2>&1)"
  echo "[minio] credential ${eb_ak} -> policy ${eb_policy} (${eb_bucket} only)"
}

ensure_bucket "$private_bucket" "$private_policy" "$private_ak" "$private_sk" none
ensure_bucket "$public_bucket" "$public_policy" "$public_ak" "$public_sk" object-read

echo "[minio] done: ${private_bucket} (anonymous none), ${public_bucket} (anonymous GetObject only), endpoint ${endpoint}"
