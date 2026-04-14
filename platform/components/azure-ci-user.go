package components

import (
	"encoding/json"
	"fmt"

	"github.com/pulumi/pulumi-aws/sdk/v7/go/aws/iam"
	"github.com/pulumi/pulumi-aws/sdk/v7/go/aws/secretsmanager"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type AzureCIUser struct {
	pulumi.ResourceState
}

type AzureCIUserArgs struct {
	User          pulumi.StringInput
	BootstrapUser pulumi.StringInput
}

func NewAzureCIUser(ctx *pulumi.Context, name string, args *AzureCIUserArgs, opts ...pulumi.ResourceOption) (*AzureCIUser, error) {
	self := &AzureCIUser{}
	err := ctx.RegisterComponentResource("platform:iam:azure-ci-user", name, self, opts...)
	if err != nil {
		return nil, err
	}

	_, err = iam.NewUser(ctx, name, &iam.UserArgs{
		Name: args.User,
		Path: pulumi.String("/ci/"),
		Tags: pulumi.StringMap{
			"Environment": pulumi.String(ctx.Stack()),
			"ManagedBy":   args.BootstrapUser,
			"Team":        pulumi.String("SRE"),
			"Purpose":     pulumi.String("Azure Pipelines CI/CD deployments"),
		},
	}, pulumi.Parent(self))
	if err != nil {
		return nil, fmt.Errorf("error creating IAM user: %w", err)
	}

	deploymentPolicyDoc, err := json.Marshal(map[string]any{
		"Version": "2012-10-17",
		"Statement": []map[string]any{
			{
				"Sid":      "ECRAuth",
				"Effect":   "Allow",
				"Action":   []string{"ecr:GetAuthorizationToken"},
				"Resource": "*",
			},
			{
				"Sid":    "ECRRepository",
				"Effect": "Allow",
				"Action": []string{
					"ecr:DescribeRepositories",
					"ecr:CreateRepository",
					"ecr:TagResource",
					"ecr:BatchCheckLayerAvailability",
					"ecr:GetDownloadUrlForLayer",
					"ecr:BatchGetImage",
					"ecr:InitiateLayerUpload",
					"ecr:UploadLayerPart",
					"ecr:CompleteLayerUpload",
					"ecr:PutImage",
				},
				"Resource": "*",
			},
			{
				"Sid":    "ECSTaskDefinition",
				"Effect": "Allow",
				"Action": []string{
					"ecs:RegisterTaskDefinition",
					"ecs:DescribeTaskDefinition",
				},
				"Resource": "*",
			},
			{
				"Sid":    "ECSService",
				"Effect": "Allow",
				"Action": []string{
					"ecs:UpdateService",
					"ecs:DescribeServices",
				},
				"Resource": "*",
			},
			{
				"Sid":    "ECSCluster",
				"Effect": "Allow",
				"Action": []string{
					"ecs:CreateCluster",
					"ecs:DeleteCluster",
					"ecs:UpdateCluster",
					"ecs:DescribeClusters",
				},
				"Resource": "*",
			},
			{
				"Sid":    "PassRoleToECS",
				"Effect": "Allow",
				"Action": "iam:PassRole",
				"Resource": "*",
				"Condition": map[string]any{
					"StringEquals": map[string]string{
						"iam:PassedToService": "ecs-tasks.amazonaws.com",
					},
				},
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("error marshaling deployment policy: %w", err)
	}

	deploymentPolicy, err := iam.NewPolicy(ctx, fmt.Sprintf("%v-deployment-policy", name), &iam.PolicyArgs{
		Name:        pulumi.Sprintf("%v-deployment-policy", args.User),
		Path:        pulumi.String("/ci/"),
		Description: pulumi.String("ECR, ECS, and PassRole permissions for Azure Pipelines CI/CD deployments"),
		Policy:      pulumi.String(string(deploymentPolicyDoc)),
		Tags: pulumi.StringMap{
			"ManagedBy": args.BootstrapUser,
			"Team":      pulumi.String("SRE"),
			"Purpose":   pulumi.String("CI/CD deployment"),
		},
	}, pulumi.Parent(self))
	if err != nil {
		return nil, fmt.Errorf("error creating deployment policy: %w", err)
	}

	_, err = iam.NewUserPolicyAttachment(ctx, fmt.Sprintf("%v-deployment-policy-attachment", name), &iam.UserPolicyAttachmentArgs{
		User:      args.User,
		PolicyArn: deploymentPolicy.Arn,
	}, pulumi.Parent(self))
	if err != nil {
		return nil, fmt.Errorf("error attaching deployment policy to user: %w", err)
	}

	_ = ctx.RegisterResourceOutputs(self, pulumi.Map{})

	return self, nil
}