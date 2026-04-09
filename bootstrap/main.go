package main

import (
	"bootstrap/components"
	"fmt"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		// Load stack configuration
		cfg, err := loadConfig(ctx)
		if err != nil {
			return err
		}

		// -------------------------------------------------------
		// 1. KMS Key for S3 bucket encryption
		// -------------------------------------------------------
		stateKey, err := components.NewKeyAndAlias(ctx, "pulumi-key", components.KeyAndAliasArgs{
			KmsName:       pulumi.String(cfg.kmsName),
			BootstrapUser: pulumi.String(cfg.bootstrapUser),
		}, nil)
		if err != nil {
			return err
		}

		// -------------------------------------------------------
		// 2. S3 Bucket: bc-pulumi-state
		// -------------------------------------------------------
		bucket, err := components.NewStateBucket(ctx, cfg.bucketName, &components.StateBucketArgs{
			BucketName:    pulumi.String(cfg.bucketName),
			BootstrapUser: pulumi.String(cfg.bootstrapUser),
		}, nil)
		if err != nil {
			return err
		}

		// -------------------------------------------------------
		// 3. IAM Role: pulumi-executor
		// -------------------------------------------------------
		executerRole, err := components.NewExecutorRole(ctx, cfg.ciRole, &components.ExecutorRoleArgs{
			AWSAccountID:  pulumi.String(cfg.identity.AccountId),
			Role:          pulumi.String(cfg.ciRole),
			BootstrapUser: pulumi.String(cfg.bootstrapUser),
		}, nil)
		if err != nil {
			return err
		}

		// -------------------------------------------------------
		// 4. IAM Policy: access to S3 bucket and KMS key
		// -------------------------------------------------------
		_, err = components.NewExecutorPolicy(ctx, fmt.Sprintf("%s-policy", cfg.ciRole), &components.ExecutorPolicyArgs{
			BucketARN:     bucket.ARN,
			KmsARN:        stateKey.KmsARN,
			RoleName:      pulumi.String(cfg.ciRole),
			BootstrapUser: pulumi.String(cfg.bootstrapUser),
			AdditionalRole: &components.ExecutorPolicyAdditionalRole{
				S3Admin: true,
			},
		}, nil)
		if err != nil {
			return err
		}

		// -------------------------------------------------------
		// 5. KMS Key Policy — grant role access to the key
		// -------------------------------------------------------
		_, err = components.NewKeyPolicy(ctx, fmt.Sprintf("%s-policy", cfg.kmsName), &components.KeyPolicyArgs{
			AWSAccountID:    pulumi.String(cfg.identity.AccountId),
			ExecutorRoleARN: executerRole.ARN,
			KmsName:         pulumi.String(cfg.kmsName),
			KmsKeyID:        stateKey.KmsID,
			BootstrapUser:   pulumi.String(cfg.bootstrapUser),
		}, nil)
		if err != nil {
			return err
		}

		// -------------------------------------------------------
		// 6. IAM User: pulumi-ci
		// -------------------------------------------------------
		_, err = components.NewCIUser(ctx, cfg.ciUser, &components.CIUserArgs{
			User:            pulumi.String(cfg.ciUser),
			ExecutorRoleARN: executerRole.ARN,
			BootstrapUser:   pulumi.String(cfg.bootstrapUser),
		}, nil)
		if err != nil {
			return err
		}

		// -------------------------------------------------------
		// 7. Secret Manager: pulumi-ci
		// -------------------------------------------------------
		accessKey, err := components.NewPushAccessKeyToSecretManager(ctx, cfg.ciUser, &components.PushAccessKeyToSecretManagerArgs{
			User:          pulumi.String(cfg.ciUser),
			KmsARN:        stateKey.KmsARN,
			BootstrapUser: pulumi.String(cfg.bootstrapUser),
		}, nil)
		if err != nil {
			return err
		}

		// -------------------------------------------------------
		// 7. Outputs
		// -------------------------------------------------------
		ctx.Export("bucketID", bucket.ID)
		ctx.Export("bucketARN", bucket.ARN)
		ctx.Export("pulumiCICredentialsArn", accessKey.ARN)

		return nil
	})
}
