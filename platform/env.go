package main

import (
	"github.com/pulumi/pulumi-aws/sdk/v7/go/aws"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// env deploys environment-specific resources for dev, stag, and prod stacks.
func env(ctx *pulumi.Context, cfg *stackCfg, provider *aws.Provider) error {
	_ = cfg
	_ = ctx
	_ = provider
	return nil
}
