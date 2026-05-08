package components

import (
	"fmt"

	"github.com/pulumi/pulumi-aws/sdk/v7/go/aws/sns"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type SNSTopic struct {
	pulumi.ResourceState
	ARN  pulumi.StringOutput
	Name pulumi.StringOutput
}

type SNSTopicSubscription struct {
	// Protocol is one of: https, http, email, email-json, sqs, lambda, firehose, application.
	Protocol string
	Endpoint string
}

type SNSTopicArgs struct {
	Name          pulumi.StringInput
	Subscriptions []SNSTopicSubscription
}

func NewSNSTopic(ctx *pulumi.Context, name string, args *SNSTopicArgs, opts ...pulumi.ResourceOption) (*SNSTopic, error) {
	self := &SNSTopic{}
	err := ctx.RegisterComponentResource("platform:notifications:sns-topic", name, self, opts...)
	if err != nil {
		return nil, err
	}

	topic, err := sns.NewTopic(ctx, name, &sns.TopicArgs{
		Name: args.Name,
		Tags: pulumi.StringMap{
			"Name": args.Name,
		},
	}, pulumi.Parent(self))
	if err != nil {
		return nil, err
	}

	for i, sub := range args.Subscriptions {
		_, err = sns.NewTopicSubscription(ctx, fmt.Sprintf("%s-sub-%d", name, i), &sns.TopicSubscriptionArgs{
			Topic:    topic.Arn,
			Protocol: pulumi.String(sub.Protocol),
			Endpoint: pulumi.String(sub.Endpoint),
		}, pulumi.Parent(self))
		if err != nil {
			return nil, err
		}
	}

	self.ARN = topic.Arn
	self.Name = topic.Name

	_ = ctx.RegisterResourceOutputs(self, pulumi.Map{
		"ARN":  self.ARN,
		"Name": self.Name,
	})

	return self, nil
}
