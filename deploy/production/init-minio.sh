#!/bin/sh
set -eu

policy_file=/tmp/sangyu-app-policy.json

mc alias set production http://minio:9000 "$MINIO_ROOT_USER" "$MINIO_ROOT_PASSWORD"
mc mb --ignore-existing "production/$S3_BUCKET"
mc anonymous set none "production/$S3_BUCKET"

cat > "$policy_file" <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": ["s3:GetBucketLocation", "s3:ListBucket"],
      "Resource": ["arn:aws:s3:::$S3_BUCKET"]
    },
    {
      "Effect": "Allow",
      "Action": ["s3:GetObject", "s3:PutObject", "s3:DeleteObject"],
      "Resource": ["arn:aws:s3:::$S3_BUCKET/*"]
    }
  ]
}
EOF

mc admin user add production "$S3_ACCESS_KEY" "$S3_SECRET_KEY"
mc admin policy detach production sangyu-app --user "$S3_ACCESS_KEY" >/dev/null 2>&1 || true
mc admin policy remove production sangyu-app >/dev/null 2>&1 || true
mc admin policy create production sangyu-app "$policy_file"
mc admin policy attach production sangyu-app --user "$S3_ACCESS_KEY"
