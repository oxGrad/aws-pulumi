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
	BootstrapUser  pulumi.StringInput
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
			"Name":      pulumi.String(executorPolicyName),
			"ManagedBy": args.BootstrapUser,
			"Team":      pulumi.String("SRE"),
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
