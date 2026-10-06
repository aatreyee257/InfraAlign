package api

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

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

// requireAPIKey protects routes that change real AWS state. The expected key
// comes from INFRAALIGN_API_KEY; if it is unset the route refuses to run
// rather than silently allowing unauthenticated remediation.
func requireAPIKey(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		expected := os.Getenv("INFRAALIGN_API_KEY")
		if expected == "" {
			http.Error(w, "server misconfigured: INFRAALIGN_API_KEY is not set", http.StatusInternalServerError)
			return
		}
		provided := r.Header.Get("X-API-Key")
		// Constant-time comparison so the key can't be guessed via response timing.
		if subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) != 1 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

// isAllowedOrigin limits CORS to the local dashboard: pages served from
// localhost/127.0.0.1, or opened directly as a file (browsers send Origin "null").
func isAllowedOrigin(origin string) bool {
	if origin == "null" {
		return true
	}
	for _, prefix := range []string{"http://localhost", "http://127.0.0.1"} {
		if origin == prefix || strings.HasPrefix(origin, prefix+":") {
			return true
		}
	}
	return false
}

// withCORS lets the browser-based dashboard call the API from a different
// origin, and answers preflight requests (the remediate call sends a custom
// X-API-Key header, so browsers send an OPTIONS preflight first).
func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && isAllowedOrigin(origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-API-Key")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// Start runs the HTTP server on the given address (e.g. "127.0.0.1:8080").
func Start(addr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/drift", handleDrift)
	mux.HandleFunc("POST /api/remediate/{bucket}", requireAPIKey(handleRemediate))

	log.Printf("InfraAlign API listening on %s", addr)
	return http.ListenAndServe(addr, withCORS(mux))
}
