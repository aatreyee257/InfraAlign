# InfraAlign

A Go CLI and HTTP API that detects drift between Terraform-defined infrastructure and the real state of your AWS account, with Slack alerting, auto-remediation, and a local web dashboard.

InfraAlign parses your Terraform HCL to build a "blueprint" of what your infrastructure *should* look like, queries AWS directly to see what it *actually* looks like, diffs the two per-attribute, and reports (or fixes) the difference — from the terminal, over HTTP, or from the dashboard.
<img width="1071" height="466" alt="Screenshot 2026-10-06 at 18 47 22" src="https://github.com/user-attachments/assets/6e544c41-afbc-4cee-a7cc-6672310379ab" />


## How it works

```
Terraform files (.tf)          AWS Account
        │                           │
        ▼                           ▼
  parser.ParseBlueprint      aws.ScanBuckets
  (HCL parsing + reference    (AWS SDK v2)
   resolution, real bucket
   names, not Terraform
   labels)
        │                           │
        └───────────┬───────────────┘
                     ▼
            drift.DetectDrift
        (compares blueprint vs reality
         per-attribute, via pluggable
              Check implementations)
                     │
                     ▼
              Drift Report
                     │
      ┌──────────────┼────────────────────┐
      ▼              ▼                    ▼
 CLI (stdout)   notifier.SendAlert   api.Start (HTTP)
                 (Slack webhook)       │         │
                                  GET /api/drift  │
                            (optional) Remediate ─┘
                           POST /api/remediate/{bucket}
                         (requires X-API-Key, per-check
                          Remediator, CORS-enabled)
                                       │
                                       ▼
                              frontend/ dashboard
                         (plain HTML/CSS/JS, fetches
                          the API from your browser)
```

### Why the HCL parsing isn't just name-matching

Terraform links resources by *reference*, not by shared naming, and the real AWS resource name is rarely the same as the Terraform label. For example:

```hcl
resource "aws_s3_bucket" "mybucket" {
  bucket = "aatreyee-tf-bucket"   # <- the REAL AWS name
}

resource "aws_s3_bucket_server_side_encryption_configuration" "mybucket" {
  bucket = aws_s3_bucket.mybucket.id   # <- the real LINK
  ...
}
```

InfraAlign resolves both of these properly:
- The real bucket name is read from the literal `bucket = "..."` attribute on the `aws_s3_bucket` block (via `hcl.Expr.Value`), not assumed to match the Terraform resource label.
- The link between an `aws_s3_bucket_server_side_encryption_configuration` / `aws_s3_bucket_versioning` block and its bucket is resolved by parsing the `bucket = aws_s3_bucket.mybucket.id` expression itself (via `hcl.AbsTraversalForExpr`), not by assuming the two resources share a Terraform label.

### Why checks are pluggable

Drift detection isn't hardcoded to one attribute. A `Check` is anything that can answer "what's the desired value?" and "what's the actual value?" for a given resource:

```go
type Check interface {
    Name() string
    Desired(blueprint parser.BucketConfig) string
    Actual(reality myaws.BucketConfig) string
}
```

`DetectDrift` runs every check in `drift.DefaultChecks()` against every resource — adding a new kind of check (public access block, tagging, …) means writing one new type and adding it to that list, not modifying the detection engine. Currently registered: `EncryptionCheck` and `VersioningCheck`.

Remediation is opt-in per check, via a separate interface:

```go
type Remediator interface {
    Remediate(bucketName string) error
}
```

A check can implement `Check` only (detect but never auto-fix) or both `Check` and `Remediator` (detect and offer a fix). The CLI and the API both use a type assertion (`check.(Remediator)`) to find out which, at runtime, rather than hardcoding which checks are fixable.

## Currently implemented

- **Terraform parsing** (`backend/internal/parser`) — loads a Terraform module, resolves real AWS bucket names from literal attributes, and resolves encryption/versioning resource references via HCL expression traversal rather than name matching.
- **AWS scanning** (`backend/internal/aws`) — lists S3 buckets and checks encryption and versioning state via the AWS SDK v2, correctly distinguishing "genuinely not configured" from API/permission failures (via `smithy.APIError`).
- **Drift detection** (`backend/internal/drift`) — compares blueprint vs. reality per-`Check`, classifying each result as `Compliant`, `Drifted`, or `Missing`. Verified against real AWS state: suspending versioning on a live bucket is correctly detected as `Drifted`, and remediation correctly re-enables it.
- **Slack alerting** (`backend/internal/notifier`) — posts drift and remediation results to a Slack incoming webhook.
- **Auto-remediation** — via each check's `Remediator` implementation (currently: enabling S3 encryption, enabling S3 versioning).
- **HTTP API** (`backend/internal/api`) — `GET /api/drift` and `POST /api/remediate/{bucket}`, so results and actions aren't limited to stdout. The remediate route requires an `X-API-Key` header (checked against the `INFRAALIGN_API_KEY` environment variable) since it can change real AWS state; the drift route is read-only and unauthenticated. CORS is enabled so a browser-based frontend on a different origin can call it.
- **Local web dashboard** (`frontend/`) — a plain HTML/CSS/JS page (no build step, no framework) that calls the API directly from the browser. Shows drift grouped by bucket, lets you filter by status or search by bucket name, collapse/expand buckets, auto-refresh on an interval, and trigger remediation per bucket with toast notifications on the result.

## Getting started

### Prerequisites

- Go 1.22+ (see note on `go.mod`'s version below)
- AWS credentials configured (`aws configure`, environment variables, or an active SSO session) with at least `s3:ListBuckets`, `s3:GetEncryptionConfiguration`, `s3:GetBucketVersioning` (add `s3:PutEncryptionConfiguration` / `s3:PutBucketVersioning` if using auto-remediation)
- (Optional) A Slack incoming webhook URL, if you want alerts
- A modern browser, if you want the dashboard

### Setup

```bash
git clone <your-repo-url>
cd InfraAlign
go build ./...
```

> Note: `go.mod` currently pins a newer Go toolchain version than some environments have installed by default. If `go build` tries to download a toolchain and fails, either install the matching Go version or temporarily edit the `go` directive in `go.mod` to match your installed version (e.g. `go 1.22`) — this does not affect how the code runs.

### Configuration

| Environment variable | Purpose | Default |
|---|---|---|
| `SLACK_WEBHOOK_URL` | Slack incoming webhook for alerts | none — alerting fails loudly if unset |
| `AUTO_REMEDIATE` | Set to `true` to auto-fix drifted resources (CLI mode only) | `false` |
| `INFRAALIGN_API_KEY` | Required to authorize `POST /api/remediate/{bucket}` (API/dashboard mode) | none — remediate route returns 500 if unset |

### Running as a CLI (one-shot report)

From the repository root (the CLI currently reads Terraform files from a relative `terraform-samples` path):

```bash
go run ./backend/cmd/server
```

With auto-remediation enabled:

```bash
AUTO_REMEDIATE=true SLACK_WEBHOOK_URL="https://hooks.slack.com/services/..." go run ./backend/cmd/server
```

Example output:
```
--- Desired State Blueprint ---
Resource found: aws_s3_bucket.aatreyee-tf-bucket
Expected Encryption: true
aatreyee-tf-bucket: encrypted=true versioned=true
--- Actual State (AWS) ---
Bucket: aatreyee-tf-bucket | Encrypted: true
--- Drift Report ---
Bucket: aatreyee-tf-bucket | Status: compliant | Attribute: ServerSideEncryption | Expected: true | Actual: true
Bucket: aatreyee-tf-bucket | Status: drifted | Attribute: Versioning | Expected: true | Actual: false
```

### Running as an API + dashboard

Start the server:
```bash
export INFRAALIGN_API_KEY="pick-a-long-random-string"
go run ./backend/cmd/server serve
```
This binds to `localhost:8080` only (not your whole network).

Query it directly if you want:
```bash
curl http://localhost:8080/api/drift
curl -X POST http://localhost:8080/api/remediate/aatreyee-tf-bucket -H "X-API-Key: pick-a-long-random-string"
```

Open the dashboard — it's a static file, no server needed to host it:
```bash
open frontend/index.html   # or just double-click it
```
On first load, click the ⚙ settings icon and set the API key to match `INFRAALIGN_API_KEY` (the API base URL defaults to `http://localhost:8080`, which matches the default above). The dashboard will then show live drift and let you remediate from the Remediate button on any drifted bucket.

## Project structure

```
backend/
├── cmd/server/main.go        # entry point — CLI mode by default, `serve` arg for API mode
├── internal/
│   ├── parser/hcl.go         # Terraform HCL parsing + reference resolution
│   ├── aws/s3.go             # AWS SDK v2 S3 scanning and remediation actions
│   ├── drift/detector.go     # Check/Remediator interfaces, DefaultChecks, DetectDrift engine
│   ├── notifier/slack.go     # Slack webhook alerting
│   └── api/server.go         # HTTP API: routes, auth, CORS
frontend/
├── index.html                 # page structure
├── styles.css                 # all styling (dark-first console theme)
└── app.js                     # all logic — fetches the API, renders, handles remediation
terraform-samples/
└── main.tf                    # sample Terraform config used as the blueprint source
```

