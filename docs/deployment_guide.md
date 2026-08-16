# Enterprise Data Reconciliation Agent: Complete Redeployment Runbook

- **Target Audience**: Cloud Architects, SREs, AI Systems Engineers
- **Core Stacks**: Go 1.22+, Google Cloud Platform, Vertex AI Agent Engine, Cloud Run, Terraform
- **Estimated Deployment Time**: 25 minutes

---

## 1. Prerequisites & Environment Setup

### 1.1. Required CLI Tools
Ensure the following tools are installed on the deployment workstation:
- **Go 1.22+**: `go version`
- **Terraform 1.7+**: `terraform version`
- **Google Cloud SDK (`gcloud`)**: `gcloud version`
- **Vertex AI Agent CLI (`agentapi`)**: `agentapi --help`
- **Docker / Cloud Build**: For containerizing the Go BYO-MCP service.

### 1.2. GCP Project Variables
Export standard environment variables:

```bash
export GCP_PROJECT_ID="your-enterprise-project-id"
export GCP_REGION="us-central1"
export ENVIRONMENT="production" # or staging / sandbox
export KMS_KEYRING_NAME="recon-keyring-us-central1"
export KMS_KEY_NAME="recon-cmek-key"
export FIRESTORE_DATABASE_ID="(default)"
```

Authenticate to GCP:
```bash
gcloud auth login
gcloud auth application-default login
gcloud config set project ${GCP_PROJECT_ID}
```

---

## 2. Infrastructure as Code (Terraform) Deployment

### 2.1. Enable Required GCP APIs
```bash
gcloud services enable \
    aiplatform.googleapis.com \
    run.googleapis.com \
    firestore.googleapis.com \
    pubsub.googleapis.com \
    cloudkms.googleapis.com \
    secretmanager.googleapis.com \
    dlp.googleapis.com \
    cloudtrace.googleapis.com \
    logging.googleapis.com \
    cloudbuild.googleapis.com \
    artifactregistry.googleapis.com
```

### 2.2. Initialize and Apply Terraform
Navigate to the `terraform/` directory:

```bash
cd terraform
cp terraform.tfvars.example terraform.tfvars
```

Configure `terraform.tfvars`:
```hcl
project_id         = "your-enterprise-project-id"
region             = "us-central1"
environment        = "prod"
service_name       = "data-recon-agent"
firestore_location = "nam5"
pubsub_topic_name  = "recon-discrepancy-events"
dlp_inspection_template = "projects/your-enterprise-project-id/inspectTemplates/recon-pii-scrubber"
```

Execute Terraform plan and apply:
```bash
terraform init
terraform plan -out=tfplan.binary
terraform apply tfplan.binary
```

---

## 3. Secret Manager & Connector Configurations

### 3.1. Provision Secrets in GCP Secret Manager
Inject connector credentials securely into Secret Manager:

```bash
# 1. ServiceNow OAuth Credentials
gcloud secrets create sn-oauth-secret --replication-policy="automatic"
echo -n '{"instance_url":"https://instance.service-now.com","client_id":"...","client_secret":"..."}' | \
    gcloud secrets versions add sn-oauth-secret --data-file=-

# 2. Salesforce Connected App Credentials
gcloud secrets create sf-oauth-secret --replication-policy="automatic"
echo -n '{"instance_url":"https://login.salesforce.com","client_id":"...","client_secret":"..."}' | \
    gcloud secrets versions add sf-oauth-secret --data-file=-

# 3. Slack Webhook & KMS Signing Secret
gcloud secrets create slack-kms-secret --replication-policy="automatic"
echo -n '{"bot_token":"xoxb-...","signing_secret":"..."}' | \
    gcloud secrets versions add slack-kms-secret --data-file=-

# 4. HITL HMAC Secret Key
gcloud secrets create hitl-signing-key --replication-policy="automatic"
openssl rand -hex 32 | tr -d '\n' | \
    gcloud secrets versions add hitl-signing-key --data-file=-
```

### 3.2. Configure 3P Federated Connectors in Gemini Enterprise
1. Open Google Cloud Console $\to$ **Vertex AI Agent Engine** $\to$ **Connectors**.
2. Select **Add 3P Federated Connector**:
   - **ServiceNow**: Select OAuth2 Authorization Code Grant, enter Scope `incident.read,incident.write,sys_user.read`.
   - **Salesforce**: Select OAuth2 JWT Bearer, enter Connected App scopes `api,id,refresh_token`.
   - **Google Workspace Drive (1P)**: Enable 1P directory mapping for contract PDF search.

---

## 4. Build & Deploy Go BYO-MCP Service on Cloud Run

### 4.1. Build & Push Docker Image
```bash
export ARTIFACT_REGISTRY="${GCP_REGION}-docker.pkg.dev/${GCP_PROJECT_ID}/recon-agents"
gcloud artifacts repositories create recon-agents \
    --repository-format=docker \
    --location=${GCP_REGION} \
    --description="Data Recon Agent Container Registry"

docker build -t ${ARTIFACT_REGISTRY}/data-recon-server:latest -f Dockerfile .
docker push ${ARTIFACT_REGISTRY}/data-recon-server:latest
```

### 4.2. Deploy to Cloud Run with Secret Injections
```bash
gcloud run deploy data-recon-server \
    --image=${ARTIFACT_REGISTRY}/data-recon-server:latest \
    --region=${GCP_REGION} \
    --platform=managed \
    --service-account=recon-agent-sa@${GCP_PROJECT_ID}.iam.gserviceaccount.com \
    --set-env-vars="GCP_PROJECT_ID=${GCP_PROJECT_ID},GCP_REGION=${GCP_REGION},MODEL_ROUTER_FLASH=gemini-3.7-flash-preview,MODEL_ROUTER_PRO=gemini-3.1-pro" \
    --set-secrets="SN_CREDENTIALS=sn-oauth-secret:latest,SF_CREDENTIALS=sf-oauth-secret:latest,HITL_SECRET=hitl-signing-key:latest" \
    --concurrency=80 \
    --min-instances=1 \
    --max-instances=20 \
    --cpu=2 \
    --memory=2Gi \
    --allow-unauthenticated=false
```

---

## 5. Seed Synthetic Golden Datasets & Run Verification

### 5.1. Generate Synthetic Enterprise Data
Run the Go data synthesizing tool to generate 500+ realistic reconciliation test cases:

```bash
go run cmd/synth/main.go --count=500 --output=tests/golden/discrepancies_golden.json --seed=42
```

Upload golden dataset to GCS for continuous regression evaluation:
```bash
gsutil cp tests/golden/discrepancies_golden.json gs://${GCP_PROJECT_ID}-recon-golden-datasets/v1/
```

### 5.2. Execute Automated Evaluation Suite
Run the test harness against the live mock and schema validators:

```bash
go test -v -race ./tests/...
```

Expected Output:
```text
=== RUN   TestToolSchemaValidation
--- PASS: TestToolSchemaValidation (0.04s)
=== RUN   TestCoordinatorWorkerOrchestration
--- PASS: TestCoordinatorWorkerOrchestration (0.82s)
=== RUN   TestGuidedErrorRecovery
--- PASS: TestGuidedErrorRecovery (0.02s)
=== RUN   TestDLPPIIRedaction
--- PASS: TestDLPPIIRedaction (0.15s)
=== RUN   TestHITLWebhookSignatureEnforcement
--- PASS: TestHITLWebhookSignatureEnforcement (0.01s)
PASS
ok  	github.com/tiaaburton/Data-Recon-Agent/tests	1.04s
```

---

## 6. Health Checks & Verification Commands

Verify Agent Engine and Pub/Sub Event Loop:

```bash
# 1. Publish test discrepancy event to Pub/Sub
gcloud pubsub topics publish recon-discrepancy-events \
    --message='{"event_id":"evt-test-01","correlation_id":"CORR-9912","account_id":"ACC-88","account_name":"Acme Global","variance_amount":15400.0,"currency":"USD","severity":"CRITICAL","detected_at":"2026-08-16T23:00:00Z","description":"SAP Invoice INV-99042 diverges from Salesforce Contract CTR-4401"}'

# 2. Check Cloud Run logs for Intent vs Outcome capture and trace ID correlation
gcloud logging read 'resource.type="cloud_run_revision" AND jsonPayload.component="IntentOutcomeLogger"' \
    --limit=5 \
    --format=json

# 3. Verify Firestore session state persistence
gcloud firestore documents list "projects/${GCP_PROJECT_ID}/databases/(default)/documents/recon_sessions"
```
