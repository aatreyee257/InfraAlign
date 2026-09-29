package parser

import (
	"fmt"
	"sort"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/terraform-config-inspect/tfconfig"
	"github.com/zclconf/go-cty/cty"
)

type BucketConfig struct {
	BucketName  string // the REAL AWS bucket name (from the `bucket = "..."` attribute)
	IsEncrypted bool
	IsVersioned bool
}

// findResourceBlock parses one .tf file and returns the resource block
// with the given type and name.
func findResourceBlock(filePath, resourceType, blockName string) (*hcl.Block, error) {
	file, diags := hclparse.NewParser().ParseHCLFile(filePath)
	if diags.HasErrors() {
		return nil, diags
	}

	schema := &hcl.BodySchema{
		Blocks: []hcl.BlockHeaderSchema{
			{Type: "resource", LabelNames: []string{"type", "name"}},
		},
	}
	// PartialContent: ignore provider/terraform/variable blocks etc.
	content, _, diags := file.Body.PartialContent(schema)
	if diags.HasErrors() {
		return nil, diags
	}

	for _, block := range content.Blocks {
		if block.Labels[0] == resourceType && block.Labels[1] == blockName {
			return block, nil
		}
	}
	return nil, fmt.Errorf("resource %s.%s not found in %s", resourceType, blockName, filePath)
}

// bucketReference reads `bucket = aws_s3_bucket.X.id` and returns "X"
// (the Terraform label of the bucket this block points at).
func bucketReference(block *hcl.Block) (string, error) {
	content, _, diags := block.Body.PartialContent(&hcl.BodySchema{
		Attributes: []hcl.AttributeSchema{{Name: "bucket", Required: true}},
	})
	if diags.HasErrors() {
		return "", diags
	}

	traversal, diags := hcl.AbsTraversalForExpr(content.Attributes["bucket"].Expr)
	if diags.HasErrors() {
		return "", diags
	}
	if len(traversal) < 2 {
		return "", fmt.Errorf("unexpected bucket reference in %s.%s", block.Labels[0], block.Labels[1])
	}
	attr, ok := traversal[1].(hcl.TraverseAttr)
	if !ok {
		return "", fmt.Errorf("unexpected bucket reference shape in %s.%s", block.Labels[0], block.Labels[1])
	}
	return attr.Name, nil
}

// literalBucketName reads `bucket = "my-tf-demo-bucket"` from an aws_s3_bucket block.
func literalBucketName(block *hcl.Block) (string, error) {
	content, _, diags := block.Body.PartialContent(&hcl.BodySchema{
		Attributes: []hcl.AttributeSchema{{Name: "bucket", Required: true}},
	})
	if diags.HasErrors() {
		return "", diags
	}

	val, diags := content.Attributes["bucket"].Expr.Value(nil)
	if diags.HasErrors() {
		return "", fmt.Errorf("bucket name for aws_s3_bucket.%s must be a literal string: %w", block.Labels[1], diags)
	}
	if val.Type() != cty.String {
		return "", fmt.Errorf("bucket name for aws_s3_bucket.%s must be a string", block.Labels[1])
	}
	return val.AsString(), nil
}

// versioningStatus reads versioning_configuration { status = "Enabled" }.
func versioningStatus(block *hcl.Block) (string, error) {
	content, _, diags := block.Body.PartialContent(&hcl.BodySchema{
		Blocks: []hcl.BlockHeaderSchema{{Type: "versioning_configuration"}},
	})
	if diags.HasErrors() {
		return "", diags
	}
	if len(content.Blocks) == 0 {
		return "", fmt.Errorf("no versioning_configuration block in aws_s3_bucket_versioning.%s", block.Labels[1])
	}

	inner, _, diags := content.Blocks[0].Body.PartialContent(&hcl.BodySchema{
		Attributes: []hcl.AttributeSchema{{Name: "status", Required: true}},
	})
	if diags.HasErrors() {
		return "", diags
	}

	val, diags := inner.Attributes["status"].Expr.Value(nil)
	if diags.HasErrors() {
		return "", diags
	}
	if val.Type() != cty.String {
		return "", fmt.Errorf("versioning status must be a string")
	}
	return val.AsString(), nil
}

func ParseBlueprint(dir string) ([]BucketConfig, error) {
	module, diags := tfconfig.LoadModule(dir)
	if diags.HasErrors() {
		return nil, fmt.Errorf("failed to parse terraform: %s", diags.Error())
	}

	// keyed by TERRAFORM label (how other resources reference a bucket)
	bucketMap := map[string]*BucketConfig{}

	// pass 1: buckets
	for _, resource := range module.ManagedResources {
		if resource.Type != "aws_s3_bucket" {
			continue
		}
		block, err := findResourceBlock(resource.Pos.Filename, resource.Type, resource.Name)
		if err != nil {
			return nil, err
		}
		realName, err := literalBucketName(block)
		if err != nil {
			return nil, err
		}
		bucketMap[resource.Name] = &BucketConfig{BucketName: realName}
	}

	// pass 2: encryption + versioning configs, linked via real references
	for _, resource := range module.ManagedResources {
		if resource.Type != "aws_s3_bucket_server_side_encryption_configuration" &&
			resource.Type != "aws_s3_bucket_versioning" {
			continue
		}

		block, err := findResourceBlock(resource.Pos.Filename, resource.Type, resource.Name)
		if err != nil {
			return nil, err
		}
		label, err := bucketReference(block)
		if err != nil {
			return nil, fmt.Errorf("resolving bucket reference for %s.%s: %w", resource.Type, resource.Name, err)
		}
		entry, exists := bucketMap[label]
		if !exists {
			continue // points at a bucket not defined in this module
		}

		switch resource.Type {
		case "aws_s3_bucket_server_side_encryption_configuration":
			entry.IsEncrypted = true
		case "aws_s3_bucket_versioning":
			status, err := versioningStatus(block)
			if err != nil {
				return nil, err
			}
			entry.IsVersioned = status == "Enabled"
		}
	}

	var buckets []BucketConfig
	for _, entry := range bucketMap {
		buckets = append(buckets, *entry)
	}
	// stable order so the API output doesn't shuffle between requests
	sort.Slice(buckets, func(i, j int) bool { return buckets[i].BucketName < buckets[j].BucketName })

	return buckets, nil
}