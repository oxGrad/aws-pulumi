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
	Name       string
	TopicARN   pulumi.StringInput
	WebhookURL pulumi.StringInput
	ZipPath    string
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

	fn, err := lambda.NewFunction(ctx, name, &lambda.FunctionArgs{
		Name:    pulumi.String(args.Name),
		Runtime: pulumi.String("provided.al2023"),
		Handler: pulumi.String("bootstrap"),
		Role:    role.Arn,
		Code:    pulumi.NewFileArchive(args.ZipPath),
		Environment: &lambda.FunctionEnvironmentArgs{
			Variables: pulumi.StringMap{
				"TEAMS_WEBHOOK_URL": args.WebhookURL,
			},
		},
	}, pulumi.Parent(self))
	if err != nil {
		return nil, err
	}

	_, err = lambda.NewPermission(ctx, fmt.Sprintf("%s-sns-invoke", name), &lambda.PermissionArgs{
		Action:    pulumi.String("lambda:InvokeFunction"),
		Function:  fn.Name,
		Principal: pulumi.String("sns.amazonaws.com"),
		SourceArn: args.TopicARN,
	}, pulumi.Parent(self))
	if err != nil {
		return nil, err
	}

	_, err = sns.NewTopicSubscription(ctx, fmt.Sprintf("%s-sub", name), &sns.TopicSubscriptionArgs{
		Topic:    args.TopicARN,
		Protocol: pulumi.String("lambda"),
		Endpoint: fn.Arn,
	}, pulumi.Parent(self))
	if err != nil {
		return nil, err
	}

	_ = ctx.RegisterResourceOutputs(self, pulumi.Map{})
	return self, nil
}
