// -------------------------------------------------------
// 5. KMS Key Policy — grant role access to the key
// -------------------------------------------------------
package components

import (
	"encoding/json"
	"fmt"

	"github.com/pulumi/pulumi-aws/sdk/v7/go/aws/kms"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type KeyPolicy struct {
	pulumi.ResourceState
}

type KeyPolicyArgs struct {
	AWSAccountID    pulumi.StringInput
	ExecutorRoleARN pulumi.StringInput
	KmsName         pulumi.StringInput
	KmsKeyID        pulumi.StringInput
	BootstrapUser   pulumi.StringInput
}

func NewKeyPolicy(ctx *pulumi.Context, name string, args *KeyPolicyArgs, opts ...pulumi.ResourceOption) (*KeyPolicy, error) {
	self := &KeyPolicy{}
	err := ctx.RegisterComponentResource("bootstrap:kms:state-key-policy", name, self, opts...)
	if err != nil {
		return nil, err
	}

	kmsKeyPolicy := pulumi.All(args.AWSAccountID, args.ExecutorRoleARN).ApplyT(func(args []interface{}) (string, error) {
		accountID := args[0]
		roleArn := args[1]

		doc := map[string]any{
			"Version": "2012-10-17",
			"Statement": []map[string]any{
				{
					"Sid":    "EnableRootAccess",
					"Effect": "Allow",
					"Principal": map[string]any{
						"AWS": fmt.Sprintf("arn:aws:iam::%s:root", accountID),
					},
					"Action":   "kms:*",
					"Resource": "*",
				},
				{
					"Sid":    "AllowPulumiExecutorRole",
					"Effect": "Allow",
					"Principal": map[string]any{
						"AWS": roleArn,
					},
					"Action": []string{
						"kms:Decrypt",
						"kms:GenerateDataKey",
						"kms:DescribeKey",
						"kms:Encrypt",
						"kms:ReEncrypt*",
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

	_, err = kms.NewKeyPolicy(ctx, fmt.Sprintf("%v-policy", args.KmsName), &kms.KeyPolicyArgs{
		KeyId:  args.KmsKeyID,
		Policy: kmsKeyPolicy,
	}, pulumi.Parent(self))
	if err != nil {
		return nil, fmt.Errorf("error setting KMS key policy: %w", err)
	}

	_ = ctx.RegisterResourceOutputs(self, pulumi.Map{})

	return self, nil
}
