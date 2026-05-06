package components

import (
	"strings"

	"github.com/pulumi/pulumi-aws/sdk/v7/go/aws/lb"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type ListenerRule struct {
	pulumi.ResourceState
	ARN pulumi.StringOutput
}

type ListenerRuleArgs struct {
	ListenerARN     pulumi.StringInput
	TargetGroupARN  pulumi.StringInput
	Hosts           pulumi.StringArrayInput
	Paths           pulumi.StringArrayInput
	StripPathPrefix bool
	Priority        pulumi.IntInput
}

func NewListenerRule(ctx *pulumi.Context, name string, args *ListenerRuleArgs, opts ...pulumi.ResourceOption) (*ListenerRule, error) {
	self := &ListenerRule{}
	err := ctx.RegisterComponentResource("platform:lb:listener-rule", name, self, opts...)
	if err != nil {
		return nil, err
	}

	tags := pulumi.StringMap{
		"Name": pulumi.String(name),
	}
	if args.StripPathPrefix {
		tags["PathTransform"] = pulumi.String("strip-prefix")
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
					Values: args.Hosts,
				},
			},
			&lb.ListenerRuleConditionArgs{
				PathPattern: &lb.ListenerRuleConditionPathPatternArgs{
					Values: args.Paths,
				},
			},
		},
		Tags: tags,
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

// stripPrefix returns the static prefix portion of a path pattern.
// "/gobc-sandbox/*" → "/gobc-sandbox"
func stripPrefix(pathPattern string) string {
	idx := strings.Index(pathPattern, "*")
	if idx < 0 {
		return pathPattern
	}
	return strings.TrimRight(pathPattern[:idx], "/")
}
