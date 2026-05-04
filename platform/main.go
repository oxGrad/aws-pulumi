package main

import (
	"fmt"

	"github.com/pulumi/pulumi-aws/sdk/v7/go/aws"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		cfg, err := loadConfig(ctx)
		if err != nil {
			return err
		}

		// -------------------------------------------------------
		// 0. AWS Provider with Default Tags
		// -------------------------------------------------------
		awsCfg := config.New(ctx, "aws")
		provider, err := aws.NewProvider(ctx, "aws", &aws.ProviderArgs{
			Region: pulumi.String(awsCfg.Get("region")),
			DefaultTags: &aws.ProviderDefaultTagsArgs{
				Tags: pulumi.StringMap{
					"Environment":    pulumi.String(ctx.Stack()),
					"ManagedBy":      pulumi.String(cfg.platformManagedBy),
					"PlatformSource": pulumi.String(cfg.platformSource),
				},
			},
		})
		if err != nil {
			return fmt.Errorf("error creating aws provider: %w", err)
		}

		switch ctx.Stack() {
		case "shared":
			return shared(ctx, cfg, provider)
		default:
			return env(ctx, cfg, provider)
		}
	})
}
