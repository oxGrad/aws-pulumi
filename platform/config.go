package main

import (
	"fmt"
	"platform/composites"

	"github.com/pulumi/pulumi-aws/sdk/v7/go/aws"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

type stackCfg struct {
	identity                 *aws.GetCallerIdentityResult
	awsRegion                string
	azureCIUser              string
	createAzureUserAccessKey bool
	kmsARN                   string
	platformManagedBy        string
	platformSource           string
	clusters                 []composites.ClusterConfig
	serviceConnectNamespaces []string
	vpcID                    string
	notifications            composites.NotificationsConfig
}

func loadConfig(ctx *pulumi.Context) (*stackCfg, error) {
	var (
		sCfg = &stackCfg{}
		err  error
	)

	sCfg.identity, err = aws.GetCallerIdentity(ctx, nil, nil)
	if err != nil {
		_ = ctx.Log.Error("error getting caller identity: %w", nil)
		return nil, err
	}

	{
		platformCfg := config.New(ctx, "platform")
		sCfg.platformManagedBy = platformCfg.Get("managedby")
		sCfg.platformSource = platformCfg.Get("source")
	}

	sCfg.awsRegion = config.New(ctx, "aws").Get("region")

	switch ctx.Stack() {
	case "shared":
		ci := config.New(ctx, "ci")
		sCfg.azureCIUser = ci.Get("azureUser")
		sCfg.createAzureUserAccessKey = ci.GetBool("createAzureUserAccessKey")
		sCfg.kmsARN = ci.Get("kmsARN")
	default:
		clusterCfg := config.New(ctx, "cluster")
		if err := clusterCfg.GetObject("clusters", &sCfg.clusters); err != nil {
			return nil, fmt.Errorf("cluster:clusters config is required for env stacks: %w", err)
		}
		// Optional: list of service connect namespace names to provision.
		_ = clusterCfg.GetObject("serviceConnect", &sCfg.serviceConnectNamespaces)
		networkCfg := config.New(ctx, "network")
		sCfg.vpcID = networkCfg.Get("vpcId")

		notifCfg := config.New(ctx, "notifications")
		// Error is intentionally ignored — notifications are optional.
		_ = notifCfg.GetObject("endpoints", &sCfg.notifications)
	}

	return sCfg, nil
}
