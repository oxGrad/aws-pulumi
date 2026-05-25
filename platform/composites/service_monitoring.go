package composites

import (
	"fmt"
	"platform/components"
	"strings"

	"github.com/pulumi/pulumi-aws/sdk/v7/go/aws"
	"github.com/pulumi/pulumi-aws/sdk/v7/go/aws/cloudwatch"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// ServiceMonitoringConfig is the YAML-configurable part of service monitoring.
type ServiceMonitoringConfig struct {
	Enabled             bool    `json:"enabled"`
	LogGroupName        string  `json:"logGroupName"`
	UnhandledLogPattern string  `json:"unhandledLogPattern"`  // default: "ERROR"
	LatencySLASecs      float64 `json:"latencySLASecs"`       // p99 threshold in seconds
	DesiredTaskCount    int     `json:"desiredTaskCount"`
	MinHealthyHosts     int     `json:"minHealthyHosts"`
	CPUAlarmThreshold   float64 `json:"cpuAlarmThreshold"`    // default: 85; set above autoscaling cpuTarget
	MemoryAlarmThreshold float64 `json:"memoryAlarmThreshold"` // default: 80; set above autoscaling memoryTarget
}

// ServiceMonitoringArgs combines runtime infrastructure values with config.
// CriticalActions and WarningActions are SNS topic ARNs provisioned by ProvisionNotifications.
type ServiceMonitoringArgs struct {
	Config          ServiceMonitoringConfig
	ECSClusterName  string
	ECSServiceName  string // bare service name used as CloudWatch ECS dimension (e.g. "gobc-sandbox")
	// ServiceLabel is used for custom metric namespaces and dimensions; includes stack to avoid
	// collisions when dev and prod share the same AWS account (e.g. "gobc-sandbox-dev").
	ServiceLabel    string
	// ListenerARN is the ALB listener ARN; the load-balancer dimension is derived from it.
	ListenerARN     string
	TargetGroupARN  pulumi.StringInput
	CriticalActions pulumi.ArrayInput
	WarningActions  pulumi.ArrayInput
}

// ProvisionServiceMonitoring creates all CloudWatch alarms for a service.
func ProvisionServiceMonitoring(ctx *pulumi.Context, name string, args ServiceMonitoringArgs, provider *aws.Provider) error {
	cfg := args.Config

	criticalActions := args.CriticalActions
	warningActions := args.WarningActions

	lbDim := lbDimFromListenerARN(args.ListenerARN)

	// ALB dimensions (TargetGroupARN is a Pulumi output; LB dim is Go-time).
	albDims := args.TargetGroupARN.ToStringOutput().
		ApplyT(func(arn string) map[string]string {
			parts := strings.Split(arn, ":")
			return map[string]string{
				"LoadBalancer": lbDim,
				"TargetGroup":  parts[len(parts)-1],
			}
		}).(pulumi.StringMapOutput)

	ecsDims := pulumi.StringMap{
		"ClusterName": pulumi.String(args.ECSClusterName),
		"ServiceName": pulumi.String(args.ECSServiceName),
	}

	// ── Critical: High 5xx rate ────────────────────────────────────────────
	// Expression: HTTPCode_Target_5XX_Count / RequestCount * 100 > 1 %
	// 2-of-2 × 1-min periods; missing data = not breaching.
	_, err := components.NewCloudwatchAlarm(ctx, fmt.Sprintf("%s-5xx-rate", name), &components.CloudwatchAlarmArgs{
		AlarmName:        pulumi.Sprintf("%s-5xx-rate", name),
		AlarmDescription: pulumi.String("5xx error rate > 1%"),
		MetricQueries: cloudwatch.MetricAlarmMetricQueryArray{
			&cloudwatch.MetricAlarmMetricQueryArgs{
				Id: pulumi.String("m1"),
				Metric: &cloudwatch.MetricAlarmMetricQueryMetricArgs{
					Namespace:  pulumi.String("AWS/ApplicationELB"),
					MetricName: pulumi.String("HTTPCode_Target_5XX_Count"),
					Period:     pulumi.Int(60),
					Stat:       pulumi.String("Sum"),
					Dimensions: albDims,
				},
			},
			&cloudwatch.MetricAlarmMetricQueryArgs{
				Id: pulumi.String("m2"),
				Metric: &cloudwatch.MetricAlarmMetricQueryMetricArgs{
					Namespace:  pulumi.String("AWS/ApplicationELB"),
					MetricName: pulumi.String("RequestCount"),
					Period:     pulumi.Int(60),
					Stat:       pulumi.String("Sum"),
					Dimensions: albDims,
				},
			},
			&cloudwatch.MetricAlarmMetricQueryArgs{
				Id:         pulumi.String("e1"),
				Expression: pulumi.String("IF(m2>0, m1/m2*100, 0)"),
				ReturnData: pulumi.Bool(true),
				Label:      pulumi.String("5xx rate %"),
			},
		},
		ComparisonOperator: pulumi.String("GreaterThanThreshold"),
		Threshold:          pulumi.Float64(1),
		EvaluationPeriods:  pulumi.Int(2),
		DatapointsToAlarm:  pulumi.Int(2),
		TreatMissingData:   pulumi.String("notBreaching"),
		AlarmActions:       criticalActions,
		OkActions:          criticalActions,
	}, pulumi.Provider(provider))
	if err != nil {
		return err
	}

	// ── Critical: No healthy hosts ─────────────────────────────────────────
	minHosts := cfg.MinHealthyHosts
	if minHosts == 0 {
		minHosts = 1
	}
	_, err = components.NewCloudwatchAlarm(ctx, fmt.Sprintf("%s-healthy-hosts", name), &components.CloudwatchAlarmArgs{
		AlarmName:          pulumi.Sprintf("%s-healthy-hosts", name),
		AlarmDescription:   pulumi.String("No healthy hosts in target group"),
		Namespace:          pulumi.String("AWS/ApplicationELB"),
		MetricName:         pulumi.String("HealthyHostCount"),
		Dimensions:         albDims,
		Statistic:          pulumi.String("Minimum"),
		Period:             pulumi.Int(60),
		ComparisonOperator: pulumi.String("LessThanThreshold"),
		Threshold:          pulumi.Float64(float64(minHosts)),
		EvaluationPeriods:  pulumi.Int(1),
		TreatMissingData:   pulumi.String("breaching"),
		AlarmActions:       criticalActions,
		OkActions:          criticalActions,
	}, pulumi.Provider(provider))
	if err != nil {
		return err
	}

	// ── Critical: ECS task crash ───────────────────────────────────────────
	desired := cfg.DesiredTaskCount
	if desired == 0 {
		desired = 1
	}
	_, err = components.NewCloudwatchAlarm(ctx, fmt.Sprintf("%s-running-tasks", name), &components.CloudwatchAlarmArgs{
		AlarmName:          pulumi.Sprintf("%s-running-tasks", name),
		AlarmDescription:   pulumi.String("Running task count below desired"),
		Namespace:          pulumi.String("AWS/ECS"),
		MetricName:         pulumi.String("RunningTaskCount"),
		Dimensions:         ecsDims,
		Statistic:          pulumi.String("Minimum"),
		Period:             pulumi.Int(60),
		ComparisonOperator: pulumi.String("LessThanThreshold"),
		Threshold:          pulumi.Float64(float64(desired)),
		EvaluationPeriods:  pulumi.Int(1),
		TreatMissingData:   pulumi.String("breaching"),
		AlarmActions:       criticalActions,
		OkActions:          criticalActions,
	}, pulumi.Provider(provider))
	if err != nil {
		return err
	}

	// ── Critical: Unhandled exceptions (log metric) ────────────────────────
	if cfg.LogGroupName != "" {
		filterPattern := cfg.UnhandledLogPattern
		if filterPattern == "" {
			filterPattern = "ERROR"
		}
		metricNamespace := fmt.Sprintf("Platform/Services/%s", args.ServiceLabel)

		_, err = components.NewLogMetricFilter(ctx, fmt.Sprintf("%s-unhandled-filter", name), &components.LogMetricFilterArgs{
			LogGroupName:    pulumi.String(cfg.LogGroupName),
			FilterPattern:   filterPattern,
			MetricName:      "UnhandledException",
			MetricNamespace: metricNamespace,
		}, pulumi.Provider(provider))
		if err != nil {
			return err
		}

		_, err = components.NewCloudwatchAlarm(ctx, fmt.Sprintf("%s-unhandled", name), &components.CloudwatchAlarmArgs{
			AlarmName:          pulumi.Sprintf("%s-unhandled", name),
			AlarmDescription:   pulumi.String("Unhandled exception in logs"),
			Namespace:          pulumi.String(metricNamespace),
			MetricName:         pulumi.String("UnhandledException"),
			Statistic:          pulumi.String("Sum"),
			Period:             pulumi.Int(300),
			ComparisonOperator: pulumi.String("GreaterThanThreshold"),
			Threshold:          pulumi.Float64(0),
			EvaluationPeriods:  pulumi.Int(1),
			TreatMissingData:   pulumi.String("notBreaching"),
			AlarmActions:       criticalActions,
			OkActions:          criticalActions,
		}, pulumi.Provider(provider))
		if err != nil {
			return err
		}
	}

	// ── Warning: High CPU ──────────────────────────────────────────────────
	cpuThreshold := cfg.CPUAlarmThreshold
	if cpuThreshold == 0 {
		cpuThreshold = 85
	}
	_, err = components.NewCloudwatchAlarm(ctx, fmt.Sprintf("%s-cpu", name), &components.CloudwatchAlarmArgs{
		AlarmName:          pulumi.Sprintf("%s-cpu", name),
		AlarmDescription:   pulumi.Sprintf("CPU utilization > %.0f%%", cpuThreshold),
		Namespace:          pulumi.String("AWS/ECS"),
		MetricName:         pulumi.String("CPUUtilization"),
		Dimensions:         ecsDims,
		Statistic:          pulumi.String("Average"),
		Period:             pulumi.Int(300),
		ComparisonOperator: pulumi.String("GreaterThanThreshold"),
		Threshold:          pulumi.Float64(cpuThreshold),
		EvaluationPeriods:  pulumi.Int(2),
		DatapointsToAlarm:  pulumi.Int(2),
		AlarmActions:       warningActions,
		OkActions:          warningActions,
	}, pulumi.Provider(provider))
	if err != nil {
		return err
	}

	// ── Warning: High memory ───────────────────────────────────────────────
	memThreshold := cfg.MemoryAlarmThreshold
	if memThreshold == 0 {
		memThreshold = 80
	}
	_, err = components.NewCloudwatchAlarm(ctx, fmt.Sprintf("%s-memory", name), &components.CloudwatchAlarmArgs{
		AlarmName:          pulumi.Sprintf("%s-memory", name),
		AlarmDescription:   pulumi.Sprintf("Memory utilization > %.0f%%", memThreshold),
		Namespace:          pulumi.String("AWS/ECS"),
		MetricName:         pulumi.String("MemoryUtilization"),
		Dimensions:         ecsDims,
		Statistic:          pulumi.String("Average"),
		Period:             pulumi.Int(300),
		ComparisonOperator: pulumi.String("GreaterThanThreshold"),
		Threshold:          pulumi.Float64(memThreshold),
		EvaluationPeriods:  pulumi.Int(2),
		DatapointsToAlarm:  pulumi.Int(2),
		AlarmActions:       warningActions,
		OkActions:          warningActions,
	}, pulumi.Provider(provider))
	if err != nil {
		return err
	}

	// ── Warning: Latency degraded (p99) ───────────────────────────────────
	if cfg.LatencySLASecs > 0 {
		_, err = components.NewCloudwatchAlarm(ctx, fmt.Sprintf("%s-latency", name), &components.CloudwatchAlarmArgs{
			AlarmName:          pulumi.Sprintf("%s-latency", name),
			AlarmDescription:   pulumi.String("p99 response time exceeds SLA"),
			Namespace:          pulumi.String("AWS/ApplicationELB"),
			MetricName:         pulumi.String("TargetResponseTime"),
			Dimensions:         albDims,
			ExtendedStatistic:  pulumi.String("p99"),
			Period:             pulumi.Int(300),
			ComparisonOperator: pulumi.String("GreaterThanThreshold"),
			Threshold:          pulumi.Float64(cfg.LatencySLASecs),
			EvaluationPeriods:  pulumi.Int(2),
			DatapointsToAlarm:  pulumi.Int(2),
			AlarmActions:       warningActions,
			OkActions:          warningActions,
		}, pulumi.Provider(provider))
		if err != nil {
			return err
		}
	}

	// ── Warning: Severe instances (custom metric) ─────────────────────────
	_, err = components.NewCloudwatchAlarm(ctx, fmt.Sprintf("%s-severe-instances", name), &components.CloudwatchAlarmArgs{
		AlarmName:        pulumi.Sprintf("%s-severe-instances", name),
		AlarmDescription: pulumi.String("Severe instance count > 0"),
		Namespace:        pulumi.String("Platform/Services"),
		MetricName:       pulumi.String("InstancesSevere"),
		Dimensions: pulumi.StringMap{
			"ServiceName": pulumi.String(args.ServiceLabel),
		},
		Statistic:          pulumi.String("Maximum"),
		Period:             pulumi.Int(60),
		ComparisonOperator: pulumi.String("GreaterThanThreshold"),
		Threshold:          pulumi.Float64(0),
		EvaluationPeriods:  pulumi.Int(1),
		TreatMissingData:   pulumi.String("notBreaching"),
		AlarmActions:       warningActions,
		OkActions:          warningActions,
	}, pulumi.Provider(provider))
	if err != nil {
		return err
	}

	return nil
}

// lbDimFromListenerARN extracts the CloudWatch LoadBalancer dimension from an ALB listener ARN.
// Input:  arn:aws:...:listener/app/lb-name/lb-id/listener-id
// Output: app/lb-name/lb-id
func lbDimFromListenerARN(arn string) string {
	idx := strings.Index(arn, ":listener/")
	if idx == -1 {
		return ""
	}
	path := arn[idx+len(":listener/"):]
	lastSlash := strings.LastIndex(path, "/")
	if lastSlash == -1 {
		return path
	}
	return path[:lastSlash]
}

