package main

import (
	"fmt"
	"platform/components"

	"github.com/pulumi/pulumi-aws/sdk/v7/go/aws"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// env deploys environment-specific resources (ECS clusters, target groups, listener rules) for dev, stag, and prod stacks.
func env(ctx *pulumi.Context, cfg *stackCfg, provider *aws.Provider) error {
	for _, cluster := range cfg.clusters {
		clusterName := fmt.Sprintf("bc-%s-cluster-%s", cluster.Name, ctx.Stack())

		ecsCluster, err := components.NewECSCluster(ctx, clusterName, &components.ECSClusterArgs{
			ClusterName:                pulumi.String(clusterName),
			CapacityProviderStrategies: cluster.CapacityProviderStrategies,
		}, pulumi.Provider(provider))
		if err != nil {
			return err
		}

		ctx.Export(fmt.Sprintf("%s.clusterName", cluster.Name), ecsCluster.ClusterName)
		ctx.Export(fmt.Sprintf("%s.clusterARN", cluster.Name), ecsCluster.ClusterARN)

		for _, svc := range cluster.Services {
			resourceName := fmt.Sprintf("bc-%s-%s-%s", cluster.Name, svc.Name, ctx.Stack())

			path := svc.Path
			if path == "" {
				path = "/*"
			}

			port := svc.Port
			if port == 0 {
				port = 80
			}

			healthCheckPath := svc.HealthCheckPath
			if healthCheckPath == "" {
				healthCheckPath = "/api/health"
			}

			healthCheckInterval := svc.HealthCheckInterval
			if healthCheckInterval == 0 {
				healthCheckInterval = 30
			}

			tg, err := components.NewTargetGroup(ctx, resourceName, &components.TargetGroupArgs{
				Name:                pulumi.String(resourceName),
				VpcID:               pulumi.String(cfg.vpcID),
				Port:                pulumi.Int(port),
				Protocol:            pulumi.String("HTTP"),
				HealthCheckPath:     pulumi.String(healthCheckPath),
				HealthCheckInterval: pulumi.Int(healthCheckInterval),
			}, pulumi.Provider(provider))
			if err != nil {
				return err
			}

			rule, err := components.NewListenerRule(ctx, resourceName, &components.ListenerRuleArgs{
				ListenerARN:    pulumi.String(cluster.ListenerARN),
				TargetGroupARN: tg.ARN,
				Host:           pulumi.String(svc.Host),
				Path:           pulumi.String(path),
				Priority:       pulumi.Int(svc.Priority),
			}, pulumi.Provider(provider))
			if err != nil {
				return err
			}

			ctx.Export(fmt.Sprintf("%s.%s.targetGroupARN", cluster.Name, svc.Name), tg.ARN)
			ctx.Export(fmt.Sprintf("%s.%s.listenerRuleARN", cluster.Name, svc.Name), rule.ARN)
		}
	}

	return nil
}
