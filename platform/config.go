package main

import (
	"fmt"
	"platform/components"

	"github.com/pulumi/pulumi-aws/sdk/v7/go/aws"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

type serviceConfig struct {
	Name                string   `json:"name"`
	Hosts               []string `json:"hosts"`
	Paths               []string `json:"paths"`
	StripPathPrefix     bool     `json:"stripPathPrefix"`
	Priority            int      `json:"priority"`
	Port                int      `json:"port"`
	HealthCheckPath     string   `json:"healthCheckPath"`
	HealthCheckInterval int      `json:"healthCheckInterval"`
}

type clusterConfig struct {
	Name                       string                                `json:"name"`
	ListenerARN                string                                `json:"listenerARN"`
	Services                   []serviceConfig                       `json:"services"`
	CapacityProviderStrategies []components.CapacityProviderStrategy `json:"capacityProviderStrategies"`
}

type stackCfg struct {
	identity                 *aws.GetCallerIdentityResult
	azureCIUser              string
	createAzureUserAccessKey bool
	kmsARN                   string
	platformManagedBy        string
	platformSource           string
	clusters                 []clusterConfig
	vpcID                    string
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
		networkCfg := config.New(ctx, "network")
		sCfg.vpcID = networkCfg.Get("vpcId")
	}

	return sCfg, nil
}
