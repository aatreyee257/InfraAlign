package aws

import (
    "context"
	"fmt"
    
    "github.com/aws/aws-sdk-go-v2/aws"
    "github.com/aws/aws-sdk-go-v2/config"
    "github.com/aws/aws-sdk-go-v2/service/s3"
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
		_,err := client.GetBucketEncryption(context.TODO(), &s3.GetBucketEncryptionInput{
			Bucket: aws.String(bucketName),
		})
		if err != nil {
			fmt.Printf("%s: encryption OFF", bucketName)
			buckets = append(buckets, BucketConfig{
				BucketName: bucketName,
				IsEncrypted: false,
			})
		}else {
			fmt.Printf("%s: encryption ON", bucketName)
			buckets = append(buckets, BucketConfig{
				BucketName: bucketName,
				IsEncrypted: true,
			})
		}
	}
	return buckets, nil
}