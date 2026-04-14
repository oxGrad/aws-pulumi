package main

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		cfg, err := loadConfig(ctx)
		if err != nil {
			return err
		}

		switch ctx.Stack() {
		case "shared":
			return shared(ctx, cfg)
		default:
			return env(ctx, cfg)
		}
	})
}
