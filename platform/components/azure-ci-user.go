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
	SecretARN pulumi.StringOutput
}

type AzureCIUserArgs struct {
	User              pulumi.StringInput
	KmsARN            pulumi.StringInput
	PlatformManagedBy pulumi.StringInput
	PlatformSource    pulumi.StringInput
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
			"Environment":    pulumi.String(ctx.Stack()),
			"ManagedBy":      args.PlatformManagedBy,
			"PlatformSource": args.PlatformSource,
			"Team":           pulumi.String("SRE"),
			"Purpose":        pulumi.String("Azure Pipelines CI/CD deployments"),
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
				"Sid":      "PassRoleToECS",
				"Effect":   "Allow",
				"Action":   "iam:PassRole",
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
			"ManagedBy":      args.PlatformManagedBy,
			"PlatformSource": args.PlatformSource,
			"Team":           pulumi.String("SRE"),
			"Purpose":        pulumi.String("CI/CD deployment"),
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

	accessKey, err := iam.NewAccessKey(ctx, fmt.Sprintf("%s-access-key", name), &iam.AccessKeyArgs{
		User:   args.User,
		Status: pulumi.String("Active"),
	}, pulumi.Parent(self), pulumi.AdditionalSecretOutputs([]string{"secret"}))
	if err != nil {
		return nil, fmt.Errorf("error creating access key: %w", err)
	}

	credentialsJSON := pulumi.All(accessKey.ID().ToStringOutput(), accessKey.Secret).ApplyT(func(vals []interface{}) (string, error) {
		creds := map[string]string{
			"access_key_id":     vals[0].(string),
			"access_key_secret": vals[1].(string),
		}
		b, err := json.Marshal(creds)
		if err != nil {
			return "", fmt.Errorf("error marshaling credentials: %w", err)
		}
		return string(b), nil
	}).(pulumi.StringOutput)

	secret, err := secretsmanager.NewSecret(ctx, fmt.Sprintf("%s-credentials", name), &secretsmanager.SecretArgs{
		Name:        pulumi.Sprintf("%s/access-key", args.User),
		Description: pulumi.Sprintf("%s user access key credentials", args.User),
		KmsKeyId:    args.KmsARN,
		Tags: pulumi.StringMap{
			"Environment":    pulumi.String(ctx.Stack()),
			"ManagedBy":      args.PlatformManagedBy,
			"PlatformSource": args.PlatformSource,
			"Team":           pulumi.String("SRE"),
			"Purpose":        pulumi.String("CI/CD deployment"),
		},
	}, pulumi.Parent(self))
	if err != nil {
		return nil, fmt.Errorf("error creating secret: %w", err)
	}
	self.SecretARN = secret.Arn

	_, err = secretsmanager.NewSecretVersion(ctx, fmt.Sprintf("%s-credentials-version", name), &secretsmanager.SecretVersionArgs{
		SecretId:     secret.ID(),
		SecretString: credentialsJSON,
	}, pulumi.Parent(self))
	if err != nil {
		return nil, fmt.Errorf("error creating secret version: %w", err)
	}

	_ = ctx.RegisterResourceOutputs(self, pulumi.Map{
		"SecretARN": self.SecretARN,
	})

	return self, nil
}
