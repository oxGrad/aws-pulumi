// -------------------------------------------------------
// 1. KMS Key for S3 bucket encryption
// -------------------------------------------------------
package components

import (
	"fmt"

	"github.com/pulumi/pulumi-aws/sdk/v7/go/aws/kms"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type KeyAndAlias struct {
	pulumi.ResourceState
	KmsID  pulumi.StringOutput
	KmsARN pulumi.StringOutput
}

type KeyAndAliasArgs struct {
	KmsName       pulumi.StringInput
	BootstrapUser pulumi.StringInput
}

func NewKeyAndAlias(ctx *pulumi.Context, name string, args KeyAndAliasArgs, opts ...pulumi.ResourceOption) (*KeyAndAlias, error) {
	self := &KeyAndAlias{}
	err := ctx.RegisterComponentResource("bootstrap:kms:key-and-alias", name, self, opts...)
	if err != nil {
		return nil, err
	}

	kmsKey, err := kms.NewKey(ctx, name, &kms.KeyArgs{
		Description:          pulumi.String("Pulumi state S3 bucket encryption"),
		EnableKeyRotation:    pulumi.Bool(true),
		DeletionWindowInDays: pulumi.Int(30),
		// Policy:            pulumi.String(),
		Tags: pulumi.StringMap{
			"Environment": pulumi.String(ctx.Stack()),
			"ManagedBy":   args.BootstrapUser,
			"Team":        pulumi.String("SRE"),
		},
	}, pulumi.Parent(self))
	if err != nil {
		return nil, fmt.Errorf("error creating KMS key: %w", err)
	}
	self.KmsID = pulumi.StringOutput(kmsKey.ID())
	self.KmsARN = pulumi.StringOutput(kmsKey.Arn)

	_, err = kms.NewAlias(ctx, fmt.Sprintf("%s-alias", name), &kms.AliasArgs{
		Name:        pulumi.String(fmt.Sprintf("alias/%s", args.KmsName)),
		TargetKeyId: kmsKey.KeyId,
	}, pulumi.Parent(self))
	if err != nil {
		return nil, fmt.Errorf("error creating KMS key alias: %w", err)
	}

	_ = ctx.RegisterResourceOutputs(self, pulumi.Map{
		"KmsID":  self.KmsID,
		"KmsARN": self.KmsARN,
	})
	return self, nil
}
