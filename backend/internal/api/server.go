package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	myaws "infraalign/backend/internal/aws"
	"infraalign/backend/internal/drift"
	"infraalign/backend/internal/parser"
)

// runDetection re-runs the full parse → scan → detect pipeline fresh,
// on every request. Fine for a project this size; a future version
// might cache/schedule this instead of doing it per-request.
func runDetection() ([]drift.Difference, error) {
	blueprint, err := parser.ParseBlueprint("terraform-samples")
	if err != nil {
		return nil, fmt.Errorf("parsing blueprint: %w", err)
	}

	reality, err := myaws.ScanBuckets()
	if err != nil {
		return nil, fmt.Errorf("scanning aws: %w", err)
	}

	return drift.DetectDrift(blueprint, reality, drift.DefaultChecks()), nil
}

func handleDrift(w http.ResponseWriter, r *http.Request) {
	differences, err := runDetection()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(differences)
}

func handleRemediate(w http.ResponseWriter, r *http.Request) {
	bucketName := r.PathValue("bucket")

	differences, err := runDetection()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var results []drift.Difference
	for _, d := range differences {
		if d.BucketName != bucketName || d.Status != drift.Drifted {
			continue // only remediate real drift on the requested bucket
		}

		remediator, ok := d.Check.(drift.Remediator)
		if !ok {
			continue
		}

		if err := remediator.Remediate(d.BucketName); err != nil {
			http.Error(w, fmt.Sprintf("remediation failed: %v", err), http.StatusInternalServerError)
			return
		}

		d.Status = drift.Compliant
		d.ActualVal = d.ExpectedVal
		results = append(results, d)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

// Start runs the HTTP server on the given address (e.g. ":8080").
func Start(addr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/drift", handleDrift)
	mux.HandleFunc("POST /api/remediate/{bucket}", handleRemediate)

	log.Printf("InfraAlign API listening on %s", addr)
	return http.ListenAndServe(addr, mux)
}