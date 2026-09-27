package main

import (
	"fmt"
	"log"
	"os"

	myaws "infraalign/backend/internal/aws"
	"infraalign/backend/internal/drift"
	"infraalign/backend/internal/notifier"
	"infraalign/backend/internal/parser"
)

func main() {
	blueprint, err := parser.ParseBlueprint("terraform-samples")
	if err != nil {
		log.Fatalf("Failed to parse blueprint: %v", err)
	}
	fmt.Println("--- Desired State Blueprint ---")
	for _, b := range blueprint {
		fmt.Printf("Resource found: aws_s3_bucket.%s\n", b.BucketName)
		fmt.Printf("Expected Encryption: %v\n", b.IsEncrypted)
	}

	reality, err := myaws.ScanBuckets()
	if err != nil {
		log.Fatalf("Failed to scan AWS: %v", err)
	}

	fmt.Println("--- Actual State (AWS) ---")
	for _, b := range reality {
		fmt.Printf("Bucket: %s | Encrypted: %v\n", b.BucketName, b.IsEncrypted)
	}

	differences := drift.DetectDrift(blueprint, reality, []drift.Check{drift.EncryptionCheck{}})

	fmt.Println("--- Drift Report ---")
	for _, d := range differences {
		fmt.Printf("Bucket: %s | Status: %s | Attribute: %s | Expected: %s | Actual: %s\n",
			d.BucketName, d.Status, d.AttributeName, d.ExpectedVal, d.ActualVal)
	}

	autoRemediate := os.Getenv("AUTO_REMEDIATE") == "true"

	for _, d := range differences {
		if d.Status == drift.Drifted || d.Status == drift.Missing {
			err := notifier.SendAlert(d)
			if err != nil {
				log.Printf("Failed to send alert: %v", err)
			}

			if autoRemediate {
				if remediator, ok := d.Check.(drift.Remediator); ok {
					err := remediator.Remediate(d.BucketName)
					if err != nil {
						log.Printf("Failed to remediate bucket %s: %v", d.BucketName, err)
					} else {
						fmt.Printf("Successfully remediated bucket %s\n", d.BucketName)

						resolved := drift.Difference{
							BucketName:    d.BucketName,
							Status:        drift.Compliant,
							AttributeName: d.AttributeName,
							ExpectedVal:   d.ExpectedVal,
							ActualVal:     d.ExpectedVal,
							Check:         d.Check,
						}
						notifier.SendAlert(resolved)
					}
				} else {
					log.Printf("No remediation available for %s on bucket %s — alerting only", d.AttributeName, d.BucketName)
				}
			}
		}
	}
}