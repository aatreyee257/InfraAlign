package main

import (
    "context"
    "log"
    "github.com/aws/aws-sdk-go-v2/aws"
    "github.com/aws/aws-sdk-go-v2/config"
    "github.com/aws/aws-sdk-go-v2/service/s3"
)

func main(){
	//Load the AWS config  (Automatically reads the credentials you set up with 'aws')
	cfg,err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	//Create S3 client using the config
	client := s3.NewFromConfig(cfg)

	//Ask AWS to list all the buckets in your account
	bucketOutput, err := client.ListBuckets(context.TODO(), &s3.ListBucketsInput{})
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}
	for _,bucket := range bucketOutput.Buckets {
		bucketName := aws.ToString(bucket.Name)
		_,err := client.GetBucketEncryption(context.TODO(), &s3.GetBucketEncryptionInput{
			Bucket: aws.String(bucketName),
		})
		if err != nil {
			log.Printf("%s: encryption OFF", bucketName)
		}else {
			log.Printf("%s: encryption ON", bucketName)
		}
	}
}