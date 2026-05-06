package main

import (
	"fmt"
	"platform/components"

	"github.com/pulumi/pulumi-aws/sdk/v7/go/aws"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

// env deploys environment-specific resources (ECS clusters, target groups, listener rules) for dev, stag, and prod stacks.
func env(ctx *pulumi.Context, cfg *stackCfg, provider *aws.Provider) error {
	// Separate provider for ECR — omits the Environment tag because ECR
	// repositories are shared across environments (one repo, tags per image).
	awsCfg := config.New(ctx, "aws")
	envAgnosticProvider, err := aws.NewProvider(ctx, "aws-ecr", &aws.ProviderArgs{
		Region: pulumi.String(awsCfg.Get("region")),
		DefaultTags: &aws.ProviderDefaultTagsArgs{
			Tags: pulumi.StringMap{
				"ManagedBy":      pulumi.String(cfg.platformManagedBy),
				"PlatformSource": pulumi.String(cfg.platformSource),
			},
		},
	})
	if err != nil {
		return fmt.Errorf("error creating ECR provider: %w", err)
	}

	// ECR repositories — one per unique service name across all clusters
	provisionedECR := map[string]bool{}
	for _, cluster := range cfg.clusters {
		for _, svc := range cluster.Services {
			if provisionedECR[svc.Name] {
				continue
			}
			provisionedECR[svc.Name] = true

			repo, err := components.NewECRRepository(ctx, svc.Name, &components.ECRRepositoryArgs{
				Name: pulumi.String(svc.Name),
			}, pulumi.Provider(envAgnosticProvider))
			if err != nil {
				return err
			}
			ctx.Export(fmt.Sprintf("%s.ecrURL", svc.Name), repo.URL)
			ctx.Export(fmt.Sprintf("%s.ecrARN", svc.Name), repo.ARN)
		}
	}

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

			paths := svc.Paths
			if len(paths) == 0 {
				paths = []string{"/*"}
			}

			if len(svc.Hosts) == 0 {
				return fmt.Errorf("service %q in cluster %q has no hosts configured", svc.Name, cluster.Name)
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
				Service:             pulumi.String(svc.Name),
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
				ListenerARN:     pulumi.String(cluster.ListenerARN),
				TargetGroupARN:  tg.ARN,
				Hosts:           pulumi.ToStringArray(svc.Hosts),
				Paths:           pulumi.ToStringArray(paths),
				StripPathPrefix: svc.StripPathPrefix,
				Priority:        pulumi.Int(svc.Priority),
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
