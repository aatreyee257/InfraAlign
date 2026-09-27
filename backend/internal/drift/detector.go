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
    Check Check
	
}

type Check interface {
    Name() string
    Desired(blueprint parser.BucketConfig) string
    Actual(reality myaws.BucketConfig) string
}

type Remediator interface {
    Remediate(bucketName string) error
}

type EncryptionCheck struct{}

func (c EncryptionCheck) Name() string { return "ServerSideEncryption" }
func (c EncryptionCheck) Desired(b parser.BucketConfig) string {  return fmt.Sprintf("%v", b.IsEncrypted) }
func (c EncryptionCheck) Actual(r myaws.BucketConfig) string { return fmt.Sprintf("%v", r.IsEncrypted) }
func (c EncryptionCheck) Remediate(bucketName string) error {return myaws.EnableServerEncryption(bucketName)}
type Status string

const (
	Compliant Status = "compliant"
	Drifted Status = "drifted"
	Missing Status = "missing_in_cloud"
)

func DetectDrift(blueprints []parser.BucketConfig, reality []myaws.BucketConfig, checks []Check) []Difference {
	//convert reality slice to map for easier lookup
	realityMap := map[string]myaws.BucketConfig{}
	for _, b := range reality {
		realityMap[b.BucketName] = b
	}

	//loop through blueprints and compare with reality
	var differences []Difference
	for _,blueprint := range blueprints {
		realBucket, exists := realityMap[blueprint.BucketName]
		
		if !exists {
            // Scenario A: bucket doesn't exist in AWS. Still loop over every check, since each attribute is independently "missing."
            for _, check := range checks {
                differences = append(differences, Difference{
                    BucketName:    blueprint.BucketName,
                    Status:        Missing,
                    AttributeName: check.Name(),
                    ExpectedVal:   check.Desired(blueprint),
                    ActualVal:     "not found in AWS",
                    Check: check,
                })
            }
            continue
        }  
        
        for _, check := range checks {
    if check.Desired(blueprint) != check.Actual(realBucket) {
        differences = append(differences, Difference{
            BucketName:    blueprint.BucketName,
            Status:        Drifted,
            AttributeName: check.Name(),
            ExpectedVal:   check.Desired(blueprint),
            ActualVal:     check.Actual(realBucket),
            Check: check,
        })
    } else {
        differences = append(differences, Difference{
            BucketName:    blueprint.BucketName,
            Status:        Compliant,
            AttributeName: check.Name(),
            ExpectedVal:   check.Desired(blueprint),
            ActualVal:     check.Actual(realBucket),
            Check: check,
        })
    }
}
    }
	return differences

}
               