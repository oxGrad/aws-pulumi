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
	ARN pulumi.StringOutput
}

type PushAccessKeyToSecretManagerArgs struct {
	User          pulumi.StringInput
	KmsARN        pulumi.StringInput
	BootstrapUser pulumi.StringInput
}

func NewPushAccessKeyToSecretManager(ctx *pulumi.Context, name string, args *PushAccessKeyToSecretManagerArgs, opts ...pulumi.ResourceOption) (*PushAccessKeyToSecretManager, error) {
	self := &PushAccessKeyToSecretManager{}
	err := ctx.RegisterComponentResource("bootstrap:iam:generate-and-push-access-key", name, self, opts...)
	if err != nil {
		return nil, err
	}

	accessKeyName := fmt.Sprintf("%s-access-key", name)
	accessKey, err := iam.NewAccessKey(ctx, accessKeyName, &iam.AccessKeyArgs{
		User:   pulumi.String(name),
		Status: pulumi.String("Active"), // default "Active"
	},
		pulumi.Parent(self),
		pulumi.AdditionalSecretOutputs([]string{"secret"}),
	)
	if err != nil {
		return nil, err
	}

	credentialsJSON := pulumi.All(accessKey.ID().ToStringOutput(), accessKey.Secret).ApplyT(func(args []interface{}) (string, error) {
		accessKeyID := args[0].(string)
		accessKeySecret := args[1].(string)

		creds := map[string]string{
			"access_key_id":     accessKeyID,
			"access_key_secret": accessKeySecret,
		}

		jsonBytes, err := json.Marshal(creds)
		if err != nil {
			return "", fmt.Errorf("error marshal credentials to json: %w", err)
		}

		return string(jsonBytes), nil
	}).(pulumi.StringOutput)

	// Store in Secrets Manager
	secret, err := secretsmanager.NewSecret(ctx, fmt.Sprintf("%s-credentials", name), &secretsmanager.SecretArgs{
		Name:        pulumi.String(fmt.Sprintf("%s/access-key", name)),
		Description: pulumi.String(fmt.Sprintf("%s user access key credentials", name)),
		KmsKeyId:    args.KmsARN,
		Tags: pulumi.StringMap{
			"Environment": pulumi.String(ctx.Stack()),
			"ManagedBy":   args.BootstrapUser,
			"Team":        pulumi.String("SRE"),
		},
	}, pulumi.Parent(self))
	if err != nil {
		return nil, fmt.Errorf("error creating secret: %w", err)
	}
	self.ARN = secret.Arn

	_, err = secretsmanager.NewSecretVersion(ctx, fmt.Sprintf("%s-credentials-version", name), &secretsmanager.SecretVersionArgs{
		SecretId:     secret.ID(),
		SecretString: credentialsJSON,
	}, pulumi.Parent(self))
	if err != nil {
		return nil, fmt.Errorf("error creating secret version: %w", err)
	}

	// _, err = iam.GetAccessKey(ctx, accessKeyName, ciAccessKey.ID(), &iam.AccessKeyState{
	// 	User: pulumi.String(name),
	// }, pulumi.Parent(self))
	// if err != nil {
	// 	return nil, err
	// }
	_ = ctx.RegisterResourceOutputs(self, pulumi.Map{
		"ARN": self.ARN,
	})

	return self, nil
}
