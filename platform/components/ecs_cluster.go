package components

import (
	"fmt"

	"github.com/pulumi/pulumi-aws/sdk/v7/go/aws/cloudwatch"
	"github.com/pulumi/pulumi-aws/sdk/v7/go/aws/ecs"
	"github.com/pulumi/pulumi-aws/sdk/v7/go/aws/servicediscovery"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type ECSCluster struct {
	pulumi.ResourceState
	ClusterName              pulumi.StringOutput
	ClusterARN               pulumi.StringOutput
	ServiceConnectNamespace  pulumi.StringOutput
}

type CapacityProviderStrategy struct {
	CapacityProvider string `json:"capacityProvider"`
	Weight           int    `json:"weight"`
	Base             int    `json:"base"`
}

type ECSClusterArgs struct {
	ClusterName                pulumi.StringInput
	CapacityProviderStrategies []CapacityProviderStrategy
	ServiceConnectNamespace    string // e.g. "dev.internal", "stag.internal", "internal"
}

func NewECSCluster(ctx *pulumi.Context, name string, args *ECSClusterArgs, opts ...pulumi.ResourceOption) (*ECSCluster, error) {
	self := &ECSCluster{}
	err := ctx.RegisterComponentResource("platform:ecs:cluster", name, self, opts...)
	if err != nil {
		return nil, err
	}

	execLogGroup, err := cloudwatch.NewLogGroup(ctx, fmt.Sprintf("%s-exec-logs", name), &cloudwatch.LogGroupArgs{
		Name:            pulumi.Sprintf("/ecs/%s/exec", args.ClusterName),
		RetentionInDays: pulumi.Int(7),
	}, pulumi.Parent(self))
	if err != nil {
		return nil, fmt.Errorf("error creating exec command log group: %w", err)
	}

	clusterArgs := &ecs.ClusterArgs{
		Name: args.ClusterName,
		Settings: ecs.ClusterSettingArray{
			&ecs.ClusterSettingArgs{
				Name:  pulumi.String("containerInsights"),
				Value: pulumi.String("disabled"),
			},
		},
		Configuration: &ecs.ClusterConfigurationArgs{
			ExecuteCommandConfiguration: &ecs.ClusterConfigurationExecuteCommandConfigurationArgs{
				Logging: pulumi.String("OVERRIDE"),
				LogConfiguration: &ecs.ClusterConfigurationExecuteCommandConfigurationLogConfigurationArgs{
					CloudWatchLogGroupName:      execLogGroup.Name,
					CloudWatchEncryptionEnabled: pulumi.Bool(false),
				},
			},
		},
		Tags: pulumi.StringMap{
			"Name": args.ClusterName,
		},
	}

	if args.ServiceConnectNamespace != "" {
		ns, err := servicediscovery.NewHttpNamespace(ctx, fmt.Sprintf("%s-sc-namespace", name), &servicediscovery.HttpNamespaceArgs{
			Name: pulumi.String(args.ServiceConnectNamespace),
		}, pulumi.Parent(self))
		if err != nil {
			return nil, fmt.Errorf("error creating service connect namespace: %w", err)
		}
		clusterArgs.ServiceConnectDefaults = &ecs.ClusterServiceConnectDefaultsArgs{
			Namespace: ns.Arn,
		}
		self.ServiceConnectNamespace = ns.Arn
	}

	cluster, err := ecs.NewCluster(ctx, name, clusterArgs, pulumi.Parent(self))
	if err != nil {
		return nil, fmt.Errorf("error creating ECS cluster: %w", err)
	}

	strategies := args.CapacityProviderStrategies
	if len(strategies) == 0 {
		strategies = []CapacityProviderStrategy{
			{CapacityProvider: "FARGATE", Weight: 1, Base: 1},
			{CapacityProvider: "FARGATE_SPOT", Weight: 4, Base: 0},
		}
	}

	seen := map[string]bool{}
	var capacityProviders pulumi.StringArray
	var defaultStrategies ecs.ClusterCapacityProvidersDefaultCapacityProviderStrategyArray
	for _, s := range strategies {
		if !seen[s.CapacityProvider] {
			seen[s.CapacityProvider] = true
			capacityProviders = append(capacityProviders, pulumi.String(s.CapacityProvider))
		}
		defaultStrategies = append(defaultStrategies, &ecs.ClusterCapacityProvidersDefaultCapacityProviderStrategyArgs{
			CapacityProvider: pulumi.String(s.CapacityProvider),
			Weight:           pulumi.Int(s.Weight),
			Base:             pulumi.Int(s.Base),
		})
	}

	_, err = ecs.NewClusterCapacityProviders(ctx, fmt.Sprintf("%s-capacity", name), &ecs.ClusterCapacityProvidersArgs{
		ClusterName:                       cluster.Name,
		CapacityProviders:                 capacityProviders,
		DefaultCapacityProviderStrategies: defaultStrategies,
	}, pulumi.Parent(self))
	if err != nil {
		return nil, fmt.Errorf("error setting cluster capacity providers: %w", err)
	}

	self.ClusterName = cluster.Name
	self.ClusterARN = cluster.Arn

	outputs := pulumi.Map{
		"ClusterName": self.ClusterName,
		"ClusterARN":  self.ClusterARN,
	}
	if args.ServiceConnectNamespace != "" {
		outputs["ServiceConnectNamespace"] = self.ServiceConnectNamespace
	}
	_ = ctx.RegisterResourceOutputs(self, outputs)

	return self, nil
}
