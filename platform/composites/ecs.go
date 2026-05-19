package composites

import (
	"fmt"
	"platform/components"
	"strings"

	"github.com/pulumi/pulumi-aws/sdk/v7/go/aws"
	"github.com/pulumi/pulumi-aws/sdk/v7/go/aws/ecr"
	"github.com/pulumi/pulumi-aws/sdk/v7/go/aws/servicediscovery"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type ServiceConfig struct {
	Name                string                   `json:"name"`
	Public              bool                     `json:"public"`
	Hosts               []string                 `json:"hosts"`
	Paths               []string                 `json:"paths"`
	StripPathPrefix     bool                     `json:"stripPathPrefix"`
	Priority            int                      `json:"priority"`
	Port                int                      `json:"port"`
	HealthCheckPath     string                   `json:"healthCheckPath"`
	HealthCheckInterval int                      `json:"healthCheckInterval"`
	Monitoring          *ServiceMonitoringConfig `json:"monitoring,omitempty"`
	S3Buckets           []string                 `json:"s3Buckets,omitempty"`
}

type ClusterConfig struct {
	Name                       string                                `json:"name"`
	PublicListenerARN          string                                `json:"publicListenerARN"`
	PrivateListenerARN         string                                `json:"privateListenerARN"`
	Services                   []ServiceConfig                       `json:"services"`
	CapacityProviderStrategies []components.CapacityProviderStrategy `json:"capacityProviderStrategies"`
}

type ProvisionECSArgs struct {
	Clusters       []ClusterConfig
	VpcID          string
	ManagedBy      string
	PlatformSource string
	AWSRegion      string
	AccountID      string
}

// ProvisionServiceConnect creates HTTP namespaces for service connect and exports their ARNs.
func ProvisionServiceConnect(ctx *pulumi.Context, namespaces []string, provider *aws.Provider) error {
	for _, name := range namespaces {
		ns, err := servicediscovery.NewHttpNamespace(ctx, fmt.Sprintf("sc-namespace-%s", name), &servicediscovery.HttpNamespaceArgs{
			Name: pulumi.String(name),
		}, pulumi.Provider(provider))
		if err != nil {
			return fmt.Errorf("error creating service connect namespace %q: %w", name, err)
		}
		ctx.Export(fmt.Sprintf("serviceConnect.%s.arn", name), ns.Arn)
	}
	return nil
}

// ProvisionedService holds outputs for a single provisioned service.
type ProvisionedService struct {
	TargetGroupARN  pulumi.StringOutput
	ListenerRuleARN pulumi.StringOutput
	ListenerARN     string // the listener ARN selected for this service (public or private)
}

// ProvisionedECS is keyed by cluster config name then service name.
type ProvisionedECS struct {
	Clusters map[string]map[string]*ProvisionedService
}

func ProvisionECS(ctx *pulumi.Context, args ProvisionECSArgs, provider *aws.Provider) (*ProvisionedECS, error) {
	result := &ProvisionedECS{
		Clusters: make(map[string]map[string]*ProvisionedService),
	}
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
		return nil, fmt.Errorf("error creating ECR provider: %w", err)
	}

	// Build cache ECR repository — dev only, uses env-agnostic provider (no Environment tag).
	if ctx.Stack() == "dev" {
		cacheRepo, err := components.NewBuildCacheECRRepository(ctx, "build-cache", &components.BuildCacheECRRepositoryArgs{
			Name: pulumi.String("build-cache"),
		}, pulumi.Provider(envAgnosticProvider))
		if err != nil {
			return nil, err
		}
		ctx.Export("buildCacheECRURL", cacheRepo.URL)
	}

	// ECR repositories — one per unique service name across all clusters.
	// Repos are created in dev and looked up in all other stacks.
	provisionedECR := map[string]bool{}
	for _, cluster := range args.Clusters {
		for _, svc := range cluster.Services {
			if svc.Name == "" || provisionedECR[svc.Name] {
				continue
			}
			provisionedECR[svc.Name] = true

			if ctx.Stack() == "dev" {
				repo, err := components.NewECRRepository(ctx, svc.Name, &components.ECRRepositoryArgs{
					Name: pulumi.String(svc.Name),
				}, pulumi.Provider(envAgnosticProvider))
				if err != nil {
					return nil, err
				}
				ctx.Export(fmt.Sprintf("%s.ecrURL", svc.Name), repo.URL)
				ctx.Export(fmt.Sprintf("%s.ecrARN", svc.Name), repo.ARN)
			} else {
				repo, err := ecr.LookupRepository(ctx, &ecr.LookupRepositoryArgs{
					Name: svc.Name,
				}, pulumi.Provider(envAgnosticProvider))
				if err != nil {
					return nil, fmt.Errorf("error looking up ECR repository %q: %w", svc.Name, err)
				}
				ctx.Export(fmt.Sprintf("%s.ecrURL", svc.Name), pulumi.String(repo.RepositoryUrl))
				ctx.Export(fmt.Sprintf("%s.ecrARN", svc.Name), pulumi.String(repo.Arn))
			}
		}
	}

	// IAM roles — one execution + task role pair per unique service name.
	provisionedRoles := map[string]bool{}
	for _, cluster := range args.Clusters {
		for _, svc := range cluster.Services {
			if svc.Name == "" || provisionedRoles[svc.Name] {
				continue
			}
			provisionedRoles[svc.Name] = true

			roles, err := components.NewECSRoles(ctx, fmt.Sprintf("%s-%s-roles", svc.Name, ctx.Stack()), &components.ECSRolesArgs{
				ServiceName: svc.Name,
				Stack:       ctx.Stack(),
				AccountID:   args.AccountID,
				Region:      args.AWSRegion,
				S3Buckets:   svc.S3Buckets,
			}, pulumi.Provider(provider))
			if err != nil {
				return nil, err
			}
			ctx.Export(fmt.Sprintf("%s.executionRoleARN", svc.Name), roles.ExecutionRoleARN)
			ctx.Export(fmt.Sprintf("%s.taskRoleARN", svc.Name), roles.TaskRoleARN)
		}
	}

	for _, cluster := range args.Clusters {
		clusterName := fmt.Sprintf("bc-%s-cluster-%s", cluster.Name, ctx.Stack())

		ecsCluster, err := components.NewECSCluster(ctx, clusterName, &components.ECSClusterArgs{
			ClusterName:                pulumi.String(clusterName),
			CapacityProviderStrategies: cluster.CapacityProviderStrategies,
		}, pulumi.Provider(provider))
		if err != nil {
			return nil, err
		}

		ctx.Export(fmt.Sprintf("%s.clusterName", cluster.Name), ecsCluster.ClusterName)
		ctx.Export(fmt.Sprintf("%s.clusterARN", cluster.Name), ecsCluster.ClusterARN)

		result.Clusters[cluster.Name] = make(map[string]*ProvisionedService)

		for _, svc := range cluster.Services {
			resourceName := fmt.Sprintf("bc-%s-%s-%s", cluster.Name, svc.Name, ctx.Stack()) // TODO: need to shortened resourceName due to 32 char limit

			if svc.Name == "" {
				return nil, fmt.Errorf("cluster %q has a service with an empty name", cluster.Name)
			}
			if len(svc.Hosts) == 0 {
				return nil, fmt.Errorf("service %q in cluster %q has no hosts configured", svc.Name, cluster.Name)
			}

			listenerARN := cluster.PrivateListenerARN
			if svc.Public {
				listenerARN = cluster.PublicListenerARN
			}
			if listenerARN == "" {
				visibility := "private"
				if svc.Public {
					visibility = "public"
				}
				return nil, fmt.Errorf("service %q in cluster %q requires a %s listener ARN but none is configured", svc.Name, cluster.Name, visibility)
			}

			ps, err := provisionService(ctx, resourceName, svc, listenerARN, args.VpcID, provider)
			if err != nil {
				return nil, err
			}

			result.Clusters[cluster.Name][svc.Name] = &ProvisionedService{
				TargetGroupARN:  ps.targetGroupARN,
				ListenerRuleARN: ps.listenerRuleARN,
				ListenerARN:     listenerARN,
			}

			ctx.Export(fmt.Sprintf("%s.%s.targetGroupARN", cluster.Name, svc.Name), ps.targetGroupARN)
			ctx.Export(fmt.Sprintf("%s.%s.listenerRuleARN", cluster.Name, svc.Name), ps.listenerRuleARN)
		}
	}

	return result, nil
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
