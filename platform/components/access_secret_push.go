package components

import (
	"encoding/json"
	"fmt"

	"github.com/pulumi/pulumi-aws/sdk/v7/go/aws/iam"
	"github.com/pulumi/pulumi-aws/sdk/v7/go/aws/secretsmanager"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type PushAccessKeyToSecretManager struct {
	pulumi.ResourceState
	SecretARN pulumi.StringOutput
}

type PushAccessKeyToSecretManagerArgs struct {
	Username pulumi.StringInput // Output from create user component
	KmsARN   pulumi.StringInput
}

func NewPushAccessKeyToSecretManager(ctx *pulumi.Context, name string, args *PushAccessKeyToSecretManagerArgs, opts ...pulumi.ResourceOption) (*PushAccessKeyToSecretManager, error) {
	self := &PushAccessKeyToSecretManager{}
	err := ctx.RegisterComponentResource("platform:iam:push-access-key-to-secret-manager", name, self, opts...)
	if err != nil {
		return nil, err
	}

	accessKey, err := iam.NewAccessKey(ctx, fmt.Sprintf("%s-access-key", name), &iam.AccessKeyArgs{
		User:   args.Username,
		Status: pulumi.String("Active"),
	}, pulumi.Parent(self), pulumi.AdditionalSecretOutputs([]string{"secret"}))
	if err != nil {
		return nil, fmt.Errorf("error creating access key: %w", err)
	}

	credentialsJSON := pulumi.All(accessKey.ID().ToStringOutput(), accessKey.Secret).ApplyT(func(vals []any) (string, error) {
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
		Name:        pulumi.Sprintf("%s/access-key", args.Username),
		Description: pulumi.Sprintf("%s user access key credentials", args.Username),
		KmsKeyId:    args.KmsARN,
		Tags: pulumi.StringMap{
			"Purpose": PURPOSE,
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
