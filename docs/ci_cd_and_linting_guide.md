# 🚀 Go CI/CD, Linting & Pre-Commit Guide

This guide details the complete developer tooling, linting rules, pre-commit configuration, and automated CI/CD deployment pipelines for the **Go ADK Data Reconciliation Agent**.

---

## 1. Local Tooling & Linting

### Linting Configuration: `.golangci.yml`
The repository uses [`golangci-lint`](https://golangci-lint.run/) with aggressive checks enabled:
* **Core Linters**: `govet` (shadowing), `staticcheck`, `errcheck`, `ineffassign`, `typecheck`, `unused`.
* **Code Style & Formatting**: `gofmt -s` (simplification), `goimports` (local prefix clustering: `github.com/tiaaburton/Data-Recon-Agent`), `revive` (idiomatic naming, exported docs).
* **Security & Vulnerability**: `gosec` (AST-based security flaw detection).
* **Spelling**: `misspell` (US locale).

### Running Local Lint & Format Commands
```bash
# Format Go code, sort imports, and format Terraform HCL
make fmt

# Run Go vetting and golangci-lint
make lint

# Run full local CI matrix (Format -> Lint -> Test -> Eval -> Build)
make ci
```

---

## 2. Git Pre-Commit Hooks: `.pre-commit-config.yaml`

Pre-commit hooks validate code quality, hygiene, and security before commits are accepted.

### Hook Breakdown:
1. **Repository Hygiene**:
   * `trailing-whitespace`: Strips trailing whitespace (skips `.json` data fixtures).
   * `end-of-file-fixer`: Ensures clean newline termination.
   * `check-yaml` & `check-json`: Validates schema syntax.
   * `check-added-large-files`: Prevents accidental binary blobs (`maxkb=2048`).
   * `detect-private-key`: Blocks accidental secret leaks.
2. **Go Automation**:
   * `go-fmt`, `go-imports`, `go-vet`, `go-mod-tidy`, and `go-unit-tests`.
3. **Terraform Validation**:
   * `terraform_fmt`, `terraform_validate`.

### Installation & Usage
```bash
# Install git hooks into .git/hooks/pre-commit
make setup-hooks

# Run pre-commit checks manually across all files
make pre-commit
```

---

## 3. GitHub Actions Workflows (`.github/workflows/`)

### 1. Continuous Integration (`.github/workflows/ci.yml`)
Triggers on every `push` and `pull_request` to `main`:
* **Stage 1 (Lint & Format)**: Verifies `gofmt`, runs `go vet`, and executes `golangci-lint-action`.
* **Stage 2 (Test & Benchmark)**: Runs unit tests, executes `TestGoldenEvaluation_500` (500 records), executes `TestMultiturnReasoningEngineEvaluation`, and generates code coverage artifact.
* **Stage 3 (Build)**: Cross-compiles standalone CLI binaries (`synth`, `loader`, `verifier`, `agent`).
* **Stage 4 (Terraform Check)**: Validates `terraform fmt -check` and `terraform validate`.

### 2. Continuous Delivery (`.github/workflows/deploy.yml`)
Triggers automatically on merge to `main`:
* Authenticates to Google Cloud via **Workload Identity Federation (WIF)**.
* Executes `scripts/deploy_agent_engine.sh` to update the Vertex AI Reasoning Engine.
* Executes `scripts/setup_agent_gateway.sh` to reconcile Agent Gateway and Agent Registry state.
* Executes `scripts/register_gemini_enterprise.sh` to update Gemini Enterprise Agent Platform settings.

---

## 4. Google Cloud Build (`cloudbuild.yaml`)

For environments using Google Cloud native build pipelines:
```bash
gcloud builds submit --config=cloudbuild.yaml .
```
Runs linting, unit testing, binary compilation, and Agent Engine updates within Cloud Build containers.
