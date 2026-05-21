package components

import (
	"encoding/json"
	"fmt"

	"github.com/pulumi/pulumi-aws/sdk/v7/go/aws/iam"
	"github.com/pulumi/pulumi-aws/sdk/v7/go/aws/lambda"
	"github.com/pulumi/pulumi-aws/sdk/v7/go/aws/sns"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type TeamsNotifier struct {
	pulumi.ResourceState
}

type TeamsNotifierArgs struct {
	// Name is the AWS Lambda function name.
	Name string
	// CriticalTopicARN and WarningTopicARN are the two SNS topics to subscribe to.
	CriticalTopicARN pulumi.StringInput
	WarningTopicARN  pulumi.StringInput
	// SSMParameterPath is the path to the SecureString SSM parameter containing the webhook JSON map.
	SSMParameterPath string
	ZipPath          string
}

func NewTeamsNotifier(ctx *pulumi.Context, name string, args *TeamsNotifierArgs, opts ...pulumi.ResourceOption) (*TeamsNotifier, error) {
	self := &TeamsNotifier{}
	err := ctx.RegisterComponentResource("platform:lambda:teams-notifier", name, self, opts...)
	if err != nil {
		return nil, err
	}

	assumeRolePolicy, err := json.Marshal(map[string]any{
		"Version": "2012-10-17",
		"Statement": []map[string]any{
			{
				"Effect":    "Allow",
				"Principal": map[string]any{"Service": "lambda.amazonaws.com"},
				"Action":    "sts:AssumeRole",
			},
		},
	})
	if err != nil {
		return nil, err
	}

	role, err := iam.NewRole(ctx, fmt.Sprintf("%s-role", name), &iam.RoleArgs{
		AssumeRolePolicy: pulumi.String(string(assumeRolePolicy)),
	}, pulumi.Parent(self))
	if err != nil {
		return nil, err
	}

	_, err = iam.NewRolePolicyAttachment(ctx, fmt.Sprintf("%s-basic-exec", name), &iam.RolePolicyAttachmentArgs{
		Role:      role.Name,
		PolicyArn: pulumi.String("arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"),
	}, pulumi.Parent(self))
	if err != nil {
		return nil, err
	}

	ssmPolicyDoc, err := json.Marshal(map[string]any{
		"Version": "2012-10-17",
		"Statement": []map[string]any{
			{
				"Effect":   "Allow",
				"Action":   "ssm:GetParameter",
				"Resource": fmt.Sprintf("arn:aws:ssm:*:*:parameter%s", args.SSMParameterPath),
			},
		},
	})
	if err != nil {
		return nil, err
	}

	_, err = iam.NewRolePolicy(ctx, fmt.Sprintf("%s-ssm-policy", name), &iam.RolePolicyArgs{
		Role:   role.Name,
		Policy: pulumi.String(string(ssmPolicyDoc)),
	}, pulumi.Parent(self))
	if err != nil {
		return nil, err
	}

	fn, err := lambda.NewFunction(ctx, name, &lambda.FunctionArgs{
		Name:    pulumi.String(args.Name),
		Runtime: pulumi.String("provided.al2023"),
		Handler: pulumi.String("bootstrap"),
		Role:    role.Arn,
		Code:    pulumi.NewFileArchive(args.ZipPath),
		Environment: &lambda.FunctionEnvironmentArgs{
			Variables: pulumi.StringMap{
				"SSM_PARAMETER_PATH": pulumi.String(args.SSMParameterPath),
				"CRITICAL_TOPIC_ARN": args.CriticalTopicARN,
				"WARNING_TOPIC_ARN":  args.WarningTopicARN,
			},
		},
	}, pulumi.Parent(self))
	if err != nil {
		return nil, err
	}

	for _, topic := range []struct {
		suffix string
		arn    pulumi.StringInput
	}{
		{"critical", args.CriticalTopicARN},
		{"warning", args.WarningTopicARN},
	} {
		_, err = lambda.NewPermission(ctx, fmt.Sprintf("%s-sns-invoke-%s", name, topic.suffix), &lambda.PermissionArgs{
			Action:    pulumi.String("lambda:InvokeFunction"),
			Function:  fn.Name,
			Principal: pulumi.String("sns.amazonaws.com"),
			SourceArn: topic.arn,
		}, pulumi.Parent(self))
		if err != nil {
			return nil, err
		}

		_, err = sns.NewTopicSubscription(ctx, fmt.Sprintf("%s-sub-%s", name, topic.suffix), &sns.TopicSubscriptionArgs{
			Topic:    topic.arn,
			Protocol: pulumi.String("lambda"),
			Endpoint: fn.Arn,
		}, pulumi.Parent(self))
		if err != nil {
			return nil, err
		}
	}

	_ = ctx.RegisterResourceOutputs(self, pulumi.Map{})
	return self, nil
}
