package components

import (
	"fmt"

	"github.com/pulumi/pulumi-aws/sdk/v7/go/aws/s3"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type StateBucket struct {
	pulumi.ResourceState
	ID  pulumi.StringOutput
	ARN pulumi.StringOutput
}

type StateBucketArgs struct {
	BucketName    pulumi.StringInput
	KmsArn        pulumi.StringInput
	BootstrapUser pulumi.StringInput
}

func NewStateBucket(ctx *pulumi.Context, name string, args *StateBucketArgs, opts ...pulumi.ResourceOption) (*StateBucket, error) {
	self := &StateBucket{}
	err := ctx.RegisterComponentResource("bootstrap:s3:state-bucket", name, self, opts...)
	if err != nil {
		return nil, err
	}

	// Create an AWS resource (S3 Bucket)
	bucket, err := s3.NewBucketV2(ctx, name, &s3.BucketV2Args{
		Bucket:       args.BucketName,
		ForceDestroy: pulumi.Bool(false),
		Tags: pulumi.StringMap{
			"Environment": pulumi.String(ctx.Stack()),
			"ManagedBy":   args.BootstrapUser,
			"Team":        pulumi.String("SRE"),
		},
	}, pulumi.Parent(self))
	if err != nil {
		return nil, fmt.Errorf("error creating bucket: %w", err)
	}
	self.ID = pulumi.StringOutput(bucket.ID())
	self.ARN = pulumi.StringOutput(bucket.Arn)

	// Block public access
	_, err = s3.NewBucketPublicAccessBlock(ctx, "public-access-block", &s3.BucketPublicAccessBlockArgs{
		Bucket:                bucket.ID(),
		BlockPublicAcls:       pulumi.Bool(true),
		IgnorePublicAcls:      pulumi.Bool(true),
		BlockPublicPolicy:     pulumi.Bool(true),
		RestrictPublicBuckets: pulumi.Bool(true),
	}, pulumi.Parent(self))
	if err != nil {
		return nil, fmt.Errorf("error creating bucket public access block: %w", err)
	}

	// Enable bucket versioning
	_, err = s3.NewBucketVersioningV2(ctx, fmt.Sprintf("%s-versioning", args.BucketName), &s3.BucketVersioningV2Args{
		Bucket: bucket.ID(),
		VersioningConfiguration: s3.BucketVersioningV2VersioningConfigurationArgs{
			Status: pulumi.String("Enabled"),
		},
	}, pulumi.Parent(self))
	if err != nil {
		return nil, fmt.Errorf("error enabling bucket versioning: %w", err)
	}

	_, err = s3.NewBucketServerSideEncryptionConfigurationV2(ctx, fmt.Sprintf("%s-sse", args.BucketName), &s3.BucketServerSideEncryptionConfigurationV2Args{
		Bucket: bucket.ID(),
		Rules: s3.BucketServerSideEncryptionConfigurationV2RuleArray{
			s3.BucketServerSideEncryptionConfigurationV2RuleArgs{
				ApplyServerSideEncryptionByDefault: &s3.BucketServerSideEncryptionConfigurationV2RuleApplyServerSideEncryptionByDefaultArgs{
					SseAlgorithm:   pulumi.String("aws:kms"),
					KmsMasterKeyId: args.KmsArn,
				},
				BucketKeyEnabled: pulumi.Bool(true),
			},
		},
	}, pulumi.Parent(self))
	if err != nil {
		return nil, fmt.Errorf("error configuring bucket encryption: %w", err)
	}

	_ = ctx.RegisterResourceOutputs(self, pulumi.Map{
		"ID":  self.ID,
		"ARN": self.ARN,
	})
	return self, nil
}
