#!/usr/bin/env bash
# ==============================================================================
# Register or Update Go ADK Data Recon Agent in Gemini Enterprise / Discovery Engine
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
ENGINE_PATH="projects/${PROJECT_NUMBER}/locations/${REGION}/reasoningEngines/${REASONING_ENGINE_ID}"
APP_LOCATION="us"
COLLECTION="default_collection"
ENGINE="gemini-app"
ASSISTANT="default_assistant"

BASE_URL="https://${APP_LOCATION}-discoveryengine.googleapis.com/v1alpha/projects/${PROJECT_ID}/locations/${APP_LOCATION}/collections/${COLLECTION}/engines/${ENGINE}/assistants/${ASSISTANT}/agents"

TOKEN="$(gcloud auth print-access-token)"

echo "================================================================="
echo "🌐 Registering Data Recon Agent in Gemini Enterprise Agent Platform"
echo "================================================================="
echo "Project:          $PROJECT_ID"
echo "Reasoning Engine: $ENGINE_PATH"
echo "Target Endpoint:  $BASE_URL"
echo "================================================================="

PAYLOAD=$(cat <<EOF
{
  "displayName": "Data Reconciliation Agent (ADK v2 / A2UI)",
  "description": "Autonomous multi-system enterprise data reconciliation agent cross-referencing Salesforce CRM billed invoices and ServiceNow ITSM SLA credit disputes to resolve financial discrepancies and stream interactive A2UI v0.9 declarative component cards.",
  "adkAgentDefinition": {
    "toolSettings": {
      "toolDescription": "Cross-reconciles enterprise contract spend caps against Salesforce Opportunity billed amounts and ServiceNow Incident dispute records. Streams interactive A2UI component cards detailing variance root cause and resolution workflows."
    },
    "provisionedReasoningEngine": {
      "reasoningEngine": "${ENGINE_PATH}"
    }
  },
  "sharingConfig": {
    "scope": "ALL_USERS"
  }
}
EOF
)

format_json() {
  if command -v jq >/dev/null 2>&1; then
    jq .
  elif command -v python3 >/dev/null 2>&1; then
    python3 -m json.tool
  else
    cat
  fi
}

if [[ -n "${GE_AGENT_ID:-}" ]]; then
  echo "Updating existing agent ID: $GE_AGENT_ID..."
  curl -s -X PATCH \
    -H "Authorization: Bearer $TOKEN" \
    -H "X-Goog-User-Project: $PROJECT_ID" \
    -H "Content-Type: application/json" \
    -d "$PAYLOAD" \
    "${BASE_URL}/${GE_AGENT_ID}?updateMask=displayName,description,adkAgentDefinition,sharingConfig" | format_json
else
  echo "Creating new agent registration..."
  RESP=$(curl -s -X POST \
    -H "Authorization: Bearer $TOKEN" \
    -H "X-Goog-User-Project: $PROJECT_ID" \
    -H "Content-Type: application/json" \
    -d "$PAYLOAD" \
    "$BASE_URL")
  echo "$RESP" | format_json
fi

echo ""
echo "✅ Agent successfully registered and enabled in Gemini Enterprise Agent Gateway!"
