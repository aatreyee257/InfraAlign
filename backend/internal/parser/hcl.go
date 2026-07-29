package parser

import (
	"fmt"
	"github.com/hashicorp/terraform-config-inspect/tfconfig"
)

type BucketConfig struct {
	BucketName string
	IsEncrypted bool
}

func ParseBlueprint(dir string) ([]BucketConfig, error) {
	module,diags := tfconfig.LoadModule(dir)
	if diags.HasErrors() {
		return nil, fmt.Errorf("Failed to parse terraform: %s", diags.Error())
	}

	//internal map to hold bucket configs
	bucketMap := map[string]*BucketConfig{}

	//find all aws_s3_bucket resources and add them to the map
	for _,resource := range module.ManagedResources {
		if resource.Type == "aws_s3_bucket" {
			bucketMap[resource.Name] = &BucketConfig{
				BucketName: resource.Name,
				IsEncrypted: false,
			}
		}
	}

	//find encryption blocks and update matching buckets
	for _, resource := range module.ManagedResources {
		if resource.Type == "aws_s3_bucket_server_side_encryption_configuration" {
			if  entry, exists := bucketMap[resource.Name];exists {
				entry.IsEncrypted = true
			}
		}
	}

	//converting map to slice
	var buckets []BucketConfig
	for _,entry := range bucketMap {
		buckets = append(buckets, *entry)
	}
	
	return buckets, nil
}