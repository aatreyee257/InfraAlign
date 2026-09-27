# InfraAlign

A Go CLI that detects drift between Terraform-defined infrastructure and the real state of your AWS account, with Slack alerting and optional auto-remediation.

InfraAlign parses your Terraform HCL to build a "blueprint" of what your infrastructure *should* look like, queries AWS directly to see what it *actually* looks like, diffs the two, and reports (or fixes) the difference.

> **Status:** core detection and remediation logic is implemented and working. This is an active learning/portfolio project — see [Roadmap](#roadmap) for what's still in progress.

## How it works

```
Terraform files (.tf)          AWS Account
        │                           │
        ▼                           ▼
  parser.ParseBlueprint      aws.ScanBuckets
  (HCL parsing + reference    (AWS SDK v2)
   resolution)
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
         ┌───────────┴───────────┐
         ▼                       ▼
  notifier.SendAlert      (optional) Remediate
     (Slack webhook)       (via Remediator
                            interface, per-check)
```

### Why the HCL parsing isn't just name-matching

Terraform links resources by *reference*, not by shared naming. For example:

```hcl
resource "aws_s3_bucket" "mybucket" {
  bucket = "my-tf-demo-bucket"
}

resource "aws_s3_bucket_server_side_encryption_configuration" "mybucket" {
  bucket = aws_s3_bucket.mybucket.id
  ...
}
```

InfraAlign parses the `bucket = aws_s3_bucket.mybucket.id` expression itself (via `hcl.AbsTraversalForExpr`) to resolve which bucket an encryption config actually belongs to — it does not assume the two resources share the same Terraform resource name, since that's not guaranteed in real-world configurations.

### Why checks are pluggable

Drift detection isn't hardcoded to one attribute. A `Check` is anything that can answer "what's the desired value?" and "what's the actual value?" for a given resource:

```go
type Check interface {
    Name() string
    Desired(blueprint parser.BucketConfig) string
    Actual(reality myaws.BucketConfig) string
}
```

`DetectDrift` runs every registered `Check` against every resource — adding a new kind of check (versioning, public access, tagging) means writing one new type, not modifying the detection engine.

Remediation is opt-in per check, via a separate interface:

```go
type Remediator interface {
    Remediate(bucketName string) error
}
```

A check can implement `Check` only (detect but never auto-fix) or both `Check` and `Remediator` (detect and offer a fix). `main.go` uses a type assertion (`check.(Remediator)`) to find out which, at runtime, rather than hardcoding which checks are fixable.

## Currently implemented

- **Terraform parsing** (`backend/internal/parser`) — loads a Terraform module, finds `aws_s3_bucket` and `aws_s3_bucket_server_side_encryption_configuration` resources, and resolves the real reference between them via HCL expression traversal.
- **AWS scanning** (`backend/internal/aws`) — lists S3 buckets and checks their encryption state via the AWS SDK v2, correctly distinguishing "encryption genuinely not configured" from API/permission failures (via `smithy.APIError`).
- **Drift detection** (`backend/internal/drift`) — compares blueprint vs. reality per-`Check`, classifying each result as `Compliant`, `Drifted`, or `Missing`.
- **Slack alerting** (`backend/internal/notifier`) — posts drift and remediation results to a Slack incoming webhook.
- **Auto-remediation** — when enabled, attempts to fix drifted attributes via each check's `Remediator` implementation (currently: enabling S3 server-side encryption).
- **One concrete check** — `EncryptionCheck`, covering S3 bucket server-side encryption.

## Getting started

### Prerequisites

- Go 1.22+
- AWS credentials configured (`aws configure`, environment variables, or an active SSO session) with at least `s3:ListBuckets` and `s3:GetEncryptionConfiguration` permissions (add `s3:PutEncryptionConfiguration` if using auto-remediation)
- (Optional) A Slack incoming webhook URL, if you want alerts

### Setup

```bash
git clone <your-repo-url>
cd InfraAlign
go build ./...
```

### Configuration

| Environment variable | Purpose | Default |
|---|---|---|
| `SLACK_WEBHOOK_URL` | Slack incoming webhook for alerts | none — alerting fails loudly if unset |
| `AUTO_REMEDIATE` | Set to `true` to auto-fix drifted resources | `false` |

### Running

From the repository root (the CLI currently reads Terraform files from a relative `terraform-samples` path):

```bash
go run ./backend/cmd/server
```

With auto-remediation enabled:

```bash
AUTO_REMEDIATE=true SLACK_WEBHOOK_URL="https://hooks.slack.com/services/..." go run ./backend/cmd/server
```

### Example output

```
--- Desired State Blueprint ---
Resource found: aws_s3_bucket.mybucket
Expected Encryption: true
--- Actual State (AWS) ---
Bucket: my-tf-demo-bucket | Encrypted: false
--- Drift Report ---
Bucket: mybucket | Status: drifted | Attribute: ServerSideEncryption | Expected: true | Actual: false
```

## Project structure

```
backend/
├── cmd/server/main.go        # entry point — wires parsing, scanning, detection, alerting, remediation
├── internal/
│   ├── parser/hcl.go         # Terraform HCL parsing + reference resolution
│   ├── aws/s3.go             # AWS SDK v2 S3 scanning and remediation actions
│   ├── drift/detector.go     # Check/Remediator interfaces, DetectDrift engine
│   └── notifier/slack.go     # Slack webhook alerting
terraform-samples/
└── main.tf                   # sample Terraform config used as the blueprint source
```

## Roadmap

- [ ] Additional `Check` implementations (S3 versioning, public access block, tagging)
- [ ] Distinguish remediation behavior for `Missing` resources (can't "fix" the encryption of a bucket that doesn't exist — this needs its own handling, likely a `terraform apply`-style creation path rather than `Remediate`)
- [ ] HTTP API layer (`GET /api/drift`, `POST /api/remediate/{bucket}`) so results and actions aren't limited to stdout — required before a frontend can exist
- [ ] Web frontend, once the API layer above exists
- [ ] CLI polish — subcommands and flags (e.g. via `cobra`) instead of a single env-var-driven run
- [ ] Automated tests for `DetectDrift` and HCL reference resolution
- [ ] Support for additional AWS resource types beyond S3

## License

Not yet decided.
