package components

import (
	"encoding/json"
	"fmt"

	"github.com/pulumi/pulumi-aws/sdk/v7/go/aws/iam"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type ExecutorPolicy struct {
	pulumi.ResourceState
}

type ExecutorPolicyArgs struct {
	BucketARN      pulumi.StringInput
	KmsARN         pulumi.StringInput
	RoleName       pulumi.StringInput
	AdditionalRole *ExecutorPolicyAdditionalRole
}

type ExecutorPolicyAdditionalRole struct {
	S3Admin bool
}

func NewExecutorPolicy(ctx *pulumi.Context, name string, args *ExecutorPolicyArgs, opts ...pulumi.ResourceOption) (*ExecutorPolicy, error) {
	self := &ExecutorPolicy{}
	err := ctx.RegisterComponentResource("bootstrap:iam:role-policy-executor", name, self, opts...)
	if err != nil {
		return nil, err
	}

	ExecutorPolicyDoc := pulumi.All(args.BucketARN, args.KmsARN).ApplyT(func(args []interface{}) (string, error) {
		bucketArn := args[0]
		kmsArn := args[1]

		ctx.Log.Info(fmt.Sprintf("kmsARN: %s", kmsArn), nil)
		doc := map[string]any{
			"Version": "2012-10-17",
			"Statement": []map[string]any{
				{
					"Sid":    "S3StateAccess",
					"Effect": "Allow",
					"Action": []string{
						"s3:GetObject",
						"s3:PutObject",
						"s3:DeleteObject",
						"s3:ListBucket",
						"s3:GetBucketVersioning",
						"s3:GetBucketLocation",
						"s3:ListBucketVersions",
					},
					"Resource": []string{
						fmt.Sprintf("%s/*", bucketArn),
					},
				},
				{
					"Sid":    "KMSStateAccess",
					"Effect": "Allow",
					"Action": []string{
						"kms:Decrypt",
						"kms:GenerateDataKey",
						"kms:DescribeKey",
						"kms:Encrypt",
						"kms:ReEncrypt*",
					},
					"Resource": kmsArn,
				},
				{
					"Sid":    "IAMCIUserManagement",
					"Effect": "Allow",
					"Action": []string{
						"iam:CreateUser",
						"iam:DeleteUser",
						"iam:GetUser",
						"iam:TagUser",
						"iam:UntagUser",
						"iam:CreatePolicy",
						"iam:CreatePolicyVersion",
						"iam:DeletePolicy",
						"iam:DeletePolicyVersion",
						"iam:GetPolicy",
						"iam:GetPolicyVersion",
						"iam:ListPolicyVersions",
						"iam:SetDefaultPolicyVersion",
						"iam:AttachUserPolicy",
						"iam:DetachUserPolicy",
						"iam:ListAttachedUserPolicies",
						"iam:CreateAccessKey",
						"iam:DeleteAccessKey",
						"iam:ListAccessKeys",
						"iam:TagPolicy",
						"iam:UntagPolicy",
					},
					"Resource": "*",
				},
				{
					"Sid":    "CloudWatchLogsAccess",
					"Effect": "Allow",
					"Action": []string{
						"logs:CreateLogGroup",
						"logs:DeleteLogGroup",
						"logs:DescribeLogGroups",
						"logs:PutRetentionPolicy",
						"logs:TagLogGroup",
						"logs:ListTagsLogGroup",
						"logs:ListTagsForResource",
						"logs:TagResource",
						"logs:UntagResource",
					},
					"Resource": "*",
				},
				{
					"Sid":    "ECSClusterManagement",
					"Effect": "Allow",
					"Action": []string{
						"ecs:CreateCluster",
						"ecs:DeleteCluster",
						"ecs:DescribeClusters",
						"ecs:TagResource",
						"ecs:UntagResource",
						"ecs:PutClusterCapacityProviders",
						"ecs:ListClusters",
					},
					"Resource": "*",
				},
				{
					"Sid":    "ELBManagement",
					"Effect": "Allow",
					"Action": []string{
						"elasticloadbalancing:CreateTargetGroup",
						"elasticloadbalancing:DeleteTargetGroup",
						"elasticloadbalancing:DescribeTargetGroups",
						"elasticloadbalancing:ModifyTargetGroup",
						"elasticloadbalancing:ModifyTargetGroupAttributes",
						"elasticloadbalancing:DescribeTargetGroupAttributes",
						"elasticloadbalancing:CreateRule",
						"elasticloadbalancing:DeleteRule",
						"elasticloadbalancing:DescribeRules",
						"elasticloadbalancing:ModifyRule",
						"elasticloadbalancing:DescribeListeners",
						"elasticloadbalancing:AddTags",
						"elasticloadbalancing:RemoveTags",
						"elasticloadbalancing:DescribeTags",
					},
					"Resource": "*",
				},
				{
					"Sid":    "ECRManagement",
					"Effect": "Allow",
					"Action": []string{
						"ecr:CreateRepository",
						"ecr:DeleteRepository",
						"ecr:DescribeRepositories",
						"ecr:GetRepositoryPolicy",
						"ecr:SetRepositoryPolicy",
						"ecr:PutImageTagMutability",
						"ecr:PutImageScanningConfiguration",
						"ecr:TagResource",
						"ecr:UntagResource",
						"ecr:ListTagsForResource",
						"ecr:PutLifecyclePolicy",
						"ecr:GetLifecyclePolicy",
						"ecr:DeleteLifecyclePolicy",
					},
					"Resource": "*",
				},
				{
					"Sid":    "SecretsManagerAccess",
					"Effect": "Allow",
					"Action": []string{
						"secretsmanager:CreateSecret",
						"secretsmanager:DeleteSecret",
						"secretsmanager:GetSecretValue",
						"secretsmanager:PutSecretValue",
						"secretsmanager:UpdateSecret",
						"secretsmanager:DescribeSecret",
						"secretsmanager:TagResource",
					},
					"Resource": "*",
				},
			},
		}
		jsonBytes, err := json.Marshal(doc)
		if err != nil {
			return "", err
		}
		return string(jsonBytes), nil
	}).(pulumi.StringOutput)

	executorPolicyName := fmt.Sprintf("%s-policy", args.RoleName)
	executorPolicy, err := iam.NewPolicy(ctx, executorPolicyName, &iam.PolicyArgs{
		Name:        pulumi.String(executorPolicyName),
		Description: pulumi.String("Grants access to Pulumi state S3 bucket and KMS key"),
		Policy:      ExecutorPolicyDoc,
		Tags: pulumi.StringMap{
			"Name": pulumi.String(executorPolicyName),
		},
	}, pulumi.Parent(self))
	if err != nil {
		return nil, fmt.Errorf("error creating IAM policy: %w", err)
	}

	// Attaching policy to role
	_, err = iam.NewRolePolicyAttachment(ctx, fmt.Sprintf("%s-attachment", executorPolicyName), &iam.RolePolicyAttachmentArgs{
		Role:      args.RoleName,
		PolicyArn: executorPolicy.Arn,
	}, pulumi.Parent(self))
	if err != nil {
		return nil, fmt.Errorf("error attaching policy to role: %w", err)
	}

	// Attach S3 Full Access policy to the executor role
	if args.AdditionalRole.S3Admin {
		s3PolicyAttachmentName := fmt.Sprintf("%v-s3-admin", name)
		_, err = iam.NewRolePolicyAttachment(ctx, s3PolicyAttachmentName, &iam.RolePolicyAttachmentArgs{
			Role:      args.RoleName,
			PolicyArn: pulumi.String("arn:aws:iam::aws:policy/AmazonS3FullAccess"),
		}, pulumi.Parent(self))
		if err != nil {
			return nil, fmt.Errorf("error attaching S3 admin policy to executor role: %w", err)
		}

	}

	_ = ctx.RegisterResourceOutputs(self, pulumi.Map{})

	return self, nil
}
