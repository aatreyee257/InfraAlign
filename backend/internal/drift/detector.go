package drift

import (
	"fmt"
	myaws "infraalign/backend/internal/aws"
    "infraalign/backend/internal/parser"

)

type Difference struct {
	BucketName string
	Status Status
	AttributeName string // holds "ServerSideEncryption", "Versioning", "BucketPolicy" etc.
	ExpectedVal string
	ActualVal string
	
}

type Status string

const (
	Compliant Status = "compliant"
	Drifted Status = "drifted"
	Missing Status = "missing_in_cloud"
)

func DetectDrift(blueprints []parser.BucketConfig, reality []aws.BucketConfig) []Difference {
	//convert reality slice to map for easier lookup
	realityMap := map[string]aws.BucketConfig{}
	for _, b := range reality {
		realityMap[b.BucketName] = b
	}

	//loop through blueprints and compare with reality
	var differences []Difference
	for _,blueprint := range blueprints {
		realBucket := realityMap[blueprint.BucketName]
	}

	if !exists {
            // Scenario A: bucket in terraform but missing in AWS
            differences = append(differences, Difference{
                BucketName:    blueprint.BucketName,
                Status:        Missing,
                AttributeName: "ServerSideEncryption",
                ExpectedVal:   fmt.Sprintf("%v", blueprint.IsEncrypted),
                ActualVal:     "not found in AWS",
            })
        } else if blueprint.IsEncrypted != realBucket.IsEncrypted {
            // Scenario B: bucket exists but encryption doesn't match
            differences = append(differences, Difference{
                BucketName:    blueprint.BucketName,
                Status:        Drifted,
                AttributeName: "ServerSideEncryption",
                ExpectedVal:   fmt.Sprintf("%v", blueprint.IsEncrypted),
                ActualVal:     fmt.Sprintf("%v", realBucket.IsEncrypted),
            })
        } else {
            // Scenario C: everything matches
            differences = append(differences, Difference{
                BucketName:    blueprint.BucketName,
                Status:        Compliant,
                AttributeName: "ServerSideEncryption",
                ExpectedVal:   fmt.Sprintf("%v", blueprint.IsEncrypted),
                ActualVal:     fmt.Sprintf("%v", realBucket.IsEncrypted),
            })
		}
	return differences

}