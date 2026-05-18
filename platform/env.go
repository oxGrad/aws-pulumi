package main

import (
	"fmt"
	"platform/composites"

	"github.com/pulumi/pulumi-aws/sdk/v7/go/aws"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

func env(ctx *pulumi.Context, cfg *stackCfg, provider *aws.Provider) error {
	if err := composites.ProvisionServiceConnect(ctx, cfg.serviceConnectNamespaces, provider); err != nil {
		return err
	}

	awsCfg := config.New(ctx, "aws")
	result, err := composites.ProvisionECS(ctx, composites.ProvisionECSArgs{
		Clusters:       cfg.clusters,
		VpcID:          cfg.vpcID,
		ManagedBy:      cfg.platformManagedBy,
		PlatformSource: cfg.platformSource,
		AWSRegion:      awsCfg.Get("region"),
		AccountID:      cfg.identity.AccountId,
	}, provider)
	if err != nil {
		return err
	}

	notifs, err := composites.ProvisionNotifications(ctx, cfg.notifications, provider)
	if err != nil {
		return err
	}

	for _, cluster := range cfg.clusters {
		clusterServices, ok := result.Clusters[cluster.Name]
		if !ok {
			continue
		}
		ecsClusterName := fmt.Sprintf("bc-%s-cluster-%s", cluster.Name, ctx.Stack())

		for _, svc := range cluster.Services {
			if svc.Monitoring == nil || !svc.Monitoring.Enabled {
				continue
			}
			ps, ok := clusterServices[svc.Name]
			if !ok {
				continue
			}
			resourceName := fmt.Sprintf("bc-%s-%s-%s", cluster.Name, svc.Name, ctx.Stack())
			ecsServiceName := fmt.Sprintf("%s-%s", svc.Name, ctx.Stack())

			if err := composites.ProvisionServiceMonitoring(ctx, resourceName, composites.ServiceMonitoringArgs{
				Config:          *svc.Monitoring,
				ECSClusterName:  ecsClusterName,
				ECSServiceName:  ecsServiceName,
				ListenerARN:     ps.ListenerARN,
				TargetGroupARN:  ps.TargetGroupARN,
				CriticalActions: pulumi.Array{notifs.CriticalTopicARN},
				WarningActions:  pulumi.Array{notifs.WarningTopicARN},
			}, provider); err != nil {
				return err
			}
		}
	}

	return nil
}
