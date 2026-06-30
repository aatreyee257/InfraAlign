package aws

import (
    "context"
	"fmt"
    
    "github.com/aws/aws-sdk-go-v2/aws"
    "github.com/aws/aws-sdk-go-v2/config"
    "github.com/aws/aws-sdk-go-v2/service/s3"
)

func ScanBuckets() error{
	
	//Load the AWS config  (Automatically reads the credentials you set up with 'aws')
	cfg,err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return fmt.Errorf("Failed to load configuration: %w", err)
	}

	//Create S3 client using the config
	client := s3.NewFromConfig(cfg)

	//Ask AWS to list all the buckets in your account
	bucketOutput, err := client.ListBuckets(context.TODO(), &s3.ListBucketsInput{})
	if err != nil {
		return fmt.Errorf("failed to list buckets: %w", err)
	}
	
	for _,bucket := range bucketOutput.Buckets {
		bucketName := aws.ToString(bucket.Name)
		_,err := client.GetBucketEncryption(context.TODO(), &s3.GetBucketEncryptionInput{
			Bucket: aws.String(bucketName),
		})
		if err != nil {
			fmt.Printf("%s: encryption OFF", bucketName)
		}else {
			fmt.Printf("%s: encryption ON", bucketName)
		}
	}
	return nil
}