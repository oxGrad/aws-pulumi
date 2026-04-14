package main

import (
	"github.com/pulumi/pulumi-aws/sdk/v7/go/aws"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

type stackCfg struct {
	identity    *aws.GetCallerIdentityResult
	azureCIUser string
}

func loadConfig(ctx *pulumi.Context) (*stackCfg, error) {
	var (
		sCfg = &stackCfg{}
		err  error
	)

	// Fetch account ID dynamically — no config needed
	sCfg.identity, err = aws.GetCallerIdentity(ctx, nil, nil)
	if err != nil {
		_ = ctx.Log.Error("error getting caller identity: %w", nil)
		return nil, err
	}

	switch ctx.Stack() {
	case "shared":
		ci := config.New(ctx, "ci")
		sCfg.azureCIUser = ci.Get("azureUser")
	}

	return sCfg, nil
}
