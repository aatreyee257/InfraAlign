package main

import (
	"log"

	myaws "infraalign/backend/internal/aws"
)

func main(){
	if err := myaws.ScanBuckets();err != nil {
		log.Fatal(err)
	}
}
