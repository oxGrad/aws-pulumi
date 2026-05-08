package components

import (
	"github.com/pulumi/pulumi-aws/sdk/v7/go/aws/cloudwatch"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type CloudwatchAlarm struct {
	pulumi.ResourceState
	ARN pulumi.StringOutput
}

type CloudwatchAlarmArgs struct {
	AlarmName        pulumi.StringInput
	AlarmDescription pulumi.StringInput
	// Simple metric — set these OR MetricQueries, not both.
	Namespace         pulumi.StringInput
	MetricName        pulumi.StringInput
	Dimensions        pulumi.StringMapInput
	Statistic         pulumi.StringInput
	ExtendedStatistic pulumi.StringInput
	Period            pulumi.IntInput
	// Expression alarm
	MetricQueries cloudwatch.MetricAlarmMetricQueryArrayInput
	// Evaluation
	ComparisonOperator pulumi.StringInput
	Threshold          pulumi.Float64Input
	EvaluationPeriods  pulumi.IntInput
	DatapointsToAlarm  pulumi.IntInput
	TreatMissingData   pulumi.StringInput
	// Actions — accepts pulumi.Array{pulumi.String("arn:...")}
	AlarmActions pulumi.ArrayInput
	OkActions    pulumi.ArrayInput
}

func NewCloudwatchAlarm(ctx *pulumi.Context, name string, args *CloudwatchAlarmArgs, opts ...pulumi.ResourceOption) (*CloudwatchAlarm, error) {
	self := &CloudwatchAlarm{}
	err := ctx.RegisterComponentResource("platform:monitoring:alarm", name, self, opts...)
	if err != nil {
		return nil, err
	}

	var namePtr pulumi.StringPtrInput
	if args.AlarmName != nil {
		namePtr = args.AlarmName.ToStringOutput().ToStringPtrOutput()
	}

	var descPtr pulumi.StringPtrInput
	if args.AlarmDescription != nil {
		descPtr = args.AlarmDescription.ToStringOutput().ToStringPtrOutput()
	}

	var namespacePtr pulumi.StringPtrInput
	if args.Namespace != nil {
		namespacePtr = args.Namespace.ToStringOutput().ToStringPtrOutput()
	}

	var metricNamePtr pulumi.StringPtrInput
	if args.MetricName != nil {
		metricNamePtr = args.MetricName.ToStringOutput().ToStringPtrOutput()
	}

	var statisticPtr pulumi.StringPtrInput
	if args.Statistic != nil {
		statisticPtr = args.Statistic.ToStringOutput().ToStringPtrOutput()
	}

	var extStatPtr pulumi.StringPtrInput
	if args.ExtendedStatistic != nil {
		extStatPtr = args.ExtendedStatistic.ToStringOutput().ToStringPtrOutput()
	}

	var periodPtr pulumi.IntPtrInput
	if args.Period != nil {
		periodPtr = args.Period.ToIntOutput().ToIntPtrOutput()
	}

	var thresholdPtr pulumi.Float64PtrInput
	if args.Threshold != nil {
		thresholdPtr = args.Threshold.ToFloat64Output().ToFloat64PtrOutput()
	}

	var datapointsPtr pulumi.IntPtrInput
	if args.DatapointsToAlarm != nil {
		datapointsPtr = args.DatapointsToAlarm.ToIntOutput().ToIntPtrOutput()
	}

	var treatPtr pulumi.StringPtrInput
	if args.TreatMissingData != nil {
		treatPtr = args.TreatMissingData.ToStringOutput().ToStringPtrOutput()
	}

	alarm, err := cloudwatch.NewMetricAlarm(ctx, name, &cloudwatch.MetricAlarmArgs{
		Name:               namePtr,
		AlarmDescription:   descPtr,
		Namespace:          namespacePtr,
		MetricName:         metricNamePtr,
		Dimensions:         args.Dimensions,
		Statistic:          statisticPtr,
		ExtendedStatistic:  extStatPtr,
		Period:             periodPtr,
		MetricQueries:      args.MetricQueries,
		ComparisonOperator: args.ComparisonOperator,
		Threshold:          thresholdPtr,
		EvaluationPeriods:  args.EvaluationPeriods,
		DatapointsToAlarm:  datapointsPtr,
		TreatMissingData:   treatPtr,
		AlarmActions:       args.AlarmActions,
		OkActions:          args.OkActions,
	}, pulumi.Parent(self))
	if err != nil {
		return nil, err
	}

	self.ARN = alarm.Arn

	_ = ctx.RegisterResourceOutputs(self, pulumi.Map{
		"ARN": self.ARN,
	})

	return self, nil
}
