mod bucket
mod pulumi
mod kms


mfa-delete:
  aws s3api put-bucket-versioning \
  --bucket bc-pulumi-state \
  --versioning-configuration Status=Enabled,MFADelete=Enabled \
  --mfa "arn:aws:iam::123456789012:mfa/root-device TOTP_CODE"
