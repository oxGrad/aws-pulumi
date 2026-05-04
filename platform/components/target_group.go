package components

import (
	"github.com/pulumi/pulumi-aws/sdk/v7/go/aws/lb"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type TargetGroup struct {
	pulumi.ResourceState
	ARN pulumi.StringOutput
}

type TargetGroupArgs struct {
	Name                pulumi.StringInput
	VpcID               pulumi.StringInput
	Port                pulumi.IntInput
	Protocol            pulumi.StringInput
	HealthCheckPath     pulumi.StringInput
	HealthCheckInterval pulumi.IntInput
}

func NewTargetGroup(ctx *pulumi.Context, name string, args *TargetGroupArgs, opts ...pulumi.ResourceOption) (*TargetGroup, error) {
	self := &TargetGroup{}
	err := ctx.RegisterComponentResource("platform:lb:target-group", name, self, opts...)
	if err != nil {
		return nil, err
	}

	tg, err := lb.NewTargetGroup(ctx, name, &lb.TargetGroupArgs{
		Name:       args.Name,
		Port:       args.Port,
		Protocol:   args.Protocol,
		TargetType: pulumi.String("ip"),
		VpcId:      args.VpcID,
		HealthCheck: &lb.TargetGroupHealthCheckArgs{
			Path:               args.HealthCheckPath,
			Protocol:           pulumi.String("HTTP"),
			HealthyThreshold:   pulumi.Int(3),
			UnhealthyThreshold: pulumi.Int(3),
			Interval:           args.HealthCheckInterval,
		},
		Tags: pulumi.StringMap{
			"Name": args.Name,
		},
	}, pulumi.Parent(self))
	if err != nil {
		return nil, err
	}

	self.ARN = tg.Arn

	_ = ctx.RegisterResourceOutputs(self, pulumi.Map{
		"ARN": self.ARN,
	})

	return self, nil
}
