create-key:
  aws kms create-key \
    --description "Pulumi state bucket encryption key"

create-alias:
  aws kms create-alias \
  --alias-name alias/pulumi-state-key \
  --target-key-id <KEY_ID>
