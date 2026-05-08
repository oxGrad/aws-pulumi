package composites

import (
	"fmt"
	"platform/components"

	"github.com/pulumi/pulumi-aws/sdk/v7/go/aws"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// NotificationEndpoint is one subscriber on an SNS topic.
type NotificationEndpoint struct {
	// Protocol is one of: https, http, email, email-json, sqs, lambda.
	Protocol string `json:"protocol"`
	Endpoint string `json:"endpoint"`
}

// NotificationsConfig is YAML-configurable and lives at the stack level.
// Two topics are created per environment: critical (page immediately) and warning (Slack/email).
type NotificationsConfig struct {
	Critical []NotificationEndpoint `json:"critical"`
	Warning  []NotificationEndpoint `json:"warning"`
}

// ProvisionedNotifications exposes the two env-level SNS topic ARNs.
type ProvisionedNotifications struct {
	CriticalTopicARN pulumi.StringOutput
	WarningTopicARN  pulumi.StringOutput
}

func ProvisionNotifications(ctx *pulumi.Context, args NotificationsConfig, provider *aws.Provider) (*ProvisionedNotifications, error) {
	stack := ctx.Stack()

	criticalName := fmt.Sprintf("bc-%s-critical", stack)
	critical, err := components.NewSNSTopic(ctx, criticalName, &components.SNSTopicArgs{
		Name:          pulumi.String(criticalName),
		Subscriptions: toTopicSubscriptions(args.Critical),
	}, pulumi.Provider(provider))
	if err != nil {
		return nil, err
	}

	warningName := fmt.Sprintf("bc-%s-warning", stack)
	warning, err := components.NewSNSTopic(ctx, warningName, &components.SNSTopicArgs{
		Name:          pulumi.String(warningName),
		Subscriptions: toTopicSubscriptions(args.Warning),
	}, pulumi.Provider(provider))
	if err != nil {
		return nil, err
	}

	ctx.Export("notifications.criticalTopicARN", critical.ARN)
	ctx.Export("notifications.warningTopicARN", warning.ARN)

	return &ProvisionedNotifications{
		CriticalTopicARN: critical.ARN,
		WarningTopicARN:  warning.ARN,
	}, nil
}

func toTopicSubscriptions(endpoints []NotificationEndpoint) []components.SNSTopicSubscription {
	subs := make([]components.SNSTopicSubscription, len(endpoints))
	for i, e := range endpoints {
		subs[i] = components.SNSTopicSubscription{Protocol: e.Protocol, Endpoint: e.Endpoint}
	}
	return subs
}
