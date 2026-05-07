package main

import (
	"platform/composites"

	"github.com/pulumi/pulumi-aws/sdk/v7/go/aws"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

func env(ctx *pulumi.Context, cfg *stackCfg, provider *aws.Provider) error {
	awsCfg := config.New(ctx, "aws")
	return composites.ProvisionECS(ctx, composites.ProvisionECSArgs{
		Clusters:       cfg.clusters,
		VpcID:          cfg.vpcID,
		ManagedBy:      cfg.platformManagedBy,
		PlatformSource: cfg.platformSource,
		AWSRegion:      awsCfg.Get("region"),
	}, provider)
}
