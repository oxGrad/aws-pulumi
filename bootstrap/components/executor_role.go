package components

import (
	"encoding/json"
	"fmt"

	"github.com/pulumi/pulumi-aws/sdk/v7/go/aws/iam"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type ExecutorRole struct {
	pulumi.ResourceState
	ARN pulumi.StringOutput
}

type ExecutorRoleArgs struct {
	AWSAccountID pulumi.StringInput
	Role         pulumi.StringInput
}

func NewExecutorRole(ctx *pulumi.Context, name string, args *ExecutorRoleArgs, opts ...pulumi.ResourceOption) (*ExecutorRole, error) {
	self := &ExecutorRole{}
	err := ctx.RegisterComponentResource("bootstrap:role:executor", name, self, opts...)
	if err != nil {
		return nil, err
	}

	assumeRolePolicyDoc := pulumi.All(args.AWSAccountID).ApplyT(func(args []any) (string, error) {
		accountID := args[0].(string)
		doc := map[string]any{
			"Version": "2012-10-17",
			"Statement": []map[string]any{
				{
					"Effect": "Allow",
					"Principal": map[string]any{
						"AWS": fmt.Sprintf("arn:aws:iam::%s:root", accountID),
					},
					"Action": "sts:AssumeRole",
				},
			},
		}
		jsonBytes, err := json.Marshal(doc)
		if err != nil {
			return "", err
		}
		return string(jsonBytes), nil
	}).(pulumi.StringOutput)

	executorRole, err := iam.NewRole(ctx, name, &iam.RoleArgs{
		Name:             args.Role,
		Description:      pulumi.String("Role for Pulumi CI/CD executor with access to state bucket and KMS key"),
		AssumeRolePolicy: assumeRolePolicyDoc,
		Tags: pulumi.StringMap{
			"Name": pulumi.String("pulumi-executor"),
		},
	}, pulumi.Parent(self))
	if err != nil {
		return nil, fmt.Errorf("error creating IAM role: %w", err)
	}
	self.ARN = pulumi.StringOutput(executorRole.Arn)

	_ = ctx.RegisterResourceOutputs(self, pulumi.Map{
		"ARN": self.ARN,
	})

	return self, nil
}
