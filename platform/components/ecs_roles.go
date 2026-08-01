package components

import (
	"encoding/json"
	"fmt"

	"github.com/pulumi/pulumi-aws/sdk/v7/go/aws/iam"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type ECSRoles struct {
	pulumi.ResourceState
	ExecutionRoleARN pulumi.StringOutput
	TaskRoleARN      pulumi.StringOutput
}

type ECSRolesArgs struct {
	ServiceName string
	Stack       string
	AccountID   string
	Region      string
	S3Buckets   []string
}

func NewECSRoles(ctx *pulumi.Context, name string, args *ECSRolesArgs, opts ...pulumi.ResourceOption) (*ECSRoles, error) {
	self := &ECSRoles{}
	if err := ctx.RegisterComponentResource("platform:ecs:roles", name, self, opts...); err != nil {
		return nil, err
	}

	assumeRolePolicy, err := json.Marshal(map[string]any{
		"Version": "2012-10-17",
		"Statement": []map[string]any{{
			"Effect":    "Allow",
			"Principal": map[string]string{"Service": "ecs-tasks.amazonaws.com"},
			"Action":    "sts:AssumeRole",
		}},
	})
	if err != nil {
		return nil, fmt.Errorf("error marshaling assume role policy: %w", err)
	}

	// --- Execution Role ---
	execRole, err := iam.NewRole(ctx, fmt.Sprintf("%s-exec", name), &iam.RoleArgs{
		Name:             pulumi.Sprintf("%s-%s-execution", args.ServiceName, args.Stack),
		Path:             pulumi.String("/ecs/"),
		AssumeRolePolicy: pulumi.String(string(assumeRolePolicy)),
		Tags:             pulumi.StringMap{"Service": pulumi.String(args.ServiceName)},
	}, pulumi.Parent(self))
	if err != nil {
		return nil, fmt.Errorf("error creating execution role: %w", err)
	}

	_, err = iam.NewRolePolicyAttachment(ctx, fmt.Sprintf("%s-exec-task-execution-policy", name), &iam.RolePolicyAttachmentArgs{
		Role:      execRole.Name,
		PolicyArn: pulumi.String("arn:aws:iam::aws:policy/service-role/AmazonECSTaskExecutionRolePolicy"),
	}, pulumi.Parent(self))
	if err != nil {
		return nil, fmt.Errorf("error attaching AmazonECSTaskExecutionRolePolicy: %w", err)
	}

	execInlinePolicy, err := json.Marshal(map[string]any{
		"Version": "2012-10-17",
		"Statement": []map[string]any{
			{
				"Effect":   "Allow",
				"Action":   []string{"ssm:GetParameters", "ssm:GetParameter"},
				"Resource": fmt.Sprintf("arn:aws:ssm:%s:%s:parameter/%s/%s/*", args.Region, args.AccountID, args.ServiceName, args.Stack),
			},
			{
				"Effect":   "Allow",
				"Action":   []string{"secretsmanager:GetSecretValue", "secretsmanager:DescribeSecret", "secretsmanager:ListSecretVersionIds"},
				"Resource": fmt.Sprintf("arn:aws:secretsmanager:%s:%s:secret:%s/%s/*", args.Region, args.AccountID, args.ServiceName, args.Stack),
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("error marshaling execution inline policy: %w", err)
	}

	_, err = iam.NewRolePolicy(ctx, fmt.Sprintf("%s-exec-inline", name), &iam.RolePolicyArgs{
		Role:   execRole.Name,
		Policy: pulumi.String(string(execInlinePolicy)),
	}, pulumi.Parent(self))
	if err != nil {
		return nil, fmt.Errorf("error creating execution inline policy: %w", err)
	}

	self.ExecutionRoleARN = execRole.Arn

	// --- Task Role ---
	taskRole, err := iam.NewRole(ctx, fmt.Sprintf("%s-task", name), &iam.RoleArgs{
		Name:             pulumi.Sprintf("%s-%s-task", args.ServiceName, args.Stack),
		Path:             pulumi.String("/ecs/"),
		AssumeRolePolicy: pulumi.String(string(assumeRolePolicy)),
		Tags:             pulumi.StringMap{"Service": pulumi.String(args.ServiceName)},
	}, pulumi.Parent(self))
	if err != nil {
		return nil, fmt.Errorf("error creating task role: %w", err)
	}

	for suffix, policyARN := range map[string]string{
		"ecr-readonly": "arn:aws:iam::aws:policy/AmazonEC2ContainerRegistryReadOnly",
		"cw-logs":      "arn:aws:iam::aws:policy/CloudWatchLogsFullAccess",
	} {
		_, err = iam.NewRolePolicyAttachment(ctx, fmt.Sprintf("%s-task-%s", name, suffix), &iam.RolePolicyAttachmentArgs{
			Role:      taskRole.Name,
			PolicyArn: pulumi.String(policyARN),
		}, pulumi.Parent(self))
		if err != nil {
			return nil, fmt.Errorf("error attaching task policy %s: %w", suffix, err)
		}
	}

	taskStatements := []map[string]any{
		{
			"Effect":   "Allow",
			"Action":   []string{"secretsmanager:GetSecretValue", "secretsmanager:DescribeSecret", "secretsmanager:ListSecretVersionIds"},
			"Resource": fmt.Sprintf("arn:aws:secretsmanager:%s:%s:secret:%s/%s/*", args.Region, args.AccountID, args.ServiceName, args.Stack),
		},
		{
			"Effect":   "Allow",
			"Action":   []string{"xray:PutTraceSegments", "xray:PutTelemetryRecords"},
			"Resource": "*",
		},
		{
			"Effect":   "Allow",
			"Action":   []string{"logs:PutLogEvents"},
			"Resource": "*",
		},
		{
			"Effect":   "Allow",
			"Action":   []string{"ssm:GetParameter"},
			"Resource": fmt.Sprintf("arn:aws:ssm:%s:%s:parameter/%s/%s/*", args.Region, args.AccountID, args.ServiceName, args.Stack),
		},
		{
			"Effect": "Allow",
			"Action": []string{
				"sqs:SendMessage",
				"sqs:ReceiveMessage",
				"sqs:DeleteMessage",
				"sqs:ChangeMessageVisibility",
				"sqs:GetQueueAttributes",
				"sqs:GetQueueUrl",
			},
			"Resource": fmt.Sprintf("arn:aws:sqs:%s:%s:*", args.Region, args.AccountID),
		},
		{
			"Effect": "Allow",
			"Action": []string{
				"sns:Publish",
			},
			"Resource": fmt.Sprintf("arn:aws:sns:%s:%s:%s-%s-*", args.Region, args.AccountID, args.ServiceName, args.Stack),
		},
		{
			"Effect":   "Allow",
			"Action":   []string{"sns:ListTopics"},
			"Resource": "*",
		},
	}

	if len(args.S3Buckets) > 0 {
		s3Resources := make([]string, 0, len(args.S3Buckets)*2)
		for _, bucket := range args.S3Buckets {
			s3Resources = append(s3Resources, fmt.Sprintf("arn:aws:s3:::%s", bucket))
			s3Resources = append(s3Resources, fmt.Sprintf("arn:aws:s3:::%s/*", bucket))
		}
		taskStatements = append(taskStatements, map[string]any{
			"Effect":   "Allow",
			"Action":   []string{"s3:GetObject", "s3:PutObject", "s3:ListBucket", "s3:GetBucketLocation"},
			"Resource": s3Resources,
		})
	}

	taskInlinePolicy, err := json.Marshal(map[string]any{
		"Version":   "2012-10-17",
		"Statement": taskStatements,
	})
	if err != nil {
		return nil, fmt.Errorf("error marshaling task inline policy: %w", err)
	}

	_, err = iam.NewRolePolicy(ctx, fmt.Sprintf("%s-task-inline", name), &iam.RolePolicyArgs{
		Role:   taskRole.Name,
		Policy: pulumi.String(string(taskInlinePolicy)),
	}, pulumi.Parent(self))
	if err != nil {
		return nil, fmt.Errorf("error creating task inline policy: %w", err)
	}

	self.TaskRoleARN = taskRole.Arn

	_ = ctx.RegisterResourceOutputs(self, pulumi.Map{
		"ExecutionRoleARN": self.ExecutionRoleARN,
		"TaskRoleARN":      self.TaskRoleARN,
	})

	return self, nil
}
