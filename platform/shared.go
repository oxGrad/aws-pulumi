package main

import (
	"platform/components"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// shared deploys environment-agnostic resources: IAM users, roles, etc.
// These resources are shared across all environments and managed under the "shared" stack.
func shared(ctx *pulumi.Context, cfg *stackCfg) error {
	// -------------------------------------------------------
	// IAM User: azure-ci
	// -------------------------------------------------------
	_, err := components.NewAzureCIUser(ctx, "azure-ci", &components.AzureCIUserArgs{
		User:              pulumi.String(cfg.azureCIUser),
		KmsARN:            pulumi.String(cfg.kmsARN),
		PlatformManagedBy: pulumi.String(cfg.platformManagedBy),
		PlatformSource:    pulumi.String(cfg.platformSource),
	})
	if err != nil {
		return err
	}

	return nil
}
