package main

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// env deploys environment-specific resources for dev, stag, and prod stacks.
func env(ctx *pulumi.Context, cfg *stackCfg) error {
	_ = cfg
	_ = ctx
	return nil
}
