set dotenv-load := true

login:
  pulumi login s3://bc-sre-pulumi-state

new-stack-dev:
  just new-stack dev

new-stack-prod:
  just new-stack prod

new-stack env:
  pulumi stack init organization/{{ env }}

lint:
  golangci-lint run

preview-ci-shared:
  PULUMI_CONFIG_PASSPHRASE=$PULUMI_CONFIG_PASSPHRASE_SHARED just preview-ci shared

preview-ci-dev:
  PULUMI_CONFIG_PASSPHRASE=$PULUMI_CONFIG_PASSPHRASE_DEV just preview-ci dev

preview-ci stack="shared":
  #!/usr/bin/env bash
  set -euo pipefail
  CREDS=$(aws sts assume-role \
    --role-arn "arn:aws:iam::${AWS_ACCOUNT_ID}:role/pulumi-executor" \
    --role-session-name "pulumi-local-preview" \
    --duration-seconds "900" \
    --query 'Credentials.[AccessKeyId,SecretAccessKey,SessionToken]' \
    --output text)
  export AWS_ACCESS_KEY_ID=$(echo $CREDS | awk '{print $1}')
  export AWS_SECRET_ACCESS_KEY=$(echo $CREDS | awk '{print $2}')
  export AWS_SESSION_TOKEN=$(echo $CREDS | awk '{print $3}')
  pulumi preview --stack {{ stack }} --non-interactive --diff

