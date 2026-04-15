package main

import (
	"github.com/pulumi/pulumi-aws/sdk/v7/go/aws"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

type stackCfg struct {
	identity           *aws.GetCallerIdentityResult
	bootstrapManagedBy string
	bootstrapSource    string
	bucketName         string
	ciUser             string
	ciRole             string
	kmsName            string
}

func loadConfig(ctx *pulumi.Context) (*stackCfg, error) {
	var (
		sCfg = &stackCfg{}
		err  error
	)

	{
		bootstrapCfg := config.New(ctx, "bootstrap")
		sCfg.bootstrapManagedBy = bootstrapCfg.Get("managedby")
		sCfg.bootstrapSource = bootstrapCfg.Get("source")
	}

	// Fetch account ID dynamically — no config needed
	sCfg.identity, err = aws.GetCallerIdentity(ctx, nil, nil)
	if err != nil {
		_ = ctx.Log.Error("error getting caller identity: %w", nil)
		return nil, err
	}

	{
		bucketCfg := config.New(ctx, "bucket")
		sCfg.bucketName = bucketCfg.Get("name")
	}

	{
		ciCfg := config.New(ctx, "ci")
		sCfg.ciUser = ciCfg.Get("user")
		sCfg.ciRole = ciCfg.Get("role")
	}

	{
		kmsCfg := config.New(ctx, "kms")
		sCfg.kmsName = kmsCfg.Get("name")
	}

	return sCfg, nil
}
