#!/usr/bin/env bash
# ==============================================================================
# Setup & Provision Google Cloud Agent Gateway & Agent Registry (IaC Automation)
# ==============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

cd "$ROOT_DIR"

if [[ -f .env.local ]]; then
  # shellcheck source=/dev/null
  source .env.local
elif [[ -f .env ]]; then
  # shellcheck source=/dev/null
  source .env
fi

PROJECT_ID="${GCP_PROJECT_ID:-tias-demos}"
PROJECT_NUMBER="14200540645"
REGION="${GCP_REGION:-us-central1}"
REASONING_ENGINE_ID="${AGENT_ENGINE_ID:-1487588105090236416}"
GATEWAY_NAME="data-recon-egress-gateway"
AUTHZ_EXT_NAME="data-recon-iap-authz-ext"
AUTHZ_POLICY_NAME="data-recon-iap-authz-policy"
SERVICE_ENTRY_NAME="data-recon-agent"

echo "================================================================="
echo "🛡️  PROVISIONING GOOGLE CLOUD AGENT GATEWAY & AGENT REGISTRY (IaC)"
echo "================================================================="
echo "Project:          $PROJECT_ID (Number: $PROJECT_NUMBER)"
echo "Region:           $REGION"
echo "Reasoning Engine: $REASONING_ENGINE_ID"
echo "Gateway Name:     $GATEWAY_NAME"
echo "================================================================="

# ------------------------------------------------------------------------------
# 1. Enable Required Cloud APIs
# ------------------------------------------------------------------------------
echo ""
echo "--> 1. Enabling Required Gateway & Registry APIs..."
gcloud services enable \
  networkservices.googleapis.com \
  networksecurity.googleapis.com \
  agentregistry.googleapis.com \
  discoveryengine.googleapis.com \
  aiplatform.googleapis.com \
  compute.googleapis.com \
  dns.googleapis.com \
  iam.googleapis.com \
  storage.googleapis.com \
  modelarmor.googleapis.com \
  telemetry.googleapis.com \
  monitoring.googleapis.com \
  cloudtrace.googleapis.com \
  logging.googleapis.com \
  --project="$PROJECT_ID"

# ------------------------------------------------------------------------------
# 2. Deploy / Import Agent Gateway Resource
# ------------------------------------------------------------------------------
echo ""
echo "--> 2. Deploying Agent Gateway ($GATEWAY_NAME) in $REGION..."
gcloud network-services agent-gateways import "$GATEWAY_NAME" \
  --source="iac/gateway/data-recon-agent-gateway-egress.yaml" \
  --location="$REGION" \
  --project="$PROJECT_ID" || {
    echo "Notice: Gateway import command finished or already exists."
  }

# ------------------------------------------------------------------------------
# 3. Configure IAP Request Authorization Extension & Policy
# ------------------------------------------------------------------------------
echo ""
echo "--> 3. Configuring Service Extensions & Authz Policy..."
gcloud beta service-extensions authz-extensions import "$AUTHZ_EXT_NAME" \
  --source="iac/gateway/iap-request-authz-extension.yaml" \
  --location="$REGION" \
  --project="$PROJECT_ID" || true

gcloud network-security authz-policies import "$AUTHZ_POLICY_NAME" \
  --source="iac/gateway/iap-request-authz-policy.yaml" \
  --location="$REGION" \
  --project="$PROJECT_ID" || true

# ------------------------------------------------------------------------------
# 4. Register Reasoning Engine Service in Agent Registry
# ------------------------------------------------------------------------------
echo ""
echo "--> 4. Registering Reasoning Engine in Regional Agent Registry..."
ENGINE_URI="https://${REGION}-aiplatform.mtls.googleapis.com/v1/projects/${PROJECT_NUMBER}/locations/${REGION}/reasoningEngines/${REASONING_ENGINE_ID}"

gcloud agent-registry services create "$SERVICE_ENTRY_NAME" \
  --project="$PROJECT_ID" \
  --location="$REGION" \
  --display-name="Data Reconciliation Agent (ADK v2)" \
  --endpoint-spec-type=no-spec \
  --interfaces="url=${ENGINE_URI},protocolBinding=jsonrpc" \
  --format="value(registryResource)" || {
    echo "Notice: Service entry $SERVICE_ENTRY_NAME already registered or up-to-date."
  }

# ------------------------------------------------------------------------------
# 5. Register Outbound Dependency Endpoints in Agent Registry
# ------------------------------------------------------------------------------
echo ""
echo "--> 5. Registering Outbound Integration Endpoints (Salesforce, ServiceNow, Vertex AI)..."
# Register Salesforce Endpoint
gcloud agent-registry services create "salesforce-revenue-cloud" \
  --project="$PROJECT_ID" \
  --location="$REGION" \
  --display-name="Salesforce Dev Org (Revenue Cloud)" \
  --endpoint-spec-type=no-spec \
  --interfaces="url=https://orgfarm-b2f2a8eb8d-dev-ed.develop.my.salesforce.com,protocolBinding=https" || true

# Register ServiceNow Endpoint
gcloud agent-registry services create "servicenow-itsm" \
  --project="$PROJECT_ID" \
  --location="$REGION" \
  --display-name="ServiceNow Dev Instance (ITSM Incidents)" \
  --endpoint-spec-type=no-spec \
  --interfaces="url=https://dev410998.service-now.com,protocolBinding=https" || true

# Register Vertex AI & Gemini APIs
gcloud agent-registry services create "vertex-gemini-api" \
  --project="$PROJECT_ID" \
  --location="$REGION" \
  --display-name="Vertex AI Gemini APIs" \
  --endpoint-spec-type=no-spec \
  --interfaces="url=https://${REGION}-aiplatform.googleapis.com,protocolBinding=https" || true

# ------------------------------------------------------------------------------
# 6. Bind Reasoning Engine to Agent Gateway
# ------------------------------------------------------------------------------
echo ""
echo "--> 6. Associating Reasoning Engine with Agent Gateway..."
GATEWAY_RESOURCE_PATH="projects/${PROJECT_ID}/locations/${REGION}/agentGateways/${GATEWAY_NAME}"

format_json() {
  if command -v jq >/dev/null 2>&1; then
    jq .
  elif command -v python3 >/dev/null 2>&1; then
    python3 -m json.tool
  else
    cat
  fi
}

curl -s -X PATCH \
  -H "Authorization: Bearer $(gcloud auth print-access-token)" \
  -H "Content-Type: application/json; charset=utf-8" \
  -d "{
    \"spec\": {
      \"deploymentSpec\": {
        \"agentGatewayConfig\": {
          \"agentToAnywhereConfig\": {
            \"agentGateway\": \"${GATEWAY_RESOURCE_PATH}\"
          }
        }
      }
    }
  }" \
  "https://${REGION}-aiplatform.googleapis.com/v1/projects/${PROJECT_ID}/locations/${REGION}/reasoningEngines/${REASONING_ENGINE_ID}?updateMask=spec.deploymentSpec.agentGatewayConfig" | format_json

echo ""
echo "================================================================="
echo "✅ AGENT GATEWAY & AGENT REGISTRY PROVISIONING COMPLETE"
echo "================================================================="
