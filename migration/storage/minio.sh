#!/bin/sh
#
# gonotelm bucket bootstrap. Idempotent: existing objects are never touched.
#   1. create the bucket (--ignore-existing keeps both the data and the settings of an
#      already existing bucket)
#   2. set the bucket access policy to private: anonymous read and write are both denied
#   3. create a dedicated credential from GONOTELM_MINIO_ACCESS_KEY /
#      GONOTELM_MINIO_SECRET_KEY that can only read and write the gonotelm bucket;
#      no global policy such as readwrite is attached
#
# Creating the user and the policy requires admin (root) credentials, taken from
# GONOTELM_DEV_MINIO_ROOT_USER / GONOTELM_DEV_MINIO_ROOT_PASSWORD, default minioadmin
# (matches MINIO_ROOT_* of the minio service in deploy/dev/docker-compose.yaml).
#
# Environment:
#   GONOTELM_MINIO_ACCESS_KEY        required, access key of the bucket credential (same
#                                    variable the app reads)
#   GONOTELM_MINIO_SECRET_KEY        required, secret key of the bucket credential
#   GONOTELM_DEV_MINIO_ROOT_USER     admin user, default minioadmin
#   GONOTELM_DEV_MINIO_ROOT_PASSWORD admin password, default minioadmin
#   GONOTELM_MINIO_ENDPOINT          default 127.0.0.1:9000; the container passes minio:9000
#   GONOTELM_MINIO_BUCKET            default gonotelm
#   GONOTELM_MINIO_SECURE            use https when true, default false
#   GONOTELM_MINIO_POLICY_NAME       name of the bucket policy, default gonotelm-rw
#
# On the host: set -a && . ./.env && set +a && migration/storage/minio.sh
# In a container: the minio-init compose service mounts this file and runs it with
# /bin/sh, so both entry points share exactly the same logic.

set -eu

: "${GONOTELM_MINIO_ACCESS_KEY:?GONOTELM_MINIO_ACCESS_KEY is not set (access key of the bucket credential)}"
: "${GONOTELM_MINIO_SECRET_KEY:?GONOTELM_MINIO_SECRET_KEY is not set (secret key of the bucket credential)}"

root_user="${GONOTELM_DEV_MINIO_ROOT_USER:-minioadmin}"
root_password="${GONOTELM_DEV_MINIO_ROOT_PASSWORD:-minioadmin}"
access_key="$GONOTELM_MINIO_ACCESS_KEY"
secret_key="$GONOTELM_MINIO_SECRET_KEY"
bucket="${GONOTELM_MINIO_BUCKET:-gonotelm}"
policy_name="${GONOTELM_MINIO_POLICY_NAME:-gonotelm-rw}"
admin_alias="gonotelm-admin"

# GONOTELM_MINIO_ENDPOINT is a bare host:port, add the scheme based on GONOTELM_MINIO_SECURE
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

if [ "$access_key" = "$root_user" ]; then
  echo "[minio] GONOTELM_MINIO_ACCESS_KEY must not be the admin ${root_user}: the bucket needs its own credential" >&2
  exit 1
fi

# Use a throwaway MC_CONFIG_DIR so the caller's aliases in ~/.mc are left alone
work_dir="$(mktemp -d)"
MC_CONFIG_DIR="${work_dir}/mc"
export MC_CONFIG_DIR
trap 'rm -rf "$work_dir"' 0 1 2 15

mc alias set "$admin_alias" "$endpoint" "$root_user" "$root_password" >/dev/null

# An access key already used by a service account cannot become a regular user; MinIO only
# answers with a confusing "Credential is not allowed to be same as admin access key".
if mc admin user svcacct info "$admin_alias" "$access_key" >/dev/null 2>&1; then
  echo "[minio] ${access_key} is already an existing service account, pick a fresh GONOTELM_MINIO_ACCESS_KEY" >&2
  echo "[minio] or drop the old one first: mc admin user svcacct rm <alias> ${access_key}" >&2
  exit 1
fi

# 1. bucket: keeps the data when it already exists
mc mb --ignore-existing "${admin_alias}/${bucket}"

# 2. bucket access policy: private (no anonymous read, no anonymous write)
mc anonymous set none "${admin_alias}/${bucket}"

# 3. bucket-scoped read/write policy: the resources cover this bucket only
cat > "${work_dir}/policy.json" <<JSON
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
        "arn:aws:s3:::${bucket}"
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
        "arn:aws:s3:::${bucket}/*"
      ]
    }
  ]
}
JSON
mc admin policy create "$admin_alias" "$policy_name" "${work_dir}/policy.json"

# 4. the bucket's own credential: for an existing user, add updates the secret key,
#    so repeating the script also rotates it
mc admin user add "$admin_alias" "$access_key" "$secret_key"

# 5. drop the built-in global policies (readwrite can read and write every bucket) so the
#    credential ends up with the bucket policy only. The check is plain shell on purpose:
#    the minio image is UBI-micro and ships neither sed nor grep.
user_info="$(mc admin user info "$admin_alias" "$access_key" 2>/dev/null)" || user_info=""
for canned in readwrite writeonly readonly; do
  case "$user_info" in
    *"PolicyName:"*"${canned}"*)
      mc admin policy detach "$admin_alias" "$canned" --user "$access_key" >/dev/null
      echo "[minio] detached global policy ${canned} (${access_key} keeps only ${policy_name})"
      ;;
  esac
done

mc admin policy attach "$admin_alias" "$policy_name" --user "$access_key" >/dev/null

echo "[minio] bucket ${bucket} ready: $(mc anonymous get "${admin_alias}/${bucket}" 2>&1)"
echo "[minio] credential ${access_key} has read/write on ${bucket} only (policy ${policy_name}), endpoint ${endpoint}"
