package parser

import (
	"fmt"
	"github.com/hashicorp/terraform-config-inspect/tfconfig"
	"github.com/hashicorp/hcl/v2"
    "github.com/hashicorp/hcl/v2/hclparse"
)

type BucketConfig struct {
	BucketName string
	IsEncrypted bool
}

//given a .tf file path and the resource block's name, find its bucket.
func resolveBucketReference(filePath string, blockName string) (string,error) {
	parser := hclparse.NewParser()
	file, diags := parser.ParseHCLFile(filePath)
	if diags.HasErrors() {
		return "", diags
	}

	//Step 1: Describe the shape
	schema := &hcl.BodySchema{
    Blocks: []hcl.BlockHeaderSchema{
        {
            Type:       "resource",
            LabelNames: []string{"type", "name"},
        },
    },
}

content, diags := file.Body.Content(schema)
if diags.HasErrors() {
    return "", diags
}

	// Step 2: walk the resource blocks it found, and pick out the one
    // matching our type + name.
    for _, block := range content.Blocks {
        resourceType := block.Labels[0] // e.g. "aws_s3_bucket_server_side_encryption_configuration"
        resourceName := block.Labels[1] // e.g. "mybucket"

        if resourceType == "aws_s3_bucket_server_side_encryption_configuration" && resourceName == blockName {
            // Step 3: now we're INSIDE that one block's body — ask it
            // for its "bucket" attribute specifically.
            innerSchema := &hcl.BodySchema{
                Attributes: []hcl.AttributeSchema{{Name: "bucket", Required: true}},
            }
            innerContent, _, diags := block.Body.PartialContent(innerSchema)
            if diags.HasErrors() {
                return "", diags
            }

            attr := innerContent.Attributes["bucket"]

            // Step 4: the attribute's value is an EXPRESSION
            // (aws_s3_bucket.mybucket.id), not a plain string — this is
            // the call that turns it into a traversal we can read.
            traversal, diags := hcl.AbsTraversalForExpr(attr.Expr)
            if diags.HasErrors() {
                return "", diags
            }

            // traversal[0] = "aws_s3_bucket" (the resource type)
            // traversal[1] = "mybucket"      (the resource NAME — what we want)
            return traversal[1].(hcl.TraverseAttr).Name, nil
        }
    }

    return "", fmt.Errorf("no resource named %s found in %s", blockName, filePath)
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
		if resource.Type != "aws_s3_bucket_server_side_encryption_configuration" {
			continue
		}

		realBucketName, err := resolveBucketReference(resource.Pos.Filename, resource.Name)
		if err != nil {
			return nil, fmt.Errorf("Failed to resolve bucket reference for %s: %v", resource.Name, err)
		}

		if entry, exists := bucketMap[realBucketName]; exists {
			entry.IsEncrypted = true
		}
	}

	//converting map to slice
	var buckets []BucketConfig
	for _,entry := range bucketMap {
		buckets = append(buckets, *entry)
	}
	
	return buckets, nil
}