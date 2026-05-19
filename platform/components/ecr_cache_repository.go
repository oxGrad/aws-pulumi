package components

import (
	"encoding/json"
	"fmt"

	"github.com/pulumi/pulumi-aws/sdk/v7/go/aws/ecr"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type BuildCacheECRRepository struct {
	pulumi.ResourceState
	URL pulumi.StringOutput
	ARN pulumi.StringOutput
}

type BuildCacheECRRepositoryArgs struct {
	Name pulumi.StringInput
}

// NewBuildCacheECRRepository creates a shared mutable ECR repository for Docker layer caches.
// Tags follow the pattern "{service}-{env}-cache" and are overwritten on every build.
func NewBuildCacheECRRepository(ctx *pulumi.Context, name string, args *BuildCacheECRRepositoryArgs, opts ...pulumi.ResourceOption) (*BuildCacheECRRepository, error) {
	self := &BuildCacheECRRepository{}
	err := ctx.RegisterComponentResource("platform:ecr:build-cache-repository", name, self, opts...)
	if err != nil {
		return nil, err
	}

	repo, err := ecr.NewRepository(ctx, name, &ecr.RepositoryArgs{
		Name:               args.Name,
		ImageTagMutability: pulumi.String("MUTABLE"),
		ImageScanningConfiguration: &ecr.RepositoryImageScanningConfigurationArgs{
			ScanOnPush: pulumi.Bool(false),
		},
		EncryptionConfigurations: ecr.RepositoryEncryptionConfigurationArray{
			&ecr.RepositoryEncryptionConfigurationArgs{
				EncryptionType: pulumi.String("AES256"),
			},
		},
		Tags: pulumi.StringMap{
			"Name":    args.Name,
			"Purpose": pulumi.String("docker-layer-cache"),
		},
	}, pulumi.Parent(self))
	if err != nil {
		return nil, fmt.Errorf("error creating build cache ECR repository: %w", err)
	}

	if err := attachCacheLifecyclePolicy(ctx, name, repo, self); err != nil {
		return nil, err
	}

	self.URL = repo.RepositoryUrl
	self.ARN = repo.Arn

	_ = ctx.RegisterResourceOutputs(self, pulumi.Map{
		"URL": self.URL,
		"ARN": self.ARN,
	})

	return self, nil
}

func attachCacheLifecyclePolicy(ctx *pulumi.Context, name string, repo *ecr.Repository, parent pulumi.Resource) error {
	policy, err := json.Marshal(map[string]any{
		"rules": []map[string]any{
			{
				"rulePriority": 1,
				"description":  "Expire untagged (displaced) cache images after 1 day",
				"selection": map[string]any{
					"tagStatus":   "untagged",
					"countType":   "sinceImagePushed",
					"countUnit":   "days",
					"countNumber": 1,
				},
				"action": map[string]string{"type": "expire"},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("error marshaling cache lifecycle policy: %w", err)
	}

	_, err = ecr.NewLifecyclePolicy(ctx, fmt.Sprintf("%s-lifecycle", name), &ecr.LifecyclePolicyArgs{
		Repository: repo.Name,
		Policy:     pulumi.String(string(policy)),
	}, pulumi.Parent(parent))
	return err
}
