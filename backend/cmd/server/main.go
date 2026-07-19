package main

import (
	"fmt"
	"log"

	"infraalign/backend/internal/parser"
)

func main(){
	config,err := parser.ParseBlueprint("terraform-samples")
	if err != nil {
		log.Fatalf("Failed to parse blueprint: %v",err)
	}
	fmt.Println("--- Desired State Blueprint ---")
	fmt.Printf("Resource found: aws_s3_bucket.%s\n",config.BucketName)
	fmt.Printf("Expected Encryption: %v\n",config.IsEncrypted)
}
