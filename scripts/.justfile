mod bucket
mod pulumi
mod kms


mfa-delete:
  aws s3api put-bucket-versioning \
  --bucket bc-pulumi-state \
  --versioning-configuration Status=Enabled,MFADelete=Enabled \
  --mfa "arn:aws:iam::626883896657:mfa/root-device TOTP_CODE"
