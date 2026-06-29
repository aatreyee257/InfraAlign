package main

import "github.com/aws/aws-sdk-go-v2/config"

func main()
{
	//Load the AWS config  (Automatically reads the credentials you set up with 'aws')
	cfg,err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		log.Fatal("Failed to load configuration, %v", err)
	}

	//Create S3 client using the config
	client := s3Client.NewFromConfig(cfg)

	//Ask AWS to list all buckets
	output, err := client.ListObjectsV2(context.TODO(), &s3.ListObjectsV2Input{
		Bucket: aws.String("amzn-s3-demo-bucket"),
	})
	if err!= nil {
		log.Fatal("Failed to load configuration, %v", err)
	}

	log.Println("first page results")
	for _, object := range output.Contents {
		log.Printf("key=%s size=%d", aws.ToString(object.Key), *object.Size)
	}
}