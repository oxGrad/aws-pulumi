package components

import (
	"github.com/pulumi/pulumi-aws/sdk/v7/go/aws/lb"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type ListenerRule struct {
	pulumi.ResourceState
	ARN pulumi.StringOutput
}

type ListenerRuleArgs struct {
	ListenerARN    pulumi.StringInput
	TargetGroupARN pulumi.StringInput
	Host           pulumi.StringInput
	Path           pulumi.StringInput
	Priority       pulumi.IntInput
}

func NewListenerRule(ctx *pulumi.Context, name string, args *ListenerRuleArgs, opts ...pulumi.ResourceOption) (*ListenerRule, error) {
	self := &ListenerRule{}
	err := ctx.RegisterComponentResource("platform:lb:listener-rule", name, self, opts...)
	if err != nil {
		return nil, err
	}

	rule, err := lb.NewListenerRule(ctx, name, &lb.ListenerRuleArgs{
		ListenerArn: args.ListenerARN,
		Priority:    args.Priority,
		Actions: lb.ListenerRuleActionArray{
			&lb.ListenerRuleActionArgs{
				Type:           pulumi.String("forward"),
				TargetGroupArn: args.TargetGroupARN,
			},
		},
		Conditions: lb.ListenerRuleConditionArray{
			&lb.ListenerRuleConditionArgs{
				HostHeader: &lb.ListenerRuleConditionHostHeaderArgs{
					Values: pulumi.StringArray{args.Host},
				},
			},
			&lb.ListenerRuleConditionArgs{
				PathPattern: &lb.ListenerRuleConditionPathPatternArgs{
					Values: pulumi.StringArray{args.Path},
				},
			},
		},
		Tags: pulumi.StringMap{
			"Name": pulumi.String(name),
		},
	}, pulumi.Parent(self))
	if err != nil {
		return nil, err
	}

	self.ARN = rule.Arn

	_ = ctx.RegisterResourceOutputs(self, pulumi.Map{
		"ARN": self.ARN,
	})

	return self, nil
}
