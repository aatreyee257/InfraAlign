package aws

import (
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
)

type BucketConfig struct {
	BucketName  string
	IsEncrypted bool
	IsVersioned bool
}

func ScanBuckets() ([]BucketConfig, error) {
	var buckets []BucketConfig

	// Load the AWS config (automatically reads credentials set up with 'aws')
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	client := s3.NewFromConfig(cfg)

	bucketOutput, err := client.ListBuckets(context.TODO(), &s3.ListBucketsInput{})
	if err != nil {
		return nil, fmt.Errorf("failed to list buckets: %w", err)
	}

	for _, bucket := range bucketOutput.Buckets {
		bucketName := aws.ToString(bucket.Name)

		// --- encryption ---
		isEncrypted := true
		_, err := client.GetBucketEncryption(context.TODO(), &s3.GetBucketEncryptionInput{
			Bucket: aws.String(bucketName),
		})
		if err != nil {
			var apiErr smithy.APIError
			if errors.As(err, &apiErr) && apiErr.ErrorCode() == "ServerSideEncryptionConfigurationNotFoundError" {
				isEncrypted = false // only record the result; don't append here
			} else {
				return nil, fmt.Errorf("checking encryption for bucket %s: %w", bucketName, err)
			}
		}

		// --- versioning ---
		verOut, err := client.GetBucketVersioning(context.TODO(), &s3.GetBucketVersioningInput{
			Bucket: aws.String(bucketName),
		})
		if err != nil {
			return nil, fmt.Errorf("checking versioning for bucket %s: %w", bucketName, err)
		}
		isVersioned := verOut.Status == types.BucketVersioningStatusEnabled

		fmt.Printf("%s: encrypted=%v versioned=%v\n", bucketName, isEncrypted, isVersioned)

		// exactly one append per bucket
		buckets = append(buckets, BucketConfig{
			BucketName:  bucketName,
			IsEncrypted: isEncrypted,
			IsVersioned: isVersioned,
		})
	} // end for

	return buckets, nil
} // end ScanBuckets

func EnableServerEncryption(bucketName string) error {
	//Load AWS config  (Automatically reads the credentials you set up with 'aws')
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return fmt.Errorf("Failed to load configuration: %w", err)
	}

	//Create S3 client using the config
	client := s3.NewFromConfig(cfg)

	//Enable server-side encryption for the bucket
	_, err = client.PutBucketEncryption(context.TODO(), &s3.PutBucketEncryptionInput{
		Bucket: aws.String(bucketName),
		ServerSideEncryptionConfiguration: &types.ServerSideEncryptionConfiguration{
			Rules: []types.ServerSideEncryptionRule{
				{
					ApplyServerSideEncryptionByDefault: &types.ServerSideEncryptionByDefault{
						SSEAlgorithm: types.ServerSideEncryptionAes256,
					},
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("Failed to enable server-side encryption for bucket %s: %w", bucketName, err)
	}

	fmt.Printf("Server-side encryption enabled for bucket %s\n", bucketName)
	return nil
}

func EnableVersioning(bucketName string) error {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}
	client := s3.NewFromConfig(cfg)

	_, err = client.PutBucketVersioning(context.TODO(), &s3.PutBucketVersioningInput{
		Bucket: aws.String(bucketName),
		VersioningConfiguration: &types.VersioningConfiguration{
			Status: types.BucketVersioningStatusEnabled,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to enable versioning for bucket %s: %w", bucketName, err)
	}
	return nil
}
