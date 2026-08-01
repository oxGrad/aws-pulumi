package components

import (
	"encoding/json"
	"fmt"

	"github.com/pulumi/pulumi-aws/sdk/v7/go/aws/iam"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type AzureCIUser struct {
	pulumi.ResourceState
	Username pulumi.StringOutput
}

type AzureCIUserArgs struct {
	User      pulumi.StringInput
	KmsARN    pulumi.StringInput
	Region    string
	AccountID string
}

const (
	PURPOSE pulumi.String = "Azure Pipelines CI/CD deployments"
)

func NewAzureCIUser(ctx *pulumi.Context, name string, args *AzureCIUserArgs, opts ...pulumi.ResourceOption) (*AzureCIUser, error) {
	self := &AzureCIUser{}
	err := ctx.RegisterComponentResource("platform:iam:azure-ci-user", name, self, opts...)
	if err != nil {
		return nil, err
	}

	user, err := iam.NewUser(ctx, name, &iam.UserArgs{
		Name: args.User,
		Path: pulumi.String("/ci/"),
		Tags: pulumi.StringMap{
			"Purpose": PURPOSE,
		},
	}, pulumi.Parent(self))
	if err != nil {
		return nil, fmt.Errorf("error creating IAM user: %w", err)
	}
	self.Username = user.Name

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
				"Resource": fmt.Sprintf("arn:aws:ecr:%s:%s:repository/*", args.Region, args.AccountID),
			},
			{
				"Sid":    "ECSTaskDefinition",
				"Effect": "Allow",
				"Action": []string{
					"ecs:RegisterTaskDefinition",
					"ecs:DescribeTaskDefinition",
					"ecs:TagResource",
				},
				// Task definitions aren't ARN-addressable by family name at request time
				// (RegisterTaskDefinition has no resource ARN yet), so this is scoped to
				// account/region rather than a naming prefix.
				"Resource": fmt.Sprintf("arn:aws:ecs:%s:%s:task-definition/*", args.Region, args.AccountID),
			},
			{
				"Sid":    "ECSService",
				"Effect": "Allow",
				"Action": []string{
					"ecs:CreateService",
					"ecs:UpdateService",
					"ecs:DeleteService",
					"ecs:DescribeServices",
				},
				"Resource": fmt.Sprintf("arn:aws:ecs:%s:%s:service/example-*-cluster-*/*", args.Region, args.AccountID),
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
				"Resource": fmt.Sprintf("arn:aws:ecs:%s:%s:cluster/example-*-cluster-*", args.Region, args.AccountID),
			},
			{
				"Sid":      "PassRoleToECS",
				"Effect":   "Allow",
				"Action":   "iam:PassRole",
				"Resource": fmt.Sprintf("arn:aws:iam::%s:role/ecs/*", args.AccountID),
				"Condition": map[string]any{
					"StringEquals": map[string]string{
						"iam:PassedToService": "ecs-tasks.amazonaws.com",
					},
				},
			},
			{
				"Sid":    "ECSTaskStatus",
				"Effect": "Allow",
				"Action": []string{
					"ecs:DescribeTasks",
					"ecs:ListTasks",
				},
				"Resource": fmt.Sprintf("arn:aws:ecs:%s:%s:task/example-*-cluster-*/*", args.Region, args.AccountID),
			},
			{
				"Sid":    "SSMParameterAccess",
				"Effect": "Allow",
				"Action": []string{
					"ssm:PutParameter",
					"ssm:GetParameter",
					"ssm:GetParameters",
					"ssm:AddTagsToResource",
					"ssm:ListTagsForResource",
				},
				"Resource": fmt.Sprintf("arn:aws:ssm:%s:%s:parameter/*", args.Region, args.AccountID),
			},
			{
				"Sid":    "CloudWatchLogsAccess",
				"Effect": "Allow",
				"Action": []string{
					"logs:CreateLogGroup",
					"logs:CreateLogStream",
					"logs:PutLogEvents",
					"logs:DescribeLogGroups",
					"logs:DescribeLogStreams",
					"logs:TagResource",
				},
				"Resource": fmt.Sprintf("arn:aws:logs:%s:%s:log-group:*", args.Region, args.AccountID),
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
			"Purpose": PURPOSE,
		},
	}, pulumi.Parent(self))
	if err != nil {
		return nil, fmt.Errorf("error creating deployment policy: %w", err)
	}

	_, err = iam.NewUserPolicyAttachment(ctx, fmt.Sprintf("%v-deployment-policy-attachment", name), &iam.UserPolicyAttachmentArgs{
		User:      user.Name,
		PolicyArn: deploymentPolicy.Arn,
	}, pulumi.Parent(self))
	if err != nil {
		return nil, fmt.Errorf("error attaching deployment policy to user: %w", err)
	}

	_ = ctx.RegisterResourceOutputs(self, pulumi.Map{
		"Username": self.Username,
	})

	return self, nil
}
