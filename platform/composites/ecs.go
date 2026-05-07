package composites

import (
	"fmt"
	"platform/components"
	"strings"

	"github.com/pulumi/pulumi-aws/sdk/v7/go/aws"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type ServiceConfig struct {
	Name                string   `json:"name"`
	Hosts               []string `json:"hosts"`
	Paths               []string `json:"paths"`
	StripPathPrefix     bool     `json:"stripPathPrefix"`
	Priority            int      `json:"priority"`
	Port                int      `json:"port"`
	HealthCheckPath     string   `json:"healthCheckPath"`
	HealthCheckInterval int      `json:"healthCheckInterval"`
}

type ClusterConfig struct {
	Name                       string                                `json:"name"`
	ListenerARN                string                                `json:"listenerARN"`
	Services                   []ServiceConfig                       `json:"services"`
	CapacityProviderStrategies []components.CapacityProviderStrategy `json:"capacityProviderStrategies"`
}

type ProvisionECSArgs struct {
	Clusters       []ClusterConfig
	VpcID          string
	ManagedBy      string
	PlatformSource string
	AWSRegion      string
}

func ProvisionECS(ctx *pulumi.Context, args ProvisionECSArgs, provider *aws.Provider) error {
	envAgnosticProvider, err := aws.NewProvider(ctx, "aws-ecr", &aws.ProviderArgs{
		Region: pulumi.String(args.AWSRegion),
		DefaultTags: &aws.ProviderDefaultTagsArgs{
			Tags: pulumi.StringMap{
				"ManagedBy":      pulumi.String(args.ManagedBy),
				"PlatformSource": pulumi.String(args.PlatformSource),
			},
		},
	})
	if err != nil {
		return fmt.Errorf("error creating ECR provider: %w", err)
	}

	// ECR repositories — one per unique service name across all clusters
	provisionedECR := map[string]bool{}
	for _, cluster := range args.Clusters {
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

	for _, cluster := range args.Clusters {
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

			if len(svc.Hosts) == 0 {
				return fmt.Errorf("service %q in cluster %q has no hosts configured", svc.Name, cluster.Name)
			}

			ps, err := provisionService(ctx, resourceName, svc, cluster.ListenerARN, args.VpcID, provider)
			if err != nil {
				return err
			}

			ctx.Export(fmt.Sprintf("%s.%s.targetGroupARN", cluster.Name, svc.Name), ps.targetGroupARN)
			ctx.Export(fmt.Sprintf("%s.%s.listenerRuleARN", cluster.Name, svc.Name), ps.listenerRuleARN)
		}
	}

	return nil
}

type provisionedService struct {
	targetGroupARN  pulumi.StringOutput
	listenerRuleARN pulumi.StringOutput
}

func provisionService(ctx *pulumi.Context, name string, svc ServiceConfig, listenerARN, vpcID string, provider *aws.Provider) (*provisionedService, error) {
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

	paths := svc.Paths
	if len(paths) == 0 {
		paths = []string{"/*"}
	}

	tg, err := components.NewTargetGroup(ctx, name, &components.TargetGroupArgs{
		Name:                pulumi.String(name),
		Service:             pulumi.String(fmt.Sprintf("%s-%s", svc.Name, ctx.Stack())),
		VpcID:               pulumi.String(vpcID),
		Port:                pulumi.Int(port),
		Protocol:            pulumi.String("HTTP"),
		HealthCheckPath:     pulumi.String(healthCheckPath),
		HealthCheckInterval: pulumi.Int(healthCheckInterval),
	}, pulumi.Provider(provider))
	if err != nil {
		return nil, err
	}

	var pathPrefix string
	if svc.StripPathPrefix && len(paths) > 0 {
		pathPrefix = strings.TrimSuffix(paths[0], "/*")
	}

	rule, err := components.NewListenerRule(ctx, name, &components.ListenerRuleArgs{
		ListenerARN:     pulumi.String(listenerARN),
		TargetGroupARN:  tg.ARN,
		Hosts:           pulumi.ToStringArray(svc.Hosts),
		Paths:           pulumi.ToStringArray(paths),
		StripPathPrefix: svc.StripPathPrefix,
		PathPrefix:      pathPrefix,
		Priority:        pulumi.Int(svc.Priority),
	}, pulumi.Provider(provider))
	if err != nil {
		return nil, err
	}

	return &provisionedService{
		targetGroupARN:  tg.ARN,
		listenerRuleARN: rule.ARN,
	}, nil
}
