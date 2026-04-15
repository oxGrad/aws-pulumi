package main

import (
	"github.com/pulumi/pulumi-aws/sdk/v7/go/aws"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

type stackCfg struct {
	identity          *aws.GetCallerIdentityResult
	azureCIUser       string
	kmsARN            string
	platformManagedBy string
	platformSource    string
}

func loadConfig(ctx *pulumi.Context) (*stackCfg, error) {
	var (
		sCfg = &stackCfg{}
		err  error
	)

	{
		platformCfg := config.New(ctx, "platform")
		sCfg.platformManagedBy = platformCfg.Get("managedby")
		sCfg.platformSource = platformCfg.Get("source")
	}

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
		sCfg.kmsARN = ci.Get("kmsARN") // NOTE: To push azure-ci user access_key_id and access_key_secret to secret manager.
	}

	return sCfg, nil
}
