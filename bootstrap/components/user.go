package components

import (
	"encoding/json"
	"fmt"

	"github.com/pulumi/pulumi-aws/sdk/v7/go/aws/iam"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type CIUser struct {
	pulumi.ResourceState
}

type CIUserArgs struct {
	User            pulumi.StringInput // ciUser
	ExecutorRoleARN pulumi.StringInput
	BootstrapUser   pulumi.StringInput
}

func NewCIUser(ctx *pulumi.Context, name string, args *CIUserArgs, opts ...pulumi.ResourceOption) (*CIUser, error) {
	self := &CIUser{}
	err := ctx.RegisterComponentResource("bootstrap:iam:ci-user", name, self, opts...)
	if err != nil {
		return nil, err
	}

	// Create pulumi user for CI
	_, err = iam.NewUser(ctx, name, &iam.UserArgs{
		Name: args.User,
		Path: pulumi.String("/ci/"),
		Tags: pulumi.StringMap{
			"Environment": pulumi.String(ctx.Stack()),
			"ManagedBy":   args.BootstrapUser,
			"Team":        pulumi.String("SRE"),
			"Purpose":     pulumi.String("CI/CD Pulumi state access"),
		},
	}, pulumi.Parent(self))
	if err != nil {
		return nil, fmt.Errorf("error creating IAM user: %w", err)
	}

	// Inline policy: allow pulumi-ci user to assume pulumi-executor role
	assumeRoleUserPolicyDoc := pulumi.All(args.ExecutorRoleARN).ApplyT(func(args []interface{}) (string, error) {
		roleArn := args[0]

		doc := map[string]any{
			"Version": "2012-10-17",
			"Statement": []map[string]any{
				{
					"Sid":      "AssumeExecutorRole",
					"Effect":   "Allow",
					"Action":   "sts:AssumeRole",
					"Resource": roleArn,
				},
			},
		}
		jsonBytes, err := json.Marshal(doc)
		if err != nil {
			return "", err
		}
		return string(jsonBytes), nil
	}).(pulumi.StringOutput)

	userPolicyName := fmt.Sprintf("%v-assume-role-policy", args.User)
	_, err = iam.NewUserPolicy(ctx, userPolicyName, &iam.UserPolicyArgs{
		Name:   pulumi.String(userPolicyName),
		User:   args.User,
		Policy: assumeRoleUserPolicyDoc,
	}, pulumi.Parent(self))
	if err != nil {
		return nil, fmt.Errorf("error attaching inline policy to user: %w", err)
	}

	_ = ctx.RegisterResourceOutputs(self, pulumi.Map{})

	return self, nil
}
