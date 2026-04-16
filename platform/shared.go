package main

import (
	"platform/components"

	"github.com/pulumi/pulumi-aws/sdk/v7/go/aws"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// shared deploys environment-agnostic resources: IAM users, roles, etc.
// These resources are shared across all environments and managed under the "shared" stack.
func shared(ctx *pulumi.Context, cfg *stackCfg, provider *aws.Provider) error {
	// -------------------------------------------------------
	// IAM User: azure-ci
	// -------------------------------------------------------
	_, err := components.NewAzureCIUser(ctx, "azure-ci", &components.AzureCIUserArgs{
		User:   pulumi.String(cfg.azureCIUser),
		KmsARN: pulumi.String(cfg.kmsARN),
	}, pulumi.Provider(provider))
	if err != nil {
		return err
	}

	return nil
}
