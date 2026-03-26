bucket-name := 'ro-pulumi-state'

create-bucket:
  aws s3api create-bucket \
  --bucket {{ bucket-name }} \
  --region ap-southeast-1 \
  --create-bucket-configuration LocationConstraint=ap-southeast-1

enable-versioning:
  aws s3api put-bucket-versioning \
  --bucket {{ bucket-name }} \
  --versioning-configuration Status=Enabled

enable-encryption:
  aws s3api put-bucket-encryption \
  --bucket {{ bucket-name }} \
  --server-side-encryption-configuration file://server-side-encryption-configuration.json

block-public-access:
  aws s3api put-public-access-block \
  --bucket {{ bucket-name }} \
  --public-access-block-configuration file://public-access-block-configuration.json

add-tags:
  aws s3api put-bucket-tagging \
  --bucket {{ bucket-name }} \
  --tagging file://bucket-tags.json
