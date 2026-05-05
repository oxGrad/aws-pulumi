package components

import (
	"fmt"

	"github.com/pulumi/pulumi-aws/sdk/v7/go/aws/ecr"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type ECRRepository struct {
	pulumi.ResourceState
	URL pulumi.StringOutput
	ARN pulumi.StringOutput
}

type ECRRepositoryArgs struct {
	Name pulumi.StringInput
}

func NewECRRepository(ctx *pulumi.Context, name string, args *ECRRepositoryArgs, opts ...pulumi.ResourceOption) (*ECRRepository, error) {
	self := &ECRRepository{}
	err := ctx.RegisterComponentResource("platform:ecr:repository", name, self, opts...)
	if err != nil {
		return nil, err
	}

	repo, err := ecr.NewRepository(ctx, name, &ecr.RepositoryArgs{
		Name:               args.Name,
		ImageTagMutability: pulumi.String("IMMUTABLE"),
		ImageScanningConfiguration: &ecr.RepositoryImageScanningConfigurationArgs{
			ScanOnPush: pulumi.Bool(true),
		},
		EncryptionConfigurations: ecr.RepositoryEncryptionConfigurationArray{
			&ecr.RepositoryEncryptionConfigurationArgs{
				EncryptionType: pulumi.String("AES256"),
			},
		},
		Tags: pulumi.StringMap{
			"Name":    args.Name,
			"Service": args.Name,
		},
	}, pulumi.Parent(self))
	if err != nil {
		return nil, fmt.Errorf("error creating ECR repository: %w", err)
	}

	self.URL = repo.RepositoryUrl
	self.ARN = repo.Arn

	_ = ctx.RegisterResourceOutputs(self, pulumi.Map{
		"URL": self.URL,
		"ARN": self.ARN,
	})

	return self, nil
}
