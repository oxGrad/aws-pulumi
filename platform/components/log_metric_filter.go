package components

import (
	"github.com/pulumi/pulumi-aws/sdk/v7/go/aws/cloudwatch"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type LogMetricFilter struct {
	pulumi.ResourceState
	MetricName      string
	MetricNamespace string
}

type LogMetricFilterArgs struct {
	LogGroupName    pulumi.StringInput
	FilterPattern   string
	MetricName      string
	MetricNamespace string
	// MetricValue is the value emitted per matching log event. Defaults to "1".
	MetricValue string
}

func NewLogMetricFilter(ctx *pulumi.Context, name string, args *LogMetricFilterArgs, opts ...pulumi.ResourceOption) (*LogMetricFilter, error) {
	self := &LogMetricFilter{
		MetricName:      args.MetricName,
		MetricNamespace: args.MetricNamespace,
	}
	err := ctx.RegisterComponentResource("platform:monitoring:log-metric-filter", name, self, opts...)
	if err != nil {
		return nil, err
	}

	metricValue := args.MetricValue
	if metricValue == "" {
		metricValue = "1"
	}

	_, err = cloudwatch.NewLogMetricFilter(ctx, name, &cloudwatch.LogMetricFilterArgs{
		Name:         pulumi.String(name),
		Pattern:      pulumi.String(args.FilterPattern),
		LogGroupName: args.LogGroupName,
		MetricTransformation: &cloudwatch.LogMetricFilterMetricTransformationArgs{
			Name:      pulumi.String(args.MetricName),
			Namespace: pulumi.String(args.MetricNamespace),
			Value:     pulumi.String(metricValue),
		},
	}, pulumi.Parent(self))
	if err != nil {
		return nil, err
	}

	_ = ctx.RegisterResourceOutputs(self, pulumi.Map{})

	return self, nil
}
