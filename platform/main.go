package main

import (
	"fmt"

	"github.com/pulumi/pulumi-aws/sdk/v7/go/aws"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
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
		provider, err := aws.NewProvider(ctx, "aws", &aws.ProviderArgs{
			DefaultTags: &aws.ProviderDefaultTagsArgs{
				Tags: pulumi.StringMap{
					"Environment":     pulumi.String(ctx.Stack()),
					"ManagedBy":       pulumi.String(cfg.bootstrapManagedBy),
					"BootstrapSource": pulumi.String(cfg.bootstrapSource),
				},
			},
		})
		if err != nil {
			return fmt.Errorf("error creating aws provider: %w", err)
		}

		switch ctx.Stack() {
		case "shared":
			return shared(ctx, cfg)
		default:
			return env(ctx, cfg)
		}
	})
}
