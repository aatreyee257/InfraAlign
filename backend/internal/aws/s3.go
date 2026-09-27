package aws

import (
    "context"
	"fmt"
	"errors"
    
    "github.com/aws/aws-sdk-go-v2/aws"
    "github.com/aws/aws-sdk-go-v2/config"
    "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
)

type BucketConfig struct {
	BucketName string
	IsEncrypted bool
}

func ScanBuckets() ([]BucketConfig, error) {
	var buckets []BucketConfig

	//Load the AWS config  (Automatically reads the credentials you set up with 'aws')
	cfg,err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return nil, fmt.Errorf("Failed to load configuration: %w", err)
	}

	//Create S3 client using the config
	client := s3.NewFromConfig(cfg)

	//Ask AWS to list all the buckets in your account
	bucketOutput, err := client.ListBuckets(context.TODO(), &s3.ListBucketsInput{})
	if err != nil {
		return nil, fmt.Errorf("failed to list buckets: %w", err)
	}
	
	for _,bucket := range bucketOutput.Buckets {
		bucketName := aws.ToString(bucket.Name)
		_, err := client.GetBucketEncryption(context.TODO(), &s3.GetBucketEncryptionInput{
    Bucket: aws.String(bucketName),
})

if err != nil {
	var apiErr smithy.APIError
    if errors.As(err, &apiErr) && apiErr.ErrorCode() == "ServerSideEncryptionConfigurationNotFoundError"{
        fmt.Printf("%s: encryption OFF\n", bucketName)
        buckets = append(buckets, BucketConfig{BucketName: bucketName, IsEncrypted: false})
    } else {
        return nil, fmt.Errorf("checking encryption for bucket %s: %w", bucketName, err)
    }
} else {
    fmt.Printf("%s: encryption ON\n", bucketName)
    buckets = append(buckets, BucketConfig{BucketName: bucketName, IsEncrypted: true})
}
	}
	return buckets, nil
}

func EnableServerEncryption(bucketName string) error {
	//Load AWS config  (Automatically reads the credentials you set up with 'aws')
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return  fmt.Errorf("Failed to load configuration: %w", err)
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


