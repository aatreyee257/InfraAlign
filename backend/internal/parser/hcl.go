package parser

import (
	"fmt"
	"github.com/hashicorp/terraform-config-inspect/tfconfig"
)

type BucketConfig struct {
	BucketName string
	IsEncrypted bool
}

func ParseBlueprint(dir string) (*BucketConfig, error) {
	module,diags := tfconfig.LoadModule(dir)
	if diags.HasErrors() {
		return nil, fmt.Errorf("failed to parse terraform: %s", diags.Error())
	}

	config := &BucketConfig{}

	for  _, resource := range module.ManagedResources {
		if resource.Type == "aws_S3_bucket" {
			config.BucketName = resource.Name
		}

		if resource.Type == "aws_s3_bucket_server_side_encryption_configuration" {
			config.IsEncrypted = true
		}

	}
	return config, nil
}