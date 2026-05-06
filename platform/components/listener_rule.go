package components

import (
	"fmt"

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
	Hosts          pulumi.StringArrayInput
	Paths          pulumi.StringArrayInput
	// StripPathPrefix is a Go-time bool (not a Pulumi input) because the tag
	// value must be known before resource construction. It cannot depend on
	// a Pulumi output.
	StripPathPrefix bool
	PathPrefix      string
	Priority        pulumi.IntInput
}

func NewListenerRule(ctx *pulumi.Context, name string, args *ListenerRuleArgs, opts ...pulumi.ResourceOption) (*ListenerRule, error) {
	if args.Hosts == nil || args.Paths == nil {
		return nil, fmt.Errorf("NewListenerRule %q: Hosts and Paths must not be nil", name)
	}

	self := &ListenerRule{}
	err := ctx.RegisterComponentResource("platform:lb:listener-rule", name, self, opts...)
	if err != nil {
		return nil, err
	}

	tags := pulumi.StringMap{
		"Name": pulumi.String(name),
	}

	actions := lb.ListenerRuleActionArray{}

	var transforms lb.ListenerRuleTransformArray
	if args.StripPathPrefix && args.PathPrefix != "" {
		pattern := fmt.Sprintf("^%s/(.*)$", args.PathPrefix)
		transforms = lb.ListenerRuleTransformArray{
			&lb.ListenerRuleTransformArgs{
				Type:           pulumi.String("url-rewrite"),
				UrlRewriteConfig: &lb.ListenerRuleTransformUrlRewriteConfigArgs{
					Rewrite: &lb.ListenerRuleTransformUrlRewriteConfigRewriteArgs{
						Regex:  pulumi.String(pattern),
						Replace: pulumi.String("/$1"),
					},
				},
			},
		}
	}

	actions = append(actions, &lb.ListenerRuleActionArgs{
		Type:           pulumi.String("forward"),
		TargetGroupArn: args.TargetGroupARN,
	})

	rule, err := lb.NewListenerRule(ctx, name, &lb.ListenerRuleArgs{
		ListenerArn: args.ListenerARN,
		Priority:    args.Priority,
		Actions:     actions,
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
		Tags:       tags,
		Transforms: transforms,
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
