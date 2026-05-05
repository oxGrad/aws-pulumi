package components

import (
	"encoding/json"
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
		ImageTagMutability: pulumi.String("IMMUTABLE"), // NOTE: 'latest' image tag forbidden
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

	if err := attachLifecyclePolicy(ctx, name, repo, self); err != nil {
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

func attachLifecyclePolicy(ctx *pulumi.Context, name string, repo *ecr.Repository, parent pulumi.Resource) error {
	// Dev images: "{short-hash}-{YYYYMMDD}T{HHMM}" — the T is the discriminator.
	// Prod images: semver stripped of v-prefix, e.g. "1.2.3".
	policy, err := json.Marshal(map[string]any{
		"rules": []map[string]any{
			{
				"rulePriority": 1,
				"description":  "Keep last 10 development images (hash-timestampT format)",
				"selection": map[string]any{
					"tagStatus":      "tagged",
					"tagPatternList": []string{"*T*"},
					"countType":      "imageCountMoreThan",
					"countNumber":    10,
				},
				"action": map[string]string{"type": "expire"},
			},
			{
				"rulePriority": 2,
				"description":  "Keep last 10 production images (semver x.y.z format)",
				"selection": map[string]any{
					"tagStatus":      "tagged",
					"tagPatternList": []string{"*.*.*"},
					"countType":      "imageCountMoreThan",
					"countNumber":    10,
				},
				"action": map[string]string{"type": "expire"},
			},
			{
				"rulePriority": 3,
				"description":  "Expire untagged images after 1 day",
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
		return fmt.Errorf("error marshaling lifecycle policy: %w", err)
	}

	_, err = ecr.NewLifecyclePolicy(ctx, fmt.Sprintf("%s-lifecycle", name), &ecr.LifecyclePolicyArgs{
		Repository: repo.Name,
		Policy:     pulumi.String(string(policy)),
	}, pulumi.Parent(parent))
	return err
}
