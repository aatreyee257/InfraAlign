package main

import (
	"fmt"
	"log"
	
	myaws "infraalign/backend/internal/aws"
	"infraalign/backend/internal/parser"
)

func main(){
	blueprint, err := parser.ParseBlueprint("terraform-samples")
	if err != nil {
		log.Fatalf("Failed to parse blueprint: %v",err)
	}
	fmt.Println("--- Desired State Blueprint ---")
	for _, b := range blueprint {
		fmt.Printf("Resource found: aws_s3_bucket.%s\n",b.BucketName)
		fmt.Printf("Expected Encryption: %v\n",b.IsEncrypted)
	}
	
	reality, err := myaws.ScanBuckets()
    if err != nil {
        log.Fatalf("Failed to scan AWS: %v", err)
    }

    fmt.Println("--- Actual State (AWS) ---")
    for _, b := range reality {
        fmt.Printf("Bucket: %s | Encrypted: %v\n", b.BucketName, b.IsEncrypted)
    }
}
