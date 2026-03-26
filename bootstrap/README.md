# Pulumi Bootstrap (Go)

This project provisions the foundational infrastructure required to manage Pulumi state in AWS. It creates a secure S3 bucket with KMS encryption and IAM identities for CI/CD operations.

## Features

- **KMS Key**: Dedicated AWS KMS key for encrypting the Pulumi state bucket with rotation enabled.
- **S3 Bucket**: Versioned, private S3 bucket configured with public access blocks and KMS encryption.
- **IAM Executor Role**: A scoped IAM role that Pulumi CI/CD processes assume to perform infrastructure changes.
- **IAM Executor Policy**: Least-privilege permissions granted to the executor role for S3 and KMS access.
- **IAM CI User**: A dedicated IAM user for CI/CD runners with permissions to assume the executor role.
- **KMS Key Policy**: Securely manages access to the KMS key, ensuring the executor role can use it for encryption/decryption.

## Prerequisites

- [Go 1.25+](https://go.dev/dl/)
- [Pulumi CLI](https://www.pulumi.com/docs/get-started/install/)
- AWS credentials configured (e.g., via `aws configure`)

## Architecture Overview

```
┌─────────────────────────────────────────────┐
│                AWS Account                  │
│                                             │
│  ┌──────────────┐    ┌───────────────────┐  │
│  │  KMS Key     │    │  S3 Bucket        │  │
│  │  pulumi-     │◄───│  bc-pulumi-state  │  │
│  │  state-key   │    │  (versioned)      │  │
│  └──────┬───────┘    └───────────────────┘  │
│         │                     ▲             │
│         │                     │             │
│  ┌──────▼────────────────────┐│             │
│  │  IAM Role: pulumi-executor││             │
│  │  - S3 read/write          ├┘             │
│  │  - KMS encrypt/decrypt    │              │
│  └──────────────▲────────────┘              │
│                 │ sts:AssumeRole            │
│  ┌──────────────┴───────────┐               │
│  │  IAM User: pulumi-ci     │               │
│  └──────────────────────────┘               │
└─────────────────────────────────────────────┘

iam.NewAccessKey()
        │
        │  accessKey.Id + accessKey.Secret
        ▼
secretsmanager.NewSecretVersion()
        │
        │
        ▼
  secret ARN (exported)
        │
        ▼
  aws secretsmanager
  get-secret-value
```

## Configuration

The project requires the following configuration values:

| Key              | Description                                                     |
| ---------------- | --------------------------------------------------------------- |
| `bootstrap:user` | Name of the administrator or system provisioning the bootstrap. |
| `bucket:name`    | The unique name for the S3 state bucket.                        |
| `ci:user`        | The name for the IAM CI user.                                   |
| `ci:role`        | The name for the IAM Executor role.                             |
| `kms:name`       | The name/alias for the KMS key.                                 |

Example setup:

```bash
pulumi config set bootstrap:user admin-name
pulumi config set bucket:name my-company-pulumi-state
pulumi config set ci:user pulumi-ci
pulumi config set ci:role pulumi-executor
pulumi config set kms:name pulumi-state-key
```

## Getting Started

1. **Install dependencies**:

   ```bash
   go mod download
   ```

2. **Preview changes**:

   ```bash
   pulumi preview
   ```

3. **Deploy the stack**:

   ```bash
   pulumi up
   ```

## Project Structure

- `main.go`: Orchestrates the creation of all bootstrap components.
- `components/`: Contains modular Pulumi components:
  - `bucket.go`: S3 bucket resource definition.
  - `bucket_key.go`: KMS key resource definition.
  - `executor_role.go`: IAM role for Pulumi execution.
  - `executor_policy.go`: Permissions for the executor role.
  - `key_policy.go`: KMS access control policy.
  - `user.go`: IAM user for CI/CD and role assumption policy.

## Outputs

- `bucketID`: The ID (name) of the created S3 bucket.

## Next Steps

After deploying this bootstrap stack, you can configure other Pulumi projects to use this S3 bucket as their backend:

```bash
pulumi login s3://<bucket-name>
```

Ensure your CI/CD environment uses the created IAM CI user and assumes the Executor role.
